package transferfiles

import (
	"sync"

	"github.com/jfrog/gofrog/parallel"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
)

type producerConsumerWrapper struct {
	// This Producer-Consumer is used to upload chunks, initialized in newProducerConsumerWrapper; each uploading thread waits to be given tasks from the queue.
	chunkUploaderProducerConsumer parallel.Runner
	// This Producer-Consumer is used to execute AQLs and build chunks from the AQLs' results. The chunks data is sent to the go routines that will upload them.
	// Initialized in newProducerConsumerWrapper; each builder thread waits to be given tasks from the queue.
	chunkBuilderProducerConsumer parallel.Runner
	// Errors related to chunkUploaderProducerConsumer and chunkBuilderProducerConsumer are logged in this queue.
	errorsQueue *clientUtils.ErrorsQueue
	// This variable holds the total number of upload chunk that were sent to the source Artifactory instance to process.
	// Together with this mutex, they control the load on the user plugin and couple it to the local number of threads.
	totalProcessedUploadChunks int
	processedUploadChunksMutex sync.Mutex
}

// Checks whether the total number of upload chunks sent is lower than the number of threads, and if so, increments it.
// Returns true if the total number was indeed incremented.
func (producerConsumerWrapper *producerConsumerWrapper) incProcessedChunksWhenPossible() bool {
	producerConsumerWrapper.processedUploadChunksMutex.Lock()
	defer producerConsumerWrapper.processedUploadChunksMutex.Unlock()
	if producerConsumerWrapper.totalProcessedUploadChunks < GetChunkUploaderThreads() {
		producerConsumerWrapper.totalProcessedUploadChunks++
		return true
	}
	return false
}

// Reduces the current total number of upload chunks processed. Called when an upload chunks doesn't require polling for status -
// if it's done processing, or an error occurred when sending it.
func (producerConsumerWrapper *producerConsumerWrapper) decProcessedChunks() {
	producerConsumerWrapper.processedUploadChunksMutex.Lock()
	defer producerConsumerWrapper.processedUploadChunksMutex.Unlock()
	producerConsumerWrapper.totalProcessedUploadChunks--
}
