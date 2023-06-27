package transferfiles

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transfer"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
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
	uploadChunkChan chan UploadedChunk,
	delayHelper delayUploadHelper,
	errorsChannelMng *ErrorsChannelMng) error

type transferDelayAction func(phase phaseBase, addedDelayFiles []string) error

// Transfer files using the 'producer-consumer' mechanism and apply a delay action.
func (ftm *transferManager) doTransferWithProducerConsumer(transferAction transferActionWithProducerConsumerType, delayAction transferDelayAction) error {
	ftm.pcDetails = newProducerConsumerWrapper()
	return ftm.doTransfer(ftm.pcDetails, transferAction, delayAction)
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
func (ftm *transferManager) doTransfer(pcWrapper *producerConsumerWrapper, transferAction transferActionWithProducerConsumerType, delayAction transferDelayAction) error {
	uploadChunkChan := make(chan UploadedChunk, transfer.MaxThreadsLimit)
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

// Runs 2 go routines:
// 1. Periodically update the worker threads count & check whether the process should be stopped.
// 2. Poll for uploaded chunks.
func (ptm *PollingTasksManager) start(phaseBase *phaseBase, runWaitGroup *sync.WaitGroup, pcWrapper *producerConsumerWrapper, uploadChunkChan chan UploadedChunk, errorsChannelMng *ErrorsChannelMng) error {
	// Update threads by polling on the settings file.
	runWaitGroup.Add(1)
	err := ptm.addGoRoutine()
	if err != nil {
		return err
	}
	go func() {
		defer runWaitGroup.Done()
		periodicallyUpdateThreadsAndStopStatus(pcWrapper, ptm.doneChannel, phaseBase.buildInfoRepo, phaseBase.stopSignal)
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
		return errorutils.CheckErrorf("can't create another polling go routine. maximum number of go routines is: %d", ptm.totalGoRoutines)
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

// This function polls on chunks of files that were uploaded during one of the phases.
// It does so by requesting the status of each chunk, by sending the uuid token that was returned when the chunk was uploaded.
// Number of chunks is limited by the number of threads.
// Whenever the status of a chunk was received and is DONE, its token is removed from the tokens batch, making room for a new chunk to be uploaded
// and a new token to be polled on.
func pollUploads(phaseBase *phaseBase, srcUpService *srcUserPluginService, uploadChunkChan chan UploadedChunk, doneChan chan bool, errorsChannelMng *ErrorsChannelMng) {
	curTokensBatch := api.UploadChunksStatusBody{}
	chunksLifeCycleManager := ChunksLifeCycleManager{
		deletedChunksSet: datastructures.MakeSet[api.ChunkId](),
		nodeToChunksMap:  make(map[nodeId]map[api.ChunkId]UploadedChunkData),
	}
	curProcessedUploadChunks = 0
	var timeEstMng *state.TimeEstimationManager
	if phaseBase != nil {
		timeEstMng = &phaseBase.stateManager.TimeEstimationManager
	}
	for i := 0; ; i++ {
		if ShouldStop(phaseBase, nil, errorsChannelMng) {
			return
		}
		time.Sleep(waitTimeBetweenChunkStatusSeconds * time.Second)

		// Run once per 3 minutes
		if i%60 == 0 {
			// 'Working threads' are determined by how many upload chunks are currently being processed by the source Artifactory instance.
			if err := phaseBase.stateManager.SetWorkingThreads(curProcessedUploadChunks); err != nil {
				log.Error("Couldn't set the current number of working threads:", err.Error())
			}
		}

		// Each uploading thread receives a token and a node id from the source via the uploadChunkChan, so this go routine can poll on its status.
		fillChunkDataBatch(&chunksLifeCycleManager, uploadChunkChan)
		if err := chunksLifeCycleManager.StoreStaleChunks(phaseBase.stateManager); err != nil {
			log.Error("Couldn't store the stale chunks:", err.Error())
		}
		// When totalChunks size is zero, it means that all the tokens are uploaded,
		// we received 'DONE' for all of them, and we notified the source that they can be deleted from the memory.
		// If during the polling some chunks data were lost due to network issues, either on the client or on the source,
		// it will be written to the error channel
		if chunksLifeCycleManager.totalChunks == 0 {
			if shouldStopPolling(doneChan) {
				return
			}
			continue
		}

		chunksStatus, err := sendSyncChunksRequest(curTokensBatch, &chunksLifeCycleManager, srcUpService)
		if err != nil {
			continue
		}
		// Clear body for the next request
		curTokensBatch = api.UploadChunksStatusBody{}
		removeDeletedChunksFromSet(chunksStatus.DeletedChunks, chunksLifeCycleManager.deletedChunksSet)
		toStop := handleChunksStatuses(phaseBase, &chunksStatus, &chunksLifeCycleManager, timeEstMng, errorsChannelMng)
		if toStop {
			return
		}
	}
}

// Fill chunk data batch till full. Return if no new chunk data is available.
func fillChunkDataBatch(chunksLifeCycleManager *ChunksLifeCycleManager, uploadChunkChan chan UploadedChunk) {
	for chunksLifeCycleManager.totalChunks < GetThreads() {
		select {
		case data := <-uploadChunkChan:
			currentNodeId := nodeId(data.NodeId)
			currentChunkId := api.ChunkId(data.UuidToken)
			if _, exist := chunksLifeCycleManager.nodeToChunksMap[currentNodeId]; !exist {
				chunksLifeCycleManager.nodeToChunksMap[currentNodeId] = make(map[api.ChunkId]UploadedChunkData)
			}
			chunksLifeCycleManager.nodeToChunksMap[currentNodeId][currentChunkId] = data.UploadedChunkData
			chunksLifeCycleManager.totalChunks++
		default:
			// No new tokens are waiting.
			return
		}
	}
}

func shouldStopPolling(doneChan chan bool) bool {
	select {
	case done := <-doneChan:
		return done
	default:
	}
	return false
}

// Send and handle.
func sendSyncChunksRequest(curTokensBatch api.UploadChunksStatusBody, chunksLifeCycleManager *ChunksLifeCycleManager, srcUpService *srcUserPluginService) (api.UploadChunksStatusResponse, error) {
	curTokensBatch.AwaitingStatusChunks = chunksLifeCycleManager.GetInProgressTokensSlice()
	curTokensBatch.ChunksToDelete = chunksLifeCycleManager.deletedChunksSet.ToSlice()
	chunksStatus, err := srcUpService.syncChunks(curTokensBatch)
	// Log the error only if the transfer wasn't interrupted by the user
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Error("error returned when getting upload chunks statuses: " + err.Error())
	}
	return chunksStatus, err
}

func removeDeletedChunksFromSet(deletedChunks []string, deletedChunksSet *datastructures.Set[api.ChunkId]) {
	// deletedChunks is an array received from the source, confirming which chunks were deleted from the source side.
	// In deletedChunksSet, we keep only chunks for which we have yet to receive confirmation
	for _, deletedChunk := range deletedChunks {
		err := deletedChunksSet.Remove(api.ChunkId(deletedChunk))
		if err != nil {
			log.Error(err.Error())
			continue
		}
	}
}

// handleChunksStatuses handles the chunk statuses from the response received from the source Artifactory Instance.
// It syncs the chunk status between the CLI and the source Artifactory instance,
// When a chunk is DONE, the progress bar is updated, and the number of working threads is decreased.
func handleChunksStatuses(phase *phaseBase, chunksStatus *api.UploadChunksStatusResponse,
	chunksLifeCycleManager *ChunksLifeCycleManager, timeEstMng *state.TimeEstimationManager, errorsChannelMng *ErrorsChannelMng) bool {
	checkChunkStatusSync(chunksStatus, chunksLifeCycleManager, errorsChannelMng)
	for _, chunk := range chunksStatus.ChunksStatus {
		if chunk.UuidToken == "" {
			log.Error("Unexpected empty uuid token in status")
			continue
		}
		switch chunk.Status {
		case api.InProgress:
			continue
		case api.Done:
			reduceCurProcessedChunks()
			log.Debug("Received status DONE for chunk '" + chunk.UuidToken + "'")

			chunkSentTime := chunksLifeCycleManager.nodeToChunksMap[nodeId(chunksStatus.NodeId)][api.ChunkId(chunk.UuidToken)].TimeSent
			err := updateProgress(phase, timeEstMng, chunk, chunkSentTime)
			if err != nil {
				log.Error("Unexpected error in progress update: " + err.Error())
				continue
			}
			delete(chunksLifeCycleManager.nodeToChunksMap[nodeId(chunksStatus.NodeId)], api.ChunkId(chunk.UuidToken))
			chunksLifeCycleManager.totalChunks--
			// Using the deletedChunksSet, we inform the source that the 'DONE' message has been received, and it no longer has to keep those chunks UUIDs.
			chunksLifeCycleManager.deletedChunksSet.Add(api.ChunkId(chunk.UuidToken))
			stopped := handleFilesOfCompletedChunk(chunk.Files, errorsChannelMng)
			// In case an error occurred while writing errors status's to the errors file - stop transferring.
			if stopped {
				return true
			}
			err = setChunkCompletedInRepoSnapshot(phase.stateManager, chunk.Files)
			if err != nil {
				log.Error(err)
				continue
			}
		}
	}
	return false
}

func updateProgress(phase *phaseBase, timeEstMng *state.TimeEstimationManager,
	chunk api.ChunkStatus, chunkSentTime time.Time) error {
	if phase == nil {
		return nil
	}

	err := state.UpdateChunkInState(phase.stateManager, &chunk)
	if err != nil {
		return err
	}

	if timeEstMng != nil {
		timeEstMng.AddChunkStatus(chunk, time.Since(chunkSentTime).Milliseconds())
	}
	return nil
}

// Verify and handle in progress chunks synchronization between the CLI and the Source Artifactory instance
func checkChunkStatusSync(chunkStatus *api.UploadChunksStatusResponse, manager *ChunksLifeCycleManager, errorsChannelMng *ErrorsChannelMng) {
	// Compare between the number of chunks received from the latest syncChunks request to the chunks data we handle locally in nodeToChunksMap.
	// If the number of the in progress chunks of a node within nodeToChunksMap differs from the chunkStatus received, there is missing data on the source side.
	if len(chunkStatus.ChunksStatus) != len(manager.nodeToChunksMap[nodeId(chunkStatus.NodeId)]) {
		// Get all the chunks uuids on the Artifactory side in a set of uuids
		chunksUuidsSetFromResponse := datastructures.MakeSet[api.ChunkId]()
		for _, chunk := range chunkStatus.ChunksStatus {
			chunksUuidsSetFromResponse.Add(api.ChunkId(chunk.UuidToken))
		}
		// Get all the chunks uuids on the CLI side
		chunksUuidsSliceFromMap := manager.GetInProgressTokensSliceByNodeId(nodeId(chunkStatus.NodeId))
		failedFile := api.FileUploadStatusResponse{
			Status:     api.Fail,
			StatusCode: SyncErrorStatusCode,
			Reason:     SyncErrorReason,
		}
		// Send all missing chunks from the source Artifactory instance to errorsChannelMng
		// Missing chunks are those that are inside chunksUuidsSliceFromMap but not in chunksUuidsSetFromResponse
		for _, chunkUuid := range chunksUuidsSliceFromMap {
			if !chunksUuidsSetFromResponse.Exists(chunkUuid) {
				for _, file := range manager.nodeToChunksMap[nodeId(chunkStatus.NodeId)][chunkUuid].ChunkFiles {
					failedFile.FileRepresentation = file
					// errorsChannelMng will upload failed files again in phase 3 or in an additional transfer file run.
					addErrorToChannel(errorsChannelMng, failedFile)
				}
				delete(manager.nodeToChunksMap[nodeId(chunkStatus.NodeId)], chunkUuid)
				manager.totalChunks--
				reduceCurProcessedChunks()
			}
		}
	}
}
