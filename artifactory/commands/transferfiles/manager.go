package transferfiles

import (
	"fmt"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transfer"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"sync"
)

const (
	totalNumberPollingGoRoutines = 2
	tasksMaxCapacity             = 5000000
)

type transferManager struct {
	phaseBase
	delayUploadComparisonFunctions []shouldDelayUpload
}

func newTransferManager(base phaseBase, delayUploadComparisonFunctions []shouldDelayUpload) transferManager {
	return transferManager{phaseBase: base, delayUploadComparisonFunctions: delayUploadComparisonFunctions}
}

type transferActionWithProducerConsumerType func(pcDetails *producerConsumerDetails, uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error
type transferActionType func(uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error

// Transfer files using the 'producer-consumer' mechanism.
func (ftm *transferManager) doTransferWithProducerConsumer(transferAction transferActionWithProducerConsumerType) error {
	pcDetails := initProducerConsumer()
	return ftm.doTransfer(&pcDetails, transferAction)
}

// Transfer files using a single producer.
func (ftm *transferManager) doTransferWithSingleProducer(transferAction transferActionType) error {
	transferActionPc := func(pcDetails *producerConsumerDetails, uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
		return transferAction(uploadTokensChan, delayHelper, errorsChannelMng)
	}
	return ftm.doTransfer(nil, transferActionPc)
}

// This function handles a transfer process as part of a phase.
// As part of the process, the transferAction gets executed. It may utilize a producer consumer or not.
// The transferAction collects artifacts to be uploaded into chunks, and sends them to the source Artifactory instance to handle asynchronously.
// An uuid token will be returned and sent in a channel to be polled on for status in pollUploads.
// In some repositories the order of deployment is important. In these cases, any artifacts that should be delayed will be collected by
// the delayedArtifactsMng and will later be handled by handleDelayedArtifactsFiles.
// Any deployment failures will be written to a file by the transferErrorsMng to be handled on next run.
// The number of threads affect both the producer consumer if used, and limits the number of uploaded chunks. The number can be externally modified,
// and will be updated on runtime by periodicallyUpdateThreads.
func (ftm *transferManager) doTransfer(pcDetails *producerConsumerDetails, transferAction transferActionWithProducerConsumerType) error {
	uploadTokensChan := make(chan string, transfer.MaxThreadsLimit)
	var runWaitGroup sync.WaitGroup
	var writersWaitGroup sync.WaitGroup

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

	// Manager for the transfer's delayed artifacts writing mechanism
	delayedArtifactsChannelMng := createdDelayedArtifactsChannelMng()
	delayedArtifactsMng := newTransferDelayedArtifactsToFile(&delayedArtifactsChannelMng)
	if len(ftm.delayUploadComparisonFunctions) > 0 {
		writersWaitGroup.Add(1)
		go func() {
			defer writersWaitGroup.Done()
			delayedArtifactsChannelMng.err = delayedArtifactsMng.start()
		}()
	}

	// Manager for transfer's tasks "done" channel - channel which indicate that files transfer is finished.
	pollingTasksManager := newPollingTasksManager(totalNumberPollingGoRoutines)
	var producerConsumer parallel.Runner
	if pcDetails != nil {
		producerConsumer = pcDetails.producerConsumer
	}
	err = pollingTasksManager.start(&ftm.phaseBase, &runWaitGroup, producerConsumer, uploadTokensChan, ftm.srcUpService, &errorsChannelMng, ftm.progressBar)
	if err != nil {
		pollingTasksManager.stop()
		return err
	}
	// Transfer action to execute.
	runWaitGroup.Add(1)
	var actionErr error
	go func() {
		defer runWaitGroup.Done()
		actionErr = transferAction(pcDetails, uploadTokensChan, delayUploadHelper{shouldDelayFunctions: ftm.delayUploadComparisonFunctions, delayedArtifactsChannelMng: &delayedArtifactsChannelMng}, &errorsChannelMng)
		if pcDetails == nil {
			pollingTasksManager.stop()
		}
	}()

	// Run and wait till done if producer consumer is used.
	var runnerErr error
	var executionErr error
	if pcDetails != nil {
		runnerErr, executionErr = runProducerConsumer(*pcDetails, &runWaitGroup)
		pollingTasksManager.stop()
	}
	// After done is sent, wait for polling go routines to exit.
	runWaitGroup.Wait()
	// Close writer channels.
	errorsChannelMng.close()
	delayedArtifactsChannelMng.close()
	// Wait for writers channels to exit. Writers must exit last.
	writersWaitGroup.Wait()

	var returnedError error
	for _, err := range []error{actionErr, errorsChannelMng.err, delayedArtifactsChannelMng.err, runnerErr, executionErr, ftm.getInterruptionErr()} {
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

type PollingTasksManager struct {
	// Done channel notifies the polling go routines that no more tasks are expected.
	doneChannel chan bool
	// Number of go routines expected to write to the doneChannel
	totalGoRoutines int
	// The actual number of running go routines
	totalRunningGoRoutines int
}

func newPollingTasksManager(totalGoRoutines int) PollingTasksManager {
	// The channel's size is 'totalGoRoutines', since there are a limited number of routines that need to be signaled to stop by 'doneChannel'.
	return PollingTasksManager{doneChannel: make(chan bool, totalGoRoutines), totalGoRoutines: totalGoRoutines}
}

// Runs 2 go routines :
// 1. Check number of threads
// 2. Poll uploaded chunks
func (ptm *PollingTasksManager) start(phaseBase *phaseBase, runWaitGroup *sync.WaitGroup, producerConsumer parallel.Runner, uploadTokensChan chan string, srcUpService *srcUserPluginService, errorsChannelMng *ErrorsChannelMng, progressbar *progressbar.TransferProgressMng) error {
	// Update threads by polling on the settings file.
	runWaitGroup.Add(1)
	err := ptm.addGoRoutine()
	if err != nil {
		return err
	}
	go func() {
		defer runWaitGroup.Done()
		periodicallyUpdateThreads(producerConsumer, ptm.doneChannel, phaseBase.buildInfoRepo)
	}()

	// Check status of uploaded chunks.
	runWaitGroup.Add(1)
	err = ptm.addGoRoutine()
	if err != nil {
		return err
	}
	go func() {
		defer runWaitGroup.Done()
		pollUploads(phaseBase, srcUpService, uploadTokensChan, ptm.doneChannel, errorsChannelMng, progressbar)
	}()
	return nil
}

func (ptm *PollingTasksManager) addGoRoutine() error {
	if ptm.totalGoRoutines < ptm.totalRunningGoRoutines+1 {
		return fmt.Errorf("can't create another polling go routine. maximum number of go routines is: %d", ptm.totalGoRoutines)
	}
	ptm.totalRunningGoRoutines++
	return nil
}

func (ptm *PollingTasksManager) stop() {
	// Notify the other go routines that work is done.
	for i := 0; i < ptm.totalRunningGoRoutines; i++ {
		ptm.doneChannel <- true
	}
}

type producerConsumerDetails struct {
	producerConsumer parallel.Runner
	errorsQueue      *clientUtils.ErrorsQueue
}

func initProducerConsumer() producerConsumerDetails {
	producerConsumer := parallel.NewRunner(GetThreads(), tasksMaxCapacity, false)
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
