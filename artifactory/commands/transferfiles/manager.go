package transferfiles

import (
	"github.com/jfrog/gofrog/parallel"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"sync"
)

type transferManager struct {
	phaseBase
	delayUploadComparisonFunctions []shouldDelayUpload
}

func newTransferManager(base phaseBase, delayUploadComparisonFunctions []shouldDelayUpload) transferManager {
	return transferManager{phaseBase: base, delayUploadComparisonFunctions: delayUploadComparisonFunctions}
}

type transferActionType func(optionalPcDetails producerConsumerDetails, uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error

// This function handles a transfer process as part of a phase.
// As part of the process, the transferAction gets executed. It may utilize a producer consumer or not.
// The transferAction collects artifacts to be uploaded into chunks, and sends them to the source Artifactory instance to handle.
// The Artifactory user plugin in the source instance will try to checksum-deploy all the artifacts in the chunk.
// If not successful, an uuid token will be returned and sent in a channel to be polled on for status in pollUploads.
// In some repositories the order of deployment is important. In these cases, any artifacts that should be delayed will be collected by
// the delayedArtifactsMng and will later be handled by handleDelayedArtifactsFiles.
// Any deployment failures will be written to a file by the transferErrorsMng to be handled on next run.
// The number of threads affect both the producer consumer if used, and limits the number of uploaded chunks. The number can be externally modified,
// and will be updated on runtime by periodicallyUpdateThreads.
func (ftm *transferManager) doTransfer(isProducerConsumer bool, transferAction transferActionType) error {
	uploadTokensChan := make(chan string, tasksMaxCapacity)
	var runWaitGroup sync.WaitGroup
	var writersWaitGroup sync.WaitGroup
	// Done channel notifies the polling go routines that no more tasks are expected.
	doneChan := make(chan bool, 2)

	// Manager for the transfer's errors statuses writing mechanism
	errorsChannelMng := createErrorsChannelMng()
	transferErrorsMng, err := newTransferErrorsToFile(ftm.repoKey, ftm.phaseId, convertTimeToEpochMilliseconds(ftm.startTime), &errorsChannelMng)
	if err != nil {
		return err
	}

	writersWaitGroup.Add(1)
	go func() {
		defer writersWaitGroup.Done()
		errorsChannelMng.err = transferErrorsMng.start()
	}()

	delayedArtifactsChannelMng := createdDelayedArtifactsChannelMng()
	delayedArtifactsMng := newTransferDelayedArtifactsToFile(&delayedArtifactsChannelMng)
	if len(ftm.delayUploadComparisonFunctions) > 0 {
		writersWaitGroup.Add(1)
		go func() {
			defer writersWaitGroup.Done()
			delayedArtifactsChannelMng.err = delayedArtifactsMng.start()
		}()
	}

	var pcDetails producerConsumerDetails
	if isProducerConsumer {
		pcDetails = initProducerConsumer()
	}

	// Update threads by polling on the settings file.
	runWaitGroup.Add(1)
	go func() {
		defer runWaitGroup.Done()
		periodicallyUpdateThreads(pcDetails.producerConsumer, doneChan)
	}()

	// Check status of uploaded chunks.
	runWaitGroup.Add(1)
	var pollingError error
	go func() {
		defer runWaitGroup.Done()
		pollingError = pollUploads(ftm.srcUpService, uploadTokensChan, doneChan, &errorsChannelMng)
	}()

	// Transfer action to execute.
	runWaitGroup.Add(1)
	var actionErr error
	go func() {
		defer runWaitGroup.Done()
		actionErr = transferAction(pcDetails, uploadTokensChan, delayUploadHelper{shouldDelayFunctions: ftm.delayUploadComparisonFunctions, delayedArtifactsChannelMng: &delayedArtifactsChannelMng}, &errorsChannelMng)
		if !isProducerConsumer {
			// Notify the other go routines that work is done.
			doneChan <- true
			doneChan <- true
		}
	}()

	// Run and wait till done if producer consumer is used.
	var runnerErr error
	var executionErr error
	if isProducerConsumer {
		runnerErr, executionErr = runProducerConsumer(pcDetails, &runWaitGroup)
		// Notify the other go routines that work is done.
		doneChan <- true
		doneChan <- true
	}
	// After done is sent, wait for polling go routines to exit.
	runWaitGroup.Wait()
	// Close writer channels.
	errorsChannelMng.close()
	delayedArtifactsChannelMng.close()
	// Wait for writers channels to exit. Writers must exit last.
	writersWaitGroup.Wait()

	var returnedError error
	for _, err := range []error{actionErr, pollingError, errorsChannelMng.err, delayedArtifactsChannelMng.err, runnerErr, executionErr} {
		if err != nil {
			log.Error(err)
			returnedError = err
		}
	}

	// If delayed uploads, handle them now.
	if returnedError == nil && len(ftm.delayUploadComparisonFunctions) > 0 && len(delayedArtifactsMng.filesToConsume) > 0 {
		// Remove the first delay comparison function to no longer delay it.
		returnedError = handleDelayedArtifactsFiles(delayedArtifactsMng.filesToConsume, ftm.phaseBase, ftm.delayUploadComparisonFunctions[1:])
	}
	return returnedError
}

func initProducerConsumer() producerConsumerDetails {
	producerConsumer := parallel.NewRunner(getThreads(), tasksMaxCapacity, false)
	errorsQueue := clientUtils.NewErrorsQueue(1)

	return producerConsumerDetails{
		producerConsumer: producerConsumer,
		errorsQueue:      errorsQueue,
	}
}

func runProducerConsumer(pcDetails producerConsumerDetails, runWaitGroup *sync.WaitGroup) (runnerErr error, executionErr error) {
	runWaitGroup.Add(1)
	go func() {
		defer runWaitGroup.Done()
		// When the producer consumer is idle for assumeProducerConsumerDoneWhenIdleForSeconds (not tasks are being handled)
		// the work is assumed to be done.
		runnerErr = pcDetails.producerConsumer.DoneWhenAllIdle(assumeProducerConsumerDoneWhenIdleForSeconds)
	}()
	// Blocked until finish consuming
	pcDetails.producerConsumer.Run()
	executionErr = pcDetails.errorsQueue.GetError()
	return
}
