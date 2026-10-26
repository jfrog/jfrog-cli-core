package transferfiles

import (
	"sync"

	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	totalNumberPollingGoRoutines = 1
	tasksMaxCapacity             = 5000000
)

type transferManager struct {
	phaseBase
	delayUploadComparisonFunctions []shouldDelayUpload
}

func newTransferManager(base phaseBase, delayUploadComparisonFunctions []shouldDelayUpload) *transferManager {
	return &transferManager{phaseBase: base, delayUploadComparisonFunctions: delayUploadComparisonFunctions}
}

type transferActionWithProducerConsumerType func(
	pcWrapper *producerConsumerWrapper,
	delayHelper delayUploadHelper,
	errorsChannelMng *ErrorsChannelMng) error

type transferDelayAction func(phase phaseBase, addedDelayFiles []string) error

// Transfer files using the 'producer-consumer' mechanism and apply a delay action.
func (ftm *transferManager) doTransferWithProducerConsumer(transferAction transferActionWithProducerConsumerType, delayAction transferDelayAction) error {
	// Set the producer-consumer value into the referenced value. This allows the Graceful Stop mechanism to access ftm.pcDetails when needed to stop the transfer.
	*ftm.pcDetails = newProducerConsumerWrapper()
	return ftm.doTransfer(ftm.pcDetails, transferAction, delayAction)
}

// This function handles a transfer process as part of a phase.
// As part of the process, the transferAction gets executed. It may utilize a producer consumer or not.
// The transferAction collects artifacts to be uploaded into chunks, and uploads them directly to the target Artifactory instance.
// In some repositories the order of deployment is important. In these cases, any artifacts that should be delayed will be collected by
// the delayedArtifactsMng and will later be handled by handleDelayedArtifactsFiles.
// Any deployment failures will be written to a file by the transferErrorsMng to be handled on next run.
// The number of threads affects both the producer consumer if used, and limits the number of chunks uploaded in parallel. The number can be externally modified,
// and will be updated on runtime by periodicallyUpdateThreadsAndStopStatus.
func (ftm *transferManager) doTransfer(pcWrapper *producerConsumerWrapper, transferAction transferActionWithProducerConsumerType, delayAction transferDelayAction) error {
	var runWaitGroup sync.WaitGroup
	var writersWaitGroup sync.WaitGroup

	// Manager for the transfer's errors statuses writing mechanism
	errorsChannelMng := createErrorsChannelMng()
	transferErrorsMng, err := newTransferErrorsToFile(ftm.repoKey, ftm.phaseId, state.ConvertTimeToEpochMilliseconds(ftm.startTime), &errorsChannelMng, ftm.progressBar, ftm.stateManager)
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
	delayedArtifactsMng, err := newTransferDelayedArtifactsManager(&delayedArtifactsChannelMng, ftm.repoKey, state.ConvertTimeToEpochMilliseconds(ftm.startTime))
	if err != nil {
		return err
	}
	if len(ftm.delayUploadComparisonFunctions) > 0 {
		writersWaitGroup.Add(1)
		go func() {
			defer writersWaitGroup.Done()
			delayedArtifactsChannelMng.err = delayedArtifactsMng.start()
		}()
	}

	pollingTasksManager := newPollingTasksManager(totalNumberPollingGoRoutines)
	err = pollingTasksManager.start(&ftm.phaseBase, &runWaitGroup, pcWrapper)
	if err != nil {
		pollingTasksManager.stop()
		return err
	}
	// Transfer action to execute.
	runWaitGroup.Add(1)
	var actionErr error
	var delayUploadHelper = delayUploadHelper{
		ftm.delayUploadComparisonFunctions,
		&delayedArtifactsChannelMng,
	}
	go func() {
		defer runWaitGroup.Done()
		actionErr = transferAction(pcWrapper, delayUploadHelper, &errorsChannelMng)
		if pcWrapper == nil {
			pollingTasksManager.stop()
		}
	}()

	// Run producer consumers. This is a blocking function, which makes sure the producer consumers are closed before anything else.
	executionErr := runProducerConsumers(pcWrapper)
	pollingTasksManager.stop()
	// Wait for 'transferAction', producer consumers and polling go routines to exit.
	runWaitGroup.Wait()
	// Close writer channels.
	errorsChannelMng.close()
	delayedArtifactsChannelMng.close()
	// Wait for writers channels to exit. Writers must exit last.
	writersWaitGroup.Wait()

	var returnedError error
	for _, err := range []error{actionErr, errorsChannelMng.err, delayedArtifactsChannelMng.err, executionErr, ftm.getInterruptionErr()} {
		if err != nil {
			log.Error(err)
			returnedError = err
		}
	}

	// If delayed action was provided, handle it now.
	if returnedError == nil && delayAction != nil {
		var addedDelayFiles []string
		// If the transfer generated new delay files provide them
		if delayedArtifactsMng.delayedWriter != nil {
			addedDelayFiles = delayedArtifactsMng.delayedWriter.contentFiles
		}
		returnedError = delayAction(ftm.phaseBase, addedDelayFiles)
	}
	return returnedError
}

// PollingTasksManager coordinates the background go routines that run alongside a transfer phase.
// The name and 'doneChannel'/'totalGoRoutines' bookkeeping predate GET/PUT transfer, when this managed
// both a chunk-upload poller and the thread/stop-status poller; today only the latter
// (periodicallyUpdateThreadsAndStopStatus) runs, but the type is kept generic since more than one
// go routine may be added again in the future.
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

// Runs the polling go routine(s) for a transfer phase. Currently starts a single go routine that
// periodically updates the worker threads count & checks whether the process should be stopped.
func (ptm *PollingTasksManager) start(phaseBase *phaseBase, runWaitGroup *sync.WaitGroup, pcWrapper *producerConsumerWrapper) error {
	// Update threads by polling on the settings file.
	runWaitGroup.Add(1)
	err := ptm.addGoRoutine()
	if err != nil {
		return err
	}
	go func() {
		defer runWaitGroup.Done()
		periodicallyUpdateThreadsAndStopStatus(pcWrapper, ptm.doneChannel, phaseBase.buildInfoRepo, phaseBase.stateManager, phaseBase.stopSignal)
	}()

	if phaseBase.stateManager != nil {
		if err := phaseBase.stateManager.SetWorkingThreads(GetChunkUploaderThreads()); err != nil {
			log.Error("Couldn't set the current number of working threads:", err.Error())
		}
	}
	return nil
}

func (ptm *PollingTasksManager) addGoRoutine() error {
	if ptm.totalGoRoutines < ptm.totalRunningGoRoutines+1 {
		return errorutils.CheckErrorf("can't create another polling go routine. maximum number of go routines is: %d", ptm.totalGoRoutines)
	}
	ptm.totalRunningGoRoutines++
	return nil
}

func (ptm *PollingTasksManager) stop() {
	for i := 0; i < ptm.totalRunningGoRoutines; i++ {
		ptm.doneChannel <- true
	}
}

func newProducerConsumerWrapper() producerConsumerWrapper {
	chunkUploaderProducerConsumer := parallel.NewRunner(GetChunkUploaderThreads(), tasksMaxCapacity, false)
	chunkBuilderProducerConsumer := parallel.NewRunner(GetChunkBuilderThreads(), tasksMaxCapacity, false)
	chunkUploaderProducerConsumer.SetFinishedNotification(true)
	chunkBuilderProducerConsumer.SetFinishedNotification(true)
	errorsQueue := clientUtils.NewErrorsQueue(1)

	return producerConsumerWrapper{
		chunkUploaderProducerConsumer: chunkUploaderProducerConsumer,
		chunkBuilderProducerConsumer:  chunkBuilderProducerConsumer,
		errorsQueue:                   errorsQueue,
	}
}

func runProducerConsumers(pcWrapper *producerConsumerWrapper) (executionErr error) {
	go func() {
		pcWrapper.chunkUploaderProducerConsumer.Run()
	}()
	go func() {
		<-pcWrapper.chunkBuilderProducerConsumer.GetFinishedNotification()
		log.Debug("Chunk builder producer consumer has completed all tasks. " +
			"All files relevant to this phase were found and added to chunks that are being uploaded...")
		pcWrapper.chunkBuilderProducerConsumer.Done()
	}()

	pcWrapper.chunkBuilderProducerConsumer.Run()
	if pcWrapper.chunkUploaderProducerConsumer.IsStarted() {
		pcWrapper.chunkUploaderProducerConsumer.ResetFinishNotificationIfActive()
		<-pcWrapper.chunkUploaderProducerConsumer.GetFinishedNotification()
		log.Debug("Chunk uploaded producer consumer has completed all tasks. All files relevant to this phase have all been uploaded.")
	}
	pcWrapper.chunkUploaderProducerConsumer.Done()
	executionErr = pcWrapper.errorsQueue.GetError()
	return
}
