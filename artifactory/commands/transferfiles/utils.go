package transferfiles

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"

	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	serviceUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	waitTimeBetweenChunkStatusSeconds   = 3
	waitTimeBetweenThreadsUpdateSeconds = 20
	DefaultAqlPaginationLimit           = 10000
)

type (
	nodeId  string
	chunkId string
)

const SyncErrorReason = "unsynchronized chunk status due to network issue"
const SyncErrorStatusCode = 404

var AqlPaginationLimit = DefaultAqlPaginationLimit
var curThreads int

type UploadedChunkData struct {
	ChunkUuid  string
	ChunkFiles []FileRepresentation
	NodeIdResponse
}

type ChunksLifeCycleManager struct {
	// deletedChunksSet stores chunk uuids that have received a 'DONE' response from the source Artifactory instance
	// It is used to notify the source Artifactory instance that these chunks can be deleted from the source's status map.
	deletedChunksSet *datastructures.Set[chunkId]
	// nodeToChunksMap stores a map of the node IDs of the source Artifactory instance,
	// In each node, we store a map of the chunks that are currently in progress and their matching files.
	// In case network fails, and the uploaded chunks data is lost,
	// These chunks files will be written to the errors file using this map.
	nodeToChunksMap map[nodeId]map[chunkId][]FileRepresentation
	// Counts the total of chunks that are currently in progress by the source Artifactory instance.
	totalChunks int
}

func (clcm *ChunksLifeCycleManager) GetInProgressTokensSlice() []chunkId {
	var inProgressTokens []chunkId
	for _, node := range clcm.nodeToChunksMap {
		for id := range node {
			inProgressTokens = append(inProgressTokens, id)
		}
	}

	return inProgressTokens
}

func (clcm *ChunksLifeCycleManager) GetInProgressTokensSliceByNodeId(nodeId nodeId) []chunkId {
	var inProgressTokens []chunkId
	for chunkId := range clcm.nodeToChunksMap[nodeId] {
		inProgressTokens = append(inProgressTokens, chunkId)
	}

	return inProgressTokens
}

type InterruptionErr struct{}

func (m *InterruptionErr) Error() string {
	return "Files transfer was interrupted by user"
}

func createTransferServiceManager(ctx context.Context, serverDetails *config.ServerDetails) (artifactory.ArtifactoryServicesManager, error) {
	return utils.CreateServiceManagerWithContext(ctx, serverDetails, false, 0, retries, retriesWaitMilliSecs)
}

func createSrcRtUserPluginServiceManager(ctx context.Context, sourceRtDetails *config.ServerDetails) (*srcUserPluginService, error) {
	serviceManager, err := createTransferServiceManager(ctx, sourceRtDetails)
	if err != nil {
		return nil, err
	}
	return NewSrcUserPluginService(serviceManager.GetConfig().GetServiceDetails(), serviceManager.Client()), nil
}

func runAql(ctx context.Context, sourceRtDetails *config.ServerDetails, query string) (result *serviceUtils.AqlSearchResult, err error) {
	serviceManager, err := createTransferServiceManager(ctx, sourceRtDetails)
	if err != nil {
		return nil, err
	}
	reader, err := serviceManager.Aql(query)
	if err != nil {
		return nil, err
	}
	defer func() {
		if reader != nil {
			e := reader.Close()
			if err == nil {
				err = errorutils.CheckError(e)
			}
		}
	}()

	respBody, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	result = &serviceUtils.AqlSearchResult{}
	err = json.Unmarshal(respBody, result)
	return result, errorutils.CheckError(err)
}

func createTargetAuth(targetRtDetails *config.ServerDetails, proxyKey string) TargetAuth {
	targetAuth := TargetAuth{
		TargetArtifactoryUrl: targetRtDetails.ArtifactoryUrl,
		TargetToken:          targetRtDetails.AccessToken,
		TargetProxyKey:       proxyKey,
	}
	if targetAuth.TargetToken == "" {
		targetAuth.TargetUsername = targetRtDetails.User
		targetAuth.TargetPassword = targetRtDetails.Password
	}
	return targetAuth
}

// This variable holds the total number of upload chunk that were sent to the source Artifactory instance to process.
// Together with this mutex, they control the load on the user plugin and couple it to the local number of threads.
var curProcessedUploadChunks = 0
var processedUploadChunksMutex sync.Mutex

// This function polls on chunks of files that were uploaded during one of the phases.
// It does so by requesting the status of each chunk, by sending the uuid token that was returned when the chunk was uploaded.
// Number of chunks is limited by the number of threads.
// Whenever the status of a chunk was received and is DONE, its token is removed from the tokens batch, making room for a new chunk to be uploaded
// and a new token to be polled on.
func pollUploads(phaseBase *phaseBase, srcUpService *srcUserPluginService, uploadChunkChan chan UploadedChunkData, doneChan chan bool, errorsChannelMng *ErrorsChannelMng) {
	curTokensBatch := UploadChunksStatusBody{}
	chunksLifeCycleManager := ChunksLifeCycleManager{
		deletedChunksSet: datastructures.MakeSet[chunkId](),
		nodeToChunksMap:  map[nodeId]map[chunkId][]FileRepresentation{},
	}
	curProcessedUploadChunks = 0
	var progressBar *TransferProgressMng
	var timeEstMng *timeEstimationManager
	if phaseBase != nil {
		progressBar = phaseBase.progressBar
		timeEstMng = phaseBase.timeEstMng
	}
	for {
		if ShouldStop(phaseBase, nil, errorsChannelMng) {
			return
		}
		time.Sleep(waitTimeBetweenChunkStatusSeconds * time.Second)
		// 'Working threads' are determined by how many upload chunks are currently being processed by the source Artifactory instance.
		if progressBar != nil {
			progressBar.SetRunningThreads(curProcessedUploadChunks)
		}

		// Each uploading thread receive a token and a node id from the source via the uploadChunkChan, so this go routine can poll on its status.
		fillChunkDataBatch(&chunksLifeCycleManager, uploadChunkChan)
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
		curTokensBatch = UploadChunksStatusBody{}
		removeDeletedChunksFromSet(chunksStatus.DeletedChunks, chunksLifeCycleManager.deletedChunksSet)
		toStop := handleChunksStatuses(phaseBase, &chunksStatus, progressBar, &chunksLifeCycleManager, timeEstMng, errorsChannelMng)
		if toStop {
			return
		}
	}
}

// Verify and handle in progress chunks synchronization between the CLI and the Source Artifactory instance
func checkChunkStatusSync(chunkStatus *UploadChunksStatusResponse, manager *ChunksLifeCycleManager, errorsChannelMng *ErrorsChannelMng) {
	// Compare between the number of chunks received from the latest syncChunks request to the chunks data we handle locally in nodeToChunksMap
	// If the number of the in progress chunks of a node within nodeToChunksMap differs from the chunkStatus received, There is missing data on the source side.
	if len(chunkStatus.ChunksStatus) != len(manager.nodeToChunksMap[nodeId(chunkStatus.NodeId)]) {
		// Get all the chunks uuids on the Artifactory side in a set of uuids
		chunksUuidsSetFromResponse := datastructures.MakeSet[chunkId]()
		for _, chunk := range chunkStatus.ChunksStatus {
			chunksUuidsSetFromResponse.Add(chunkId(chunk.UuidToken))
		}
		// Get all the chunks uuids on the CLI side
		chunksUuidsSliceFromMap := manager.GetInProgressTokensSliceByNodeId(nodeId(chunkStatus.NodeId))
		failedFile := FileUploadStatusResponse{
			Status:     Fail,
			StatusCode: SyncErrorStatusCode,
			Reason:     SyncErrorReason,
		}
		// Send all missing chunks from the source Artifactory instance to errorsChannelMng
		// Missing chunks are those that are inside chunksUuidsSliceFromMap but not in chunksUuidsSetFromResponse
		for _, chunkUuid := range chunksUuidsSliceFromMap {
			if !chunksUuidsSetFromResponse.Exists(chunkUuid) {
				for _, file := range manager.nodeToChunksMap[nodeId(chunkStatus.NodeId)][chunkUuid] {
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

// Send and handle.
func sendSyncChunksRequest(curTokensBatch UploadChunksStatusBody, chunksLifeCycleManager *ChunksLifeCycleManager, srcUpService *srcUserPluginService) (UploadChunksStatusResponse, error) {
	curTokensBatch.AwaitingStatusChunks = chunksLifeCycleManager.GetInProgressTokensSlice()
	curTokensBatch.ChunksToDelete = chunksLifeCycleManager.deletedChunksSet.ToSlice()
	chunksStatus, err := srcUpService.syncChunks(curTokensBatch)
	// Log the error only if the transfer wasn't interrupted by the user
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Error("error returned when getting upload chunks statuses: " + err.Error())
	}
	return chunksStatus, err
}

func removeDeletedChunksFromSet(deletedChunks []string, deletedChunksSet *datastructures.Set[chunkId]) {
	// deletedChunks is an array received from the source, confirming which chunks were deleted from the source side.
	// In deletedChunksSet, we keep only chunks for which we have yet to receive confirmation
	for _, deletedChunk := range deletedChunks {
		err := deletedChunksSet.Remove(chunkId(deletedChunk))
		if err != nil {
			log.Error(err.Error())
			continue
		}
	}
}

// handleChunksStatuses handles the chunk statuses from the response received from the source Artifactory Instance.
// It syncs the chunk status between the CLI and the source Artifactory instance,
// When a chunk is DONE, the progress bar is updated, and the number of working threads is decreased.
func handleChunksStatuses(phase *phaseBase, chunksStatus *UploadChunksStatusResponse, progressbar *TransferProgressMng,
	chunksLifeCycleManager *ChunksLifeCycleManager, timeEstMng *timeEstimationManager, errorsChannelMng *ErrorsChannelMng) bool {
	initialWorkingThreads := curProcessedUploadChunks
	checkChunkStatusSync(chunksStatus, chunksLifeCycleManager, errorsChannelMng)
	for _, chunk := range chunksStatus.ChunksStatus {
		if chunk.UuidToken == "" {
			log.Error("Unexpected empty uuid token in status")
			continue
		}
		switch chunk.Status {
		case InProgress:
			continue
		case Done:
			reduceCurProcessedChunks()
			log.Debug("Received status DONE for chunk '" + chunk.UuidToken + "'")

			err := updateProgress(phase, progressbar, timeEstMng, chunk, initialWorkingThreads)
			if err != nil {
				log.Error("Unexpected error in progress update: " + err.Error())
				continue
			}
			delete(chunksLifeCycleManager.nodeToChunksMap[nodeId(chunksStatus.NodeId)], chunkId(chunk.UuidToken))
			chunksLifeCycleManager.totalChunks--
			// Using the deletedChunksSet, we inform the source that the 'DONE' message has been received, and it no longer has to keep those chunks UUIDs.
			chunksLifeCycleManager.deletedChunksSet.Add(chunkId(chunk.UuidToken))
			stopped := handleFilesOfCompletedChunk(chunk.Files, errorsChannelMng)
			// In case an error occurred while writing errors status's to the errors file - stop transferring.
			if stopped {
				return true
			}
		}
	}
	return false
}

func updateProgress(phase *phaseBase, progressbar *TransferProgressMng, timeEstMng *timeEstimationManager, chunk ChunkStatus, workingThreads int) error {
	if phase == nil {
		return nil
	}
	includedInTotalSize := false
	if phase.phaseId == FullTransferPhase || phase.phaseId == ErrorsPhase {
		includedInTotalSize = true
		if progressbar != nil {
			err := progressbar.IncrementPhaseBy(phase.phaseId, len(chunk.Files))
			if err != nil {
				return err
			}
		}
	}
	if timeEstMng != nil {
		timeEstMng.addChunkStatus(chunk, workingThreads, includedInTotalSize)
	}
	return nil
}

// Checks whether the total number of upload chunks sent is lower than the number of threads, and if so, increments it.
// Returns true if the total number was indeed incremented.
func incrCurProcessedChunksWhenPossible() bool {
	processedUploadChunksMutex.Lock()
	defer processedUploadChunksMutex.Unlock()
	if curProcessedUploadChunks < GetThreads() {
		curProcessedUploadChunks++
		return true
	}
	return false
}

// Reduces the current total number of upload chunks processed. Called when an upload chunks doesn't require polling for status -
// if it's done processing, or an error occurred when sending it.
func reduceCurProcessedChunks() {
	processedUploadChunksMutex.Lock()
	defer processedUploadChunksMutex.Unlock()
	curProcessedUploadChunks--
}

func handleFilesOfCompletedChunk(chunkFiles []FileUploadStatusResponse, errorsChannelMng *ErrorsChannelMng) (stopped bool) {
	for _, file := range chunkFiles {
		switch file.Status {
		case Success:
		case SkippedMetadataFile:
			// Skipping metadata on purpose - no need to write error.
		case Fail, SkippedLargeProps:
			stopped = addErrorToChannel(errorsChannelMng, file)
			if stopped {
				return
			}
		}
	}
	return
}

// Uploads chunk when there is room in queue.
// This is a blocking method.
func uploadChunkWhenPossible(phaseBase *phaseBase, chunk UploadChunk, uploadTokensChan chan UploadedChunkData, errorsChannelMng *ErrorsChannelMng) (stopped bool) {
	for {
		if ShouldStop(phaseBase, nil, errorsChannelMng) {
			return true
		}
		// If increment done, this go routine can proceed to upload the chunk. Otherwise, sleep and try again.
		isIncr := incrCurProcessedChunksWhenPossible()
		if !isIncr {
			time.Sleep(waitTimeBetweenChunkStatusSeconds * time.Second)
			continue
		}
		err := uploadChunkAndAddToken(phaseBase.srcUpService, chunk, uploadTokensChan)
		if err != nil {
			// Chunk not uploaded due to error. Reduce processed chunks count and send all chunk content to error channel, so that the files could be uploaded on next run.
			reduceCurProcessedChunks()
			// If the transfer is interrupted by the user, we shouldn't write it in the CSV file
			if errors.Is(err, context.Canceled) {
				return true
			}
			return sendAllChunkToErrorChannel(chunk, errorsChannelMng, err)
		}
		return ShouldStop(phaseBase, nil, errorsChannelMng)
	}
}

func sendAllChunkToErrorChannel(chunk UploadChunk, errorsChannelMng *ErrorsChannelMng, err error) (stopped bool) {
	for _, file := range chunk.UploadCandidates {
		err := FileUploadStatusResponse{
			FileRepresentation: file,
			Reason:             err.Error(),
		}
		// In case an error occurred while handling errors files - stop transferring.
		stopped = addErrorToChannel(errorsChannelMng, err)
		if stopped {
			return
		}
	}
	return
}

// Sends an upload chunk to the source Artifactory instance, to be handled asynchronously by the data-transfer plugin.
// An uuid token is returned in order to poll on it for status.
// This function sends the token to the uploadTokensChan for the pollUploads function to read and poll on.
func uploadChunkAndAddToken(sup *srcUserPluginService, chunk UploadChunk, uploadTokensChan chan UploadedChunkData) error {
	uploadResponse, err := sup.uploadChunk(chunk)
	if err != nil {
		return err
	}

	// Add chunk data for polling.
	log.Debug("Chunk sent to node" + uploadResponse.NodeId + ". Adding chunk token '" + uploadResponse.UuidToken + "' to poll on for status.")
	uploadedChunkData := UploadedChunkData{
		ChunkUuid:      uploadResponse.UuidToken,
		ChunkFiles:     chunk.UploadCandidates,
		NodeIdResponse: uploadResponse.NodeIdResponse,
	}
	uploadTokensChan <- uploadedChunkData
	return nil
}

func GetThreads() int {
	return curThreads
}

// Periodically reads settings file and updates the number of threads.
// Number of threads in the settings files is expected to change by running a separate command.
// The new number of threads should be almost immediately (checked every waitTimeBetweenThreadsUpdateSeconds) reflected on
// the CLI side (by updating the producer consumer if used and the local variable) and as a result reflected on the Artifactory User Plugin side.
func periodicallyUpdateThreads(pcWrapper *producerConsumerWrapper, doneChan chan bool, buildInfoRepo bool) {
	for {
		time.Sleep(waitTimeBetweenThreadsUpdateSeconds * time.Second)
		if shouldStopPolling(doneChan) {
			return
		}
		err := updateThreads(pcWrapper, buildInfoRepo)
		if err != nil {
			log.Error(err)
		}
	}
}

func updateThreads(pcWrapper *producerConsumerWrapper, buildInfoRepo bool) error {
	settings, err := utils.LoadTransferSettings()
	if err != nil || settings == nil {
		return err
	}
	calculatedNumberOfThreads := settings.CalcNumberOfThreads(buildInfoRepo)
	if curThreads != calculatedNumberOfThreads {
		curThreads = calculatedNumberOfThreads
		if pcWrapper != nil {
			updateProducerConsumerMaxParallel(pcWrapper.chunkBuilderProducerConsumer, calculatedNumberOfThreads)
			updateProducerConsumerMaxParallel(pcWrapper.chunkUploaderProducerConsumer, calculatedNumberOfThreads)
		}
		log.Info("Number of threads have been updated to " + strconv.Itoa(curThreads))
	}
	return nil
}

func updateProducerConsumerMaxParallel(producerConsumer parallel.Runner, calculatedNumberOfThreads int) {
	if producerConsumer != nil {
		producerConsumer.SetMaxParallel(calculatedNumberOfThreads)
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

func uploadChunkWhenPossibleHandler(pcWrapper *producerConsumerWrapper, phaseBase *phaseBase, chunk UploadChunk,
	uploadTokensChan chan UploadedChunkData, errorsChannelMng *ErrorsChannelMng) parallel.TaskFunc {
	return func(threadId int) error {
		logMsgPrefix := clientUtils.GetLogMsgPrefix(threadId, false)
		log.Debug(logMsgPrefix + "Handling chunk upload")
		shouldStop := uploadChunkWhenPossible(phaseBase, chunk, uploadTokensChan, errorsChannelMng)
		pcWrapper.notifyIfUploaderFinished()
		if shouldStop {
			// The specific error that triggered the stop is already in the errors channel
			return errorutils.CheckErrorf("%sstopped.", logMsgPrefix)
		}
		return nil
	}
}

// Collects files in chunks of size uploadChunkSize and sends them to be uploaded whenever possible (the amount of chunks uploaded is limited by the number of threads).
// An uuid token is returned after the chunk is sent and is being polled on for status.
func uploadByChunks(files []FileRepresentation, uploadTokensChan chan UploadedChunkData, base phaseBase, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng, pcWrapper *producerConsumerWrapper) (shouldStop bool, err error) {
	curUploadChunk := UploadChunk{
		TargetAuth:                createTargetAuth(base.targetRtDetails, base.proxyKey),
		CheckExistenceInFilestore: base.checkExistenceInFilestore,
	}

	for _, item := range files {
		file := FileRepresentation{Repo: item.Repo, Path: item.Path, Name: item.Name}
		var delayed bool
		delayed, shouldStop = delayHelper.delayUploadIfNecessary(base, file)
		if shouldStop {
			return
		}
		if delayed {
			continue
		}
		curUploadChunk.appendUploadCandidateIfNeeded(file, base.buildInfoRepo)
		if len(curUploadChunk.UploadCandidates) == uploadChunkSize {
			_, err = pcWrapper.chunkUploaderProducerConsumer.AddTaskWithError(uploadChunkWhenPossibleHandler(pcWrapper, &base, curUploadChunk, uploadTokensChan, errorsChannelMng), pcWrapper.errorsQueue.AddError)
			if err != nil {
				return
			}
			// Empty the uploaded chunk.
			curUploadChunk.UploadCandidates = []FileRepresentation{}
		}
	}
	// Chunk didn't reach full size. Upload the remaining files.
	if len(curUploadChunk.UploadCandidates) > 0 {
		_, err = pcWrapper.chunkUploaderProducerConsumer.AddTaskWithError(uploadChunkWhenPossibleHandler(pcWrapper, &base, curUploadChunk, uploadTokensChan, errorsChannelMng), pcWrapper.errorsQueue.AddError)
		if err != nil {
			return
		}
	}
	return
}

// Add a new error to the common error channel.
// In case an error occurs when creating the upload errors files, we would like to stop the transfer right away and stop adding elements to the channel.
func addErrorToChannel(errorsChannelMng *ErrorsChannelMng, file FileUploadStatusResponse) (stopped bool) {
	if errorsChannelMng.add(file) {
		log.Debug("Stop transferring data - error occurred while handling transfer's errors files.")
		return true
	}
	return false
}

// ShouldStop Stop transferring if one of the following happened:
// * Error occurred while handling errors (for example - not enough space in file system)
// * Error occurred during delayed artifacts handling
// * User interrupted the process (ctrl+c)
func ShouldStop(phase *phaseBase, delayHelper *delayUploadHelper, errorsChannelMng *ErrorsChannelMng) bool {
	if phase != nil && phase.ShouldStop() {
		log.Debug("Stop transferring data - Interrupted.")
		return true
	}
	if delayHelper != nil && delayHelper.delayedArtifactsChannelMng.shouldStop() {
		log.Debug("Stop transferring data - error occurred while handling transfer's delayed artifacts files.")
		return true
	}
	if errorsChannelMng != nil && errorsChannelMng.shouldStop() {
		log.Debug("Stop transferring data - error occurred while handling transfer's errors.")
		return true
	}
	return false
}

func getRunningNodes(ctx context.Context, sourceRtDetails *config.ServerDetails) ([]string, error) {
	serviceManager, err := createTransferServiceManager(ctx, sourceRtDetails)
	if err != nil {
		return nil, err
	}
	return serviceManager.GetRunningNodes()
}

func stopTransferInArtifactoryNodes(srcUpService *srcUserPluginService, runningNodes []string) {
	remainingNodesToStop := make(map[string]string)
	for _, s := range runningNodes {
		remainingNodesToStop[s] = s
	}
	log.Debug("Running Artifactory nodes to stop transfer on:", remainingNodesToStop)
	// Send a stop command up to 5 times the number of Artifactory nodes, to make sure we reach out to all nodes
	for i := 0; i < len(runningNodes)*5; i++ {
		if len(remainingNodesToStop) == 0 {
			log.Debug("Transfer on all Artifactory nodes stopped successfully")
			return
		}
		nodeId, err := srcUpService.stop()
		if err != nil {
			log.Error(err)
		} else {
			log.Debug("Node " + nodeId + " stopped")
			delete(remainingNodesToStop, nodeId)
		}
	}
}

// getMaxUniqueSnapshots gets the local repository's setting of max unique snapshots (Maven, Gradle, NuGet, Ivy and SBT)
// or max unique tags (Docker).
// For repositories of other package types or if an error is thrown, this function returns -1.
func getMaxUniqueSnapshots(ctx context.Context, rtDetails *config.ServerDetails, repoSummary *serviceUtils.RepositorySummary) (maxUniqueSnapshots int, err error) {
	maxUniqueSnapshots = -1
	serviceManager, err := createTransferServiceManager(ctx, rtDetails)
	if err != nil {
		return
	}
	switch repoSummary.PackageType {
	case maven:
		mavenLocalRepoParams := services.MavenLocalRepositoryParams{}
		err = serviceManager.GetRepository(repoSummary.RepoKey, &mavenLocalRepoParams)
		if err != nil {
			return
		}
		maxUniqueSnapshots = *mavenLocalRepoParams.MaxUniqueSnapshots
	case gradle:
		gradleLocalRepoParams := services.GradleLocalRepositoryParams{}
		err = serviceManager.GetRepository(repoSummary.RepoKey, &gradleLocalRepoParams)
		if err != nil {
			return
		}
		maxUniqueSnapshots = *gradleLocalRepoParams.MaxUniqueSnapshots
	case nuget:
		nugetLocalRepoParams := services.NugetLocalRepositoryParams{}
		err = serviceManager.GetRepository(repoSummary.RepoKey, &nugetLocalRepoParams)
		if err != nil {
			return
		}
		maxUniqueSnapshots = *nugetLocalRepoParams.MaxUniqueSnapshots
	case ivy:
		ivyLocalRepoParams := services.IvyLocalRepositoryParams{}
		err = serviceManager.GetRepository(repoSummary.RepoKey, &ivyLocalRepoParams)
		if err != nil {
			return
		}
		maxUniqueSnapshots = *ivyLocalRepoParams.MaxUniqueSnapshots
	case sbt:
		sbtLocalRepoParams := services.SbtLocalRepositoryParams{}
		err = serviceManager.GetRepository(repoSummary.RepoKey, &sbtLocalRepoParams)
		if err != nil {
			return
		}
		maxUniqueSnapshots = *sbtLocalRepoParams.MaxUniqueSnapshots
	case docker:
		dockerLocalRepoParams := services.DockerLocalRepositoryParams{}
		err = serviceManager.GetRepository(repoSummary.RepoKey, &dockerLocalRepoParams)
		if err != nil {
			return
		}
		maxUniqueSnapshots = *dockerLocalRepoParams.MaxUniqueTags
	}
	return
}

// updateMaxUniqueSnapshots updates the local repository's setting of max unique snapshots (Maven, Gradle, NuGet, Ivy and SBT)
// or max unique tags (Docker).
// For repositories of other package types, this function does nothing.
func updateMaxUniqueSnapshots(ctx context.Context, rtDetails *config.ServerDetails, repoSummary *serviceUtils.RepositorySummary, newMaxUniqueSnapshots int) error {
	serviceManager, err := createTransferServiceManager(ctx, rtDetails)
	if err != nil {
		return err
	}
	switch repoSummary.PackageType {
	case maven:
		return updateMaxMavenUniqueSnapshots(serviceManager, repoSummary, newMaxUniqueSnapshots)
	case gradle:
		return updateMaxGradleUniqueSnapshots(serviceManager, repoSummary, newMaxUniqueSnapshots)
	case nuget:
		return updateMaxNugetUniqueSnapshots(serviceManager, repoSummary, newMaxUniqueSnapshots)
	case ivy:
		return updateMaxIvyUniqueSnapshots(serviceManager, repoSummary, newMaxUniqueSnapshots)
	case sbt:
		return updateMaxSbtUniqueSnapshots(serviceManager, repoSummary, newMaxUniqueSnapshots)
	case docker:
		return updateMaxDockerUniqueSnapshots(serviceManager, repoSummary, newMaxUniqueSnapshots)
	}
	return nil
}

func updateMaxMavenUniqueSnapshots(serviceManager artifactory.ArtifactoryServicesManager, repoSummary *serviceUtils.RepositorySummary, newMaxUniqueSnapshots int) error {
	if strings.ToLower(repoSummary.RepoType) == services.FederatedRepositoryRepoType {
		repoParams := services.NewMavenFederatedRepositoryParams()
		repoParams.Key = repoSummary.RepoKey
		repoParams.MaxUniqueSnapshots = &newMaxUniqueSnapshots
		return serviceManager.UpdateFederatedRepository().Maven(repoParams)
	}
	repoParams := services.NewMavenLocalRepositoryParams()
	repoParams.Key = repoSummary.RepoKey
	repoParams.MaxUniqueSnapshots = &newMaxUniqueSnapshots
	return serviceManager.UpdateLocalRepository().Maven(repoParams)
}

func updateMaxGradleUniqueSnapshots(serviceManager artifactory.ArtifactoryServicesManager, repoSummary *serviceUtils.RepositorySummary, newMaxUniqueSnapshots int) error {
	if strings.ToLower(repoSummary.RepoType) == services.FederatedRepositoryRepoType {
		repoParams := services.NewGradleFederatedRepositoryParams()
		repoParams.Key = repoSummary.RepoKey
		repoParams.MaxUniqueSnapshots = &newMaxUniqueSnapshots
		return serviceManager.UpdateFederatedRepository().Gradle(repoParams)
	}
	repoParams := services.NewGradleLocalRepositoryParams()
	repoParams.Key = repoSummary.RepoKey
	repoParams.MaxUniqueSnapshots = &newMaxUniqueSnapshots
	return serviceManager.UpdateLocalRepository().Gradle(repoParams)
}

func updateMaxNugetUniqueSnapshots(serviceManager artifactory.ArtifactoryServicesManager, repoSummary *serviceUtils.RepositorySummary, newMaxUniqueSnapshots int) error {
	if strings.ToLower(repoSummary.RepoType) == services.FederatedRepositoryRepoType {
		repoParams := services.NewNugetFederatedRepositoryParams()
		repoParams.Key = repoSummary.RepoKey
		repoParams.MaxUniqueSnapshots = &newMaxUniqueSnapshots
		return serviceManager.UpdateFederatedRepository().Nuget(repoParams)
	}
	repoParams := services.NewNugetLocalRepositoryParams()
	repoParams.Key = repoSummary.RepoKey
	repoParams.MaxUniqueSnapshots = &newMaxUniqueSnapshots
	return serviceManager.UpdateLocalRepository().Nuget(repoParams)
}

func updateMaxIvyUniqueSnapshots(serviceManager artifactory.ArtifactoryServicesManager, repoSummary *serviceUtils.RepositorySummary, newMaxUniqueSnapshots int) error {
	if strings.ToLower(repoSummary.RepoType) == services.FederatedRepositoryRepoType {
		repoParams := services.NewIvyFederatedRepositoryParams()
		repoParams.Key = repoSummary.RepoKey
		repoParams.MaxUniqueSnapshots = &newMaxUniqueSnapshots
		return serviceManager.UpdateFederatedRepository().Ivy(repoParams)
	}
	repoParams := services.NewIvyLocalRepositoryParams()
	repoParams.Key = repoSummary.RepoKey
	repoParams.MaxUniqueSnapshots = &newMaxUniqueSnapshots
	return serviceManager.UpdateLocalRepository().Ivy(repoParams)
}

func updateMaxSbtUniqueSnapshots(serviceManager artifactory.ArtifactoryServicesManager, repoSummary *serviceUtils.RepositorySummary, newMaxUniqueSnapshots int) error {
	if strings.ToLower(repoSummary.RepoType) == services.FederatedRepositoryRepoType {
		repoParams := services.NewSbtFederatedRepositoryParams()
		repoParams.Key = repoSummary.RepoKey
		repoParams.MaxUniqueSnapshots = &newMaxUniqueSnapshots
		return serviceManager.UpdateFederatedRepository().Sbt(repoParams)
	}
	repoParams := services.NewSbtLocalRepositoryParams()
	repoParams.Key = repoSummary.RepoKey
	repoParams.MaxUniqueSnapshots = &newMaxUniqueSnapshots
	return serviceManager.UpdateLocalRepository().Sbt(repoParams)
}

func updateMaxDockerUniqueSnapshots(serviceManager artifactory.ArtifactoryServicesManager, repoSummary *serviceUtils.RepositorySummary, newMaxUniqueSnapshots int) error {
	if strings.ToLower(repoSummary.RepoType) == services.FederatedRepositoryRepoType {
		repoParams := services.NewDockerFederatedRepositoryParams()
		repoParams.Key = repoSummary.RepoKey
		repoParams.MaxUniqueTags = &newMaxUniqueSnapshots
		return serviceManager.UpdateFederatedRepository().Docker(repoParams)
	}
	repoParams := services.NewDockerLocalRepositoryParams()
	repoParams.Key = repoSummary.RepoKey
	repoParams.MaxUniqueTags = &newMaxUniqueSnapshots
	return serviceManager.UpdateLocalRepository().Docker(repoParams)
}

func stopTransferInArtifactory(ctx context.Context, serverDetails *config.ServerDetails, srcUpService *srcUserPluginService) error {
	runningNodes, err := getRunningNodes(ctx, serverDetails)
	if err != nil {
		return err
	} else {
		stopTransferInArtifactoryNodes(srcUpService, runningNodes)
	}
	return nil
}
