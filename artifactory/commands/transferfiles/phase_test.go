package transferfiles

import (
	"testing"
	"time"

	"github.com/jfrog/gofrog/parallel"
	"github.com/stretchr/testify/assert"
)

const zeroUint32 uint32 = 0

func TestStopGracefully(t *testing.T) {
	phaseBase := &phaseBase{pcDetails: newProducerConsumerWrapper()}
	go func() {
		// Stop gracefully after half second
		time.Sleep(time.Second / 2)

		// Assert active threads before stopping the producer consumers
		phaseBase.pcDetails.chunkUploaderProducerConsumer.IsStarted()
		assert.Greater(t, phaseBase.pcDetails.chunkUploaderProducerConsumer.ActiveThreads(), zeroUint32)
		assert.Greater(t, phaseBase.pcDetails.chunkBuilderProducerConsumer.ActiveThreads(), zeroUint32)

		// Stop the running threads
		phaseBase.StopGracefully()
	}()

	// Run 5 counter tasks in the uploader and builder producer-consumers
	uploaderCounter, builderCounter := 0, 0
	for i := 0; i < 5; i++ {
		phaseBase.pcDetails.chunkUploaderProducerConsumer.AddTask(createCounterTask(&uploaderCounter))
		phaseBase.pcDetails.chunkBuilderProducerConsumer.AddTask(createCounterTask(&builderCounter))
	}
	err := runProducerConsumers(phaseBase.pcDetails)
	assert.NoError(t, err)

	// Since we stopped the tasks after half second, and the tasks sleep for one second during their execution, expect the tasks to run exactly once.
	assert.Equal(t, 1, uploaderCounter)
	assert.Equal(t, 1, builderCounter)

	// Assert no active threads
	assert.Equal(t, zeroUint32, phaseBase.pcDetails.chunkUploaderProducerConsumer.ActiveThreads())
	assert.Equal(t, zeroUint32, phaseBase.pcDetails.chunkBuilderProducerConsumer.ActiveThreads())
}

// Create a task that increases the counter by 1 after a second
func createCounterTask(counter *int) parallel.TaskFunc {
	return func(int) error {
		(*counter)++
		time.Sleep(time.Second)
		return nil
	}
}
