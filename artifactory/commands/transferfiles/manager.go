package transferfiles

import (
	"fmt"
	"sync"

	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transfer"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	totalNumberPollingGoRoutines = 2
	tasksMaxCapacity             = 5000000
)

type transferManager struct {
	phaseBase
	delayUploadComparisonFunctions []shouldDelayUpload
	pcDetails                      *producerConsumerWrapper
}

func newTransferManager(base phaseBase, delayUploadComparisonFunctions []shouldDelayUpload) *transferManager {
	return &transferManager{phaseBase: base, delayUploadComparisonFunctions: delayUploadComparisonFunctions}
}

type transferActionWithProducerConsumerType func(pcWrapper *producerConsumerWrapper, uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error
type transferActionType func(pcWrapper *producerConsumerWrapper, uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error

// Transfer files using the 'producer-consumer' mechanism.
func (ftm *transferManager) doTransferWithProducerConsumer(transferAction transferActionWithProducerConsumerType) error {
	ftm.pcDetails = newProducerConsumerWrapper()
	return ftm.doTransfer(ftm.pcDetails, transferAction)
}

// Transfer files using a single producer.
func (ftm *transferManager) doTransferWithSingleProducer(transferAction transferActionType) error {
	transferActionPc := func(pcWrapper *producerConsumerWrapper, uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
		return transferAction(pcWrapper, uploadTokensChan, delayHelper, errorsChannelMng)
	}
	ftm.pcDetails = newProducerConsumerWrapper()
	return ftm.doTransfer(ftm.pcDetails, transferActionPc)
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
func (ftm *transferManager) doTransfer(pcWrapper *producerConsumerWrapper, transferAction transferActionWithProducerConsumerType) error {
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

	if ftm.phaseId == FullTransferPhase {
		ftm.timeEstMng.setTimeEstimationUnavailable(false)
	} else {
		ftm.timeEstMng.setTimeEstimationUnavailable(true)
	}

	pollingTasksManager := newPollingTasksManager(totalNumberPollingGoRoutines)
	err = pollingTasksManager.start(&ftm.phaseBase, &runWaitGroup, pcWrapper.chunkUploaderProducerConsumer, uploadTokensChan, &errorsChannelMng)
	if err != nil {
		pollingTasksManager.stop()
		return err
	}
	// Transfer action to execute.
	runWaitGroup.Add(1)
	var actionErr error
	go func() {
		defer runWaitGroup.Done()
		actionErr = transferAction(pcWrapper, uploadTokensChan, delayUploadHelper{shouldDelayFunctions: ftm.delayUploadComparisonFunctions, delayedArtifactsChannelMng: &delayedArtifactsChannelMng}, &errorsChannelMng)
		if pcWrapper == nil {
			pollingTasksManager.stop()
		}
	}()

	// Run producer consumer
	executionErr := runProducerConsumer(pcWrapper)
	pollingTasksManager.stop()
	// Wait for 'transferAction' and polling go routines to exit.
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

	// If delayed uploads, handle them now.
	if returnedError == nil && len(ftm.delayUploadComparisonFunctions) > 0 && len(delayedArtifactsMng.filesToConsume) > 0 {
		// Remove the first delay comparison function to no longer delay it.
		returnedError = handleDelayedArtifactsFiles(delayedArtifactsMng.filesToConsume, ftm.phaseBase, ftm.delayUploadComparisonFunctions[1:])
	}
	return returnedError
}

func (ftm *transferManager) stopProcuderConsumer() {
	if ftm.pcDetails != nil {
		ftm.pcDetails.chunkBuilderProducerConsumer.Cancel()
		ftm.pcDetails.chunkUploaderProducerConsumer.Cancel()
	}
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
func (ptm *PollingTasksManager) start(phaseBase *phaseBase, runWaitGroup *sync.WaitGroup, producerConsumer parallel.Runner, uploadTokensChan chan string, errorsChannelMng *ErrorsChannelMng) error {
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
		pollUploads(phaseBase, uploadTokensChan, ptm.doneChannel, errorsChannelMng)
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

type producerConsumerWrapper struct {
	chunkBuilderProducerConsumer  parallel.Runner
	chunkUploaderProducerConsumer parallel.Runner
	errorsQueue                   *clientUtils.ErrorsQueue
}

func newProducerConsumerWrapper() *producerConsumerWrapper {
	chunkUploaderProducerConsumer := parallel.NewRunner(GetThreads(), tasksMaxCapacity, false)
	chunkBuilderProducerConsumer := parallel.NewRunner(GetThreads(), tasksMaxCapacity, false)
	errorsQueue := clientUtils.NewErrorsQueue(1)

	return &producerConsumerWrapper{
		chunkUploaderProducerConsumer: chunkUploaderProducerConsumer,
		chunkBuilderProducerConsumer:  chunkBuilderProducerConsumer,
		errorsQueue:                   errorsQueue,
	}
}

func runProducerConsumer(pcWrapper *producerConsumerWrapper) (executionErr error) {
	// When the producer consumer is idle for assumeProducerConsumerDoneWhenIdleForSeconds (no tasks are being handled)
	// the work is assumed to be done.
	go func() {
		err := pcWrapper.chunkUploaderProducerConsumer.DoneWhenAllIdle(assumeProducerConsumerDoneWhenIdleForSeconds)
		if err != nil {
			log.Error("pcWrapper.chunkUploaderProducerConsumer.DoneWhenAllIdle API failed", err.Error())
		}
	}()
	go func() {
		err := pcWrapper.chunkBuilderProducerConsumer.DoneWhenAllIdle(assumeProducerConsumerDoneWhenIdleForSeconds)
		if err != nil {
			log.Error("pcWrapper.chunkBuilderProducerConsumer.DoneWhenAllIdle API failed", err.Error())
		}
	}()

	go func() {
		pcWrapper.chunkBuilderProducerConsumer.Run()
	}()
	// Run() is a blocking method, so once all uploading threads are idle, the tasks queue closes and Run() stops running
	pcWrapper.chunkUploaderProducerConsumer.Run()
	executionErr = pcWrapper.errorsQueue.GetError()
	return
}
