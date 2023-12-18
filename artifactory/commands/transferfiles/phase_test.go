package transferfiles

import (
	"fmt"
	"testing"
	"time"

	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/stretchr/testify/assert"
)

const zeroUint32 uint32 = 0

func TestStopGracefully(t *testing.T) {
	pcWrapper := newProducerConsumerWrapper()
	pBase := &phaseBase{pcDetails: &pcWrapper}
	chunkUploaderProducerConsumer := pBase.pcDetails.chunkUploaderProducerConsumer
	chunkBuilderProducerConsumer := pBase.pcDetails.chunkBuilderProducerConsumer
	go func() {
		// Stop gracefully after half second
		time.Sleep(time.Second / 2)

		// Assert active threads before stopping the producer consumers
		chunkUploaderProducerConsumer.IsStarted()
		assert.Greater(t, chunkUploaderProducerConsumer.ActiveThreads(), zeroUint32)
		assert.Greater(t, chunkBuilderProducerConsumer.ActiveThreads(), zeroUint32)

		// Stop the running threads
		pBase.StopGracefully()
	}()

	var err error
	// Run 5 counter tasks in the uploader and builder producer-consumers
	uploaderCounter, builderCounter := 0, 0
	for i := 0; i < 5; i++ {
		_, err = chunkUploaderProducerConsumer.AddTask(createCounterTask(&uploaderCounter))
		assert.NoError(t, err)
		_, err = chunkBuilderProducerConsumer.AddTask(createCounterTask(&builderCounter))
		assert.NoError(t, err)
	}
	err = runProducerConsumers(pBase.pcDetails)
	assert.NoError(t, err)

	// Wait for no active threads
	waitForTasksToFinish(t, chunkUploaderProducerConsumer, chunkBuilderProducerConsumer)

	// Since we stopped the tasks after half second, and the tasks sleep for one second during their execution, expect the tasks to run exactly once.
	assert.Equal(t, 1, uploaderCounter)
	assert.Equal(t, 1, builderCounter)
}

// Create a task that increases the counter by 1 after a second
func createCounterTask(counter *int) parallel.TaskFunc {
	return func(int) error {
		*counter++
		time.Sleep(time.Second)
		return nil
	}
}

func waitForTasksToFinish(t *testing.T, chunkUploaderProducerConsumer, chunkBuilderProducerConsumer parallel.Runner) {
	// Wait for no active threads
	pollingExecutor := &utils.RetryExecutor{
		MaxRetries:               10,
		RetriesIntervalMilliSecs: 1000,
		ErrorMessage:             "Active producer-consumer tasks remained",
		ExecutionHandler: func() (shouldRetry bool, err error) {
			if chunkUploaderProducerConsumer.ActiveThreads() == 0 && chunkBuilderProducerConsumer.ActiveThreads() == 0 {
				return false, nil
			}
			return true, fmt.Errorf("active uploader threads: %d. Active builder threads: %d", chunkUploaderProducerConsumer.ActiveThreads(), chunkBuilderProducerConsumer.ActiveThreads())
		},
	}
	assert.NoError(t, pollingExecutor.Execute())
}
