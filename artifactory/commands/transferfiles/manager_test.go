package transferfiles

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunProducerConsumers(t *testing.T) {
	// Create the producer-consumers
	producerConsumerWrapper := newProducerConsumerWrapper()

	// Add 10 tasks for the chunkBuilderProducerConsumer. Each task provides a task to the chunkUploaderProducerConsumer.
	for i := 0; i < 10; i++ {
		_, err := producerConsumerWrapper.chunkBuilderProducerConsumer.AddTask(func(int) error {
			time.Sleep(time.Millisecond * 100)
			_, err := producerConsumerWrapper.chunkUploaderProducerConsumer.AddTask(
				func(int) error {
					time.Sleep(time.Millisecond)
					return nil
				},
			)
			assert.NoError(t, err)
			return nil
		})
		assert.NoError(t, err)
	}

	// Run the producer-consumers
	err := runProducerConsumers(&producerConsumerWrapper)
	assert.NoError(t, err)

	// Assert no active treads left in the producer-consumers
	assert.Zero(t, producerConsumerWrapper.chunkBuilderProducerConsumer.ActiveThreads())
	assert.Zero(t, producerConsumerWrapper.chunkUploaderProducerConsumer.ActiveThreads())
}
