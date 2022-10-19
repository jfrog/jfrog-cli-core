package transferfiles

import (
	"fmt"
	"sync"

	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transfer"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
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
}

func newTransferManager(base phaseBase, delayUploadComparisonFunctions []shouldDelayUpload) *transferManager {
	return &transferManager{phaseBase: base, delayUploadComparisonFunctions: delayUploadComparisonFunctions}
}

type transferActionWithProducerConsumerType func(
	pcWrapper *producerConsumerWrapper,
	uploadChunkChan chan UploadedChunkData,
	delayHelper delayUploadHelper,
	errorsChannelMng *ErrorsChannelMng) error

// Transfer files using the 'producer-consumer' mechanism.
func (ftm *transferManager) doTransferWithProducerConsumer(transferAction transferActionWithProducerConsumerType) error {
	ftm.pcDetails = newProducerConsumerWrapper()
	return ftm.doTransfer(ftm.pcDetails, transferAction)
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
	uploadChunkChan := make(chan UploadedChunkData, transfer.MaxThreadsLimit)
	var runWaitGroup sync.WaitGroup
	var writersWaitGroup sync.WaitGroup

	// Manager for the transfer's errors statuses writing mechanism
	errorsChannelMng := createErrorsChannelMng()
	transferErrorsMng, err := newTransferErrorsToFile(ftm.repoKey, ftm.phaseId, state.ConvertTimeToEpochMilliseconds(ftm.startTime), &errorsChannelMng, ftm.progressBar)
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

	if ftm.timeEstMng != nil {
		ftm.timeEstMng.setTimeEstimationUnavailable(ftm.phaseId != FullTransferPhase)
	}

	pollingTasksManager := newPollingTasksManager(totalNumberPollingGoRoutines)
	err = pollingTasksManager.start(&ftm.phaseBase, &runWaitGroup, pcWrapper, uploadChunkChan, &errorsChannelMng)
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
		actionErr = transferAction(pcWrapper, uploadChunkChan, delayUploadHelper, &errorsChannelMng)
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
func (ptm *PollingTasksManager) start(phaseBase *phaseBase, runWaitGroup *sync.WaitGroup, pcWrapper *producerConsumerWrapper, uploadChunkChan chan UploadedChunkData, errorsChannelMng *ErrorsChannelMng) error {
	// Update threads by polling on the settings file.
	runWaitGroup.Add(1)
	err := ptm.addGoRoutine()
	if err != nil {
		return err
	}
	go func() {
		defer runWaitGroup.Done()
		periodicallyUpdateThreads(pcWrapper, ptm.doneChannel, phaseBase.buildInfoRepo)
	}()

	// Check status of uploaded chunks.
	runWaitGroup.Add(1)
	err = ptm.addGoRoutine()
	if err != nil {
		return err
	}
	go func() {
		defer runWaitGroup.Done()
		pollUploads(phaseBase, phaseBase.srcUpService, uploadChunkChan, ptm.doneChannel, errorsChannelMng)
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
	// This Producer-Consumer is used to upload chunks, initialized in newProducerConsumerWrapper; each uploading thread waits to be given tasks from the queue.
	chunkUploaderProducerConsumer parallel.Runner
	// This Producer-Consumer is used to execute AQLs and build chunks from the AQLs' results. The chunks data is sent to the go routines that will upload them.
	// Initialized in newProducerConsumerWrapper; each builder thread waits to be given tasks from the queue.
	chunkBuilderProducerConsumer parallel.Runner
	// Errors related to chunkUploaderProducerConsumer and chunkBuilderProducerConsumer are logged in this queue.
	errorsQueue *clientUtils.ErrorsQueue
}

func newProducerConsumerWrapper() *producerConsumerWrapper {
	chunkUploaderProducerConsumer := parallel.NewRunner(GetThreads(), tasksMaxCapacity, false)
	chunkBuilderProducerConsumer := parallel.NewRunner(GetThreads(), tasksMaxCapacity, false)
	chunkUploaderProducerConsumer.SetFinishedNotification(true)
	chunkBuilderProducerConsumer.SetFinishedNotification(true)
	errorsQueue := clientUtils.NewErrorsQueue(1)

	return &producerConsumerWrapper{
		chunkUploaderProducerConsumer: chunkUploaderProducerConsumer,
		chunkBuilderProducerConsumer:  chunkBuilderProducerConsumer,
		errorsQueue:                   errorsQueue,
	}
}

// Run the two producer consumer that run the transfer.
// When a producer consumer is idle for assumeProducerConsumerDoneWhenIdleForSeconds (no tasks are being handled)
// the work is assumed to be done.
// Order in this function matters! We want to make sure chunkUploaderProducerConsumer is only done after chunkBuilderProducerConsumer is done.
func runProducerConsumers(pcWrapper *producerConsumerWrapper) (executionErr error) {
	go func() {
		pcWrapper.chunkUploaderProducerConsumer.Run()
	}()
	go func() {
		// Wait till notified that the builder has no additional tasks, and close the builder producer consumer.
		<-pcWrapper.chunkBuilderProducerConsumer.GetFinishedNotification()
		pcWrapper.chunkBuilderProducerConsumer.Done()
	}()

	// Run() is a blocking method, so once all chunk builders are idle, the tasks queue closes and Run() stops running.
	pcWrapper.chunkBuilderProducerConsumer.Run()
	if pcWrapper.chunkUploaderProducerConsumer.IsStarted() {
		// Wait till notified that the uploader finished its tasks, and it will not receive new tasks from the builder.
		<-pcWrapper.chunkUploaderProducerConsumer.GetFinishedNotification()
	}
	// Close the tasks queue with Done().
	pcWrapper.chunkUploaderProducerConsumer.Done()
	executionErr = pcWrapper.errorsQueue.GetError()
	return
}
