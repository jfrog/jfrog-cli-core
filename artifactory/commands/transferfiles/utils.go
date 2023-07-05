package transferfiles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	buildInfoUtils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/reposnapshot"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	serviceUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	waitTimeBetweenChunkStatusSeconds   = 3
	waitTimeBetweenThreadsUpdateSeconds = 20
	DefaultAqlPaginationLimit           = 10000

	SyncErrorReason     = "un-synchronized chunk status due to network issue"
	SyncErrorStatusCode = 404

	StopFileName = "stop"
)

type (
	nodeId string
)

var AqlPaginationLimit = DefaultAqlPaginationLimit
var curThreads int

type UploadedChunk struct {
	api.UploadChunkResponse
	UploadedChunkData
}

type UploadedChunkData struct {
	ChunkFiles []api.FileRepresentation
	TimeSent   time.Time
}

type ChunksLifeCycleManager struct {
	// deletedChunksSet stores chunk uuids that have received a 'DONE' response from the source Artifactory instance
	// It is used to notify the source Artifactory instance that these chunks can be deleted from the source's status map.
	deletedChunksSet *datastructures.Set[api.ChunkId]
	// nodeToChunksMap stores a map of the node IDs of the source Artifactory instance,
	// In each node, we store a map of the chunks that are currently in progress and their matching files.
	// In case network fails, and the uploaded chunks data is lost,
	// These chunks files will be written to the errors file using this map.
	nodeToChunksMap map[nodeId]map[api.ChunkId]UploadedChunkData
	// Counts the total of chunks that are currently in progress by the source Artifactory instance.
	totalChunks int
}

func (clcm *ChunksLifeCycleManager) GetInProgressTokensSlice() []api.ChunkId {
	var inProgressTokens []api.ChunkId
	for _, node := range clcm.nodeToChunksMap {
		for id := range node {
			inProgressTokens = append(inProgressTokens, id)
		}
	}

	return inProgressTokens
}

func (clcm *ChunksLifeCycleManager) GetInProgressTokensSliceByNodeId(nodeId nodeId) []api.ChunkId {
	var inProgressTokens []api.ChunkId
	for chunkId := range clcm.nodeToChunksMap[nodeId] {
		inProgressTokens = append(inProgressTokens, chunkId)
	}

	return inProgressTokens
}

// Save in the TransferRunStatus the chunks that have been in transit for more than 30 minutes.
// This allows them to be displayed using the '--status' option.
// stateManager - Transfer state manager
func (clcm *ChunksLifeCycleManager) StoreStaleChunks(stateManager *state.TransferStateManager) error {
	var staleChunks []state.StaleChunks
	for nodeId, chunkIdToData := range clcm.nodeToChunksMap {
		staleNodeChunks := state.StaleChunks{NodeID: string(nodeId)}
		for chunkId, uploadedChunkData := range chunkIdToData {
			if time.Since(uploadedChunkData.TimeSent).Hours() < 0.5 {
				continue
			}
			staleNodeChunk := state.StaleChunk{
				ChunkID: string(chunkId),
				Sent:    uploadedChunkData.TimeSent.Unix(),
			}
			for _, file := range uploadedChunkData.ChunkFiles {
				var sizeStr string
				if file.Size > 0 {
					sizeStr = " (" + utils.ConvertIntToStorageSizeString(file.Size) + ")"
				}
				staleNodeChunk.Files = append(staleNodeChunk.Files, path.Join(file.Repo, file.Path, file.Name)+sizeStr)
			}
			staleNodeChunks.Chunks = append(staleNodeChunks.Chunks, staleNodeChunk)
		}
		if len(staleNodeChunks.Chunks) > 0 {
			staleChunks = append(staleChunks, staleNodeChunks)
		}
	}
	return stateManager.SetStaleChunks(staleChunks)
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
			err = errors.Join(err, errorutils.CheckError(reader.Close()))
		}
	}()

	respBody, err := io.ReadAll(reader)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	result = &serviceUtils.AqlSearchResult{}
	err = json.Unmarshal(respBody, result)
	return result, errorutils.CheckError(err)
}

func createTargetAuth(targetRtDetails *config.ServerDetails, proxyKey string) api.TargetAuth {
	targetAuth := api.TargetAuth{
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

func handleFilesOfCompletedChunk(chunkFiles []api.FileUploadStatusResponse, errorsChannelMng *ErrorsChannelMng) (stopped bool) {
	for _, file := range chunkFiles {
		if file.Status == api.Fail || file.Status == api.SkippedLargeProps {
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
func uploadChunkWhenPossible(phaseBase *phaseBase, chunk api.UploadChunk, uploadTokensChan chan UploadedChunk, errorsChannelMng *ErrorsChannelMng) (stopped bool) {
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
			return sendAllChunkToErrorChannel(chunk, errorsChannelMng, err, phaseBase.stateManager)
		}
		return ShouldStop(phaseBase, nil, errorsChannelMng)
	}
}

func sendAllChunkToErrorChannel(chunk api.UploadChunk, errorsChannelMng *ErrorsChannelMng, errReason error, stateManager *state.TransferStateManager) (stopped bool) {
	var failures []api.FileUploadStatusResponse
	for _, file := range chunk.UploadCandidates {
		fileFailureResponse := api.FileUploadStatusResponse{
			FileRepresentation: file,
			Reason:             errReason.Error(),
		}
		// In case an error occurred while handling errors files - stop transferring.
		stopped = addErrorToChannel(errorsChannelMng, fileFailureResponse)
		if stopped {
			return
		}
		failures = append(failures, fileFailureResponse)
	}
	err := setChunkCompletedInRepoSnapshot(stateManager, failures)
	if err != nil {
		// We are logging the error instead of returning it since the original error is already handled.
		log.Error(err)
	}
	return
}

// If repo snapshot is tracked, mark all files of a chunk as completed in their directory's node and check if node completed (done handling the directory and child directories).
func setChunkCompletedInRepoSnapshot(stateManager *state.TransferStateManager, chunkFiles []api.FileUploadStatusResponse) (err error) {
	if !stateManager.IsRepoTransferSnapshotEnabled() {
		return
	}

	var dirNode *reposnapshot.Node
	for _, file := range chunkFiles {
		dirNode, err = stateManager.GetDirectorySnapshotNodeWithLru(file.Path)
		if err != nil {
			return
		}

		// If empty dir, skip to checking completion.
		if file.Name != "" {
			if err = dirNode.DecrementFilesCount(); err != nil {
				return
			}
		}

		if err = dirNode.CheckCompleted(); err != nil {
			return
		}
	}
	return
}

// Sends an upload chunk to the source Artifactory instance, to be handled asynchronously by the data-transfer plugin.
// An uuid token is returned in order to poll on it for status.
// This function sends the token to the uploadTokensChan for the pollUploads function to read and poll on.
func uploadChunkAndAddToken(sup *srcUserPluginService, chunk api.UploadChunk, uploadTokensChan chan UploadedChunk) error {
	uploadResponse, err := sup.uploadChunk(chunk)
	if err != nil {
		return err
	}

	// Add chunk data for polling.
	log.Debug("Chunk sent to node " + uploadResponse.NodeId + ". Adding chunk token '" + uploadResponse.UuidToken + "' to poll on for status.")
	uploadTokensChan <- newUploadedChunkStruct(uploadResponse, chunk)
	return nil
}

func newUploadedChunkStruct(uploadChunkResponse api.UploadChunkResponse, chunk api.UploadChunk) UploadedChunk {
	return UploadedChunk{
		UploadChunkResponse: uploadChunkResponse,
		UploadedChunkData: UploadedChunkData{
			ChunkFiles: chunk.UploadCandidates,
			TimeSent:   time.Now(),
		},
	}
}

func GetThreads() int {
	return curThreads
}

// Periodically reads settings file and updates the number of threads.
// Number of threads in the settings files is expected to change by running a separate command.
// The new number of threads should be almost immediately (checked every waitTimeBetweenThreadsUpdateSeconds) reflected on
// the CLI side (by updating the producer consumer if used and the local variable) and as a result reflected on the Artifactory User Plugin side.
// This method also looks for '~/.jfrog/transfer/stop' file and interrupts the transfer if exists.
func periodicallyUpdateThreadsAndStopStatus(pcWrapper *producerConsumerWrapper, doneChan chan bool, buildInfoRepo bool, stopSignal chan os.Signal) {
	log.Debug("Initializing polling on the settings and stop files...")
	for {
		time.Sleep(waitTimeBetweenThreadsUpdateSeconds * time.Second)
		if err := interruptIfRequested(stopSignal); err != nil {
			log.Error(err)
		}
		if shouldStopPolling(doneChan) {
			log.Debug("Stopping the polling on the settings and stop files for the current phase.")
			return
		}
		if err := updateThreads(pcWrapper, buildInfoRepo); err != nil {
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
		if pcWrapper != nil {
			updateProducerConsumerMaxParallel(pcWrapper.chunkBuilderProducerConsumer, calculatedNumberOfThreads)
			updateProducerConsumerMaxParallel(pcWrapper.chunkUploaderProducerConsumer, calculatedNumberOfThreads)
		}
		log.Info(fmt.Sprintf("Number of threads have been updated to %s (was %s).", strconv.Itoa(calculatedNumberOfThreads), strconv.Itoa(curThreads)))
		curThreads = calculatedNumberOfThreads
	} else {
		log.Debug("No change to the number of threads have been detected.")
	}
	return nil
}

// Interrupt the transfer by populating the stopSignal channel with the Interrupt signal if the '~/.jfrog/transfer/stop' file exists.
func interruptIfRequested(stopSignal chan os.Signal) error {
	transferDir, err := coreutils.GetJfrogTransferDir()
	if err != nil {
		return err
	}
	exist, err := fileutils.IsFileExists(filepath.Join(transferDir, StopFileName), false)
	if err != nil {
		return err
	}
	if exist {
		stopSignal <- os.Interrupt
	}
	return nil
}

func updateProducerConsumerMaxParallel(producerConsumer parallel.Runner, calculatedNumberOfThreads int) {
	if producerConsumer != nil {
		producerConsumer.SetMaxParallel(calculatedNumberOfThreads)
	}
}

func uploadChunkWhenPossibleHandler(phaseBase *phaseBase, chunk api.UploadChunk,
	uploadTokensChan chan UploadedChunk, errorsChannelMng *ErrorsChannelMng) parallel.TaskFunc {
	return func(threadId int) error {
		logMsgPrefix := clientUtils.GetLogMsgPrefix(threadId, false)
		log.Debug(logMsgPrefix + "Handling chunk upload")
		shouldStop := uploadChunkWhenPossible(phaseBase, chunk, uploadTokensChan, errorsChannelMng)
		if shouldStop {
			// The specific error that triggered the stop is already in the errors channel
			return errorutils.CheckErrorf(logMsgPrefix + "stopped")
		}
		return nil
	}
}

// Collects files in chunks of size uploadChunkSize and sends them to be uploaded whenever possible (the amount of chunks uploaded is limited by the number of threads).
// An uuid token is returned after the chunk is sent and is being polled on for status.
func uploadByChunks(files []api.FileRepresentation, uploadTokensChan chan UploadedChunk, base phaseBase, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng, pcWrapper *producerConsumerWrapper) (shouldStop bool, err error) {
	curUploadChunk := api.UploadChunk{
		TargetAuth:                createTargetAuth(base.targetRtDetails, base.proxyKey),
		CheckExistenceInFilestore: base.checkExistenceInFilestore,
		SkipFileFiltering:         base.locallyGeneratedFilter.IsEnabled(),
	}

	for _, item := range files {
		file := api.FileRepresentation{Repo: item.Repo, Path: item.Path, Name: item.Name, Size: item.Size}
		var delayed bool
		delayed, shouldStop = delayHelper.delayUploadIfNecessary(base, file)
		if shouldStop {
			return
		}
		if delayed {
			continue
		}
		curUploadChunk.AppendUploadCandidateIfNeeded(file, base.buildInfoRepo)
		if len(curUploadChunk.UploadCandidates) == uploadChunkSize {
			_, err = pcWrapper.chunkUploaderProducerConsumer.AddTaskWithError(uploadChunkWhenPossibleHandler(&base, curUploadChunk, uploadTokensChan, errorsChannelMng), pcWrapper.errorsQueue.AddError)
			if err != nil {
				return
			}
			// Empty the uploaded chunk.
			curUploadChunk.UploadCandidates = []api.FileRepresentation{}
		}
	}
	// Chunk didn't reach full size. Upload the remaining files.
	if len(curUploadChunk.UploadCandidates) > 0 {
		_, err = pcWrapper.chunkUploaderProducerConsumer.AddTaskWithError(uploadChunkWhenPossibleHandler(&base, curUploadChunk, uploadTokensChan, errorsChannelMng), pcWrapper.errorsQueue.AddError)
		if err != nil {
			return
		}
	}
	return
}

// Add a new error to the common error channel.
// In case an error occurs when creating the upload errors files, we would like to stop the transfer right away and stop adding elements to the channel.
func addErrorToChannel(errorsChannelMng *ErrorsChannelMng, file api.FileUploadStatusResponse) (stopped bool) {
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

func stopTransferInArtifactory(serverDetails *config.ServerDetails, srcUpService *srcUserPluginService) error {
	// To avoid situations where context has already been canceled, we use a new context here instead of the old context of the transfer phase.
	runningNodes, err := getRunningNodes(context.Background(), serverDetails)
	if err != nil {
		return err
	} else {
		stopTransferInArtifactoryNodes(srcUpService, runningNodes)
	}
	return nil
}

func getJfrogTransferRepoDelaysDir(repoKey string) (string, error) {
	return state.GetJfrogTransferRepoSubDir(repoKey, coreutils.JfrogTransferDelaysDirName)
}

func getJfrogTransferRepoErrorsDir(repoKey string) (string, error) {
	return state.GetJfrogTransferRepoSubDir(repoKey, coreutils.JfrogTransferErrorsDirName)
}

func getJfrogTransferRepoErrorsSubDir(repoKey, subDirName string) (string, error) {
	errorsDir, err := getJfrogTransferRepoErrorsDir(repoKey)
	if err != nil {
		return "", err
	}
	return filepath.Join(errorsDir, subDirName), nil
}

func getJfrogTransferRepoRetryableDir(repoKey string) (string, error) {
	return getJfrogTransferRepoErrorsSubDir(repoKey, coreutils.JfrogTransferRetryableErrorsDirName)
}

func getJfrogTransferRepoSkippedDir(repoKey string) (string, error) {
	return getJfrogTransferRepoErrorsSubDir(repoKey, coreutils.JfrogTransferSkippedErrorsDirName)
}

func getErrorOrDelayFiles(repoKeys []string, getDirPathFunc func(string) (string, error)) (filesPaths []string, err error) {
	var dirPath string
	for _, repoKey := range repoKeys {
		dirPath, err = getDirPathFunc(repoKey)
		if err != nil {
			return []string{}, err
		}
		exist, err := buildInfoUtils.IsDirExists(dirPath, false)
		if err != nil {
			return []string{}, err
		}
		if !exist {
			continue
		}
		files, err := buildInfoUtils.ListFiles(dirPath, false)
		if err != nil {
			return nil, err
		}
		filesPaths = append(filesPaths, files...)
	}
	return
}

// Increments index until the file path is unique.
func getUniqueErrorOrDelayFilePath(dirPath string, getFileNamePrefix func() string) (delayFilePath string, err error) {
	var exists bool
	index := 0
	for {
		delayFilePath = filepath.Join(dirPath, fmt.Sprintf("%s-%d.json", getFileNamePrefix(), index))
		exists, err = fileutils.IsFileExists(delayFilePath, false)
		if err != nil {
			return "", err
		}
		if !exists {
			break
		}
		index++
	}
	return
}
