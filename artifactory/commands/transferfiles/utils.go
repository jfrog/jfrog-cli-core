package transferfiles

import (
	"encoding/json"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"io/ioutil"
	"strconv"
	"sync"
	"time"

	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	serviceUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	waitTimeBetweenChunkStatusSeconds            = 3
	waitTimeBetweenThreadsUpdateSeconds          = 20
	assumeProducerConsumerDoneWhenIdleForSeconds = 15
	DefaultAqlPaginationLimit                    = 10000
	maxBuildInfoRepoThreads                      = 8
)

var AqlPaginationLimit = DefaultAqlPaginationLimit
var curThreads int

type InterruptionErr struct{}

func (m *InterruptionErr) Error() string {
	return "Files transfer was interrupted by user"
}

type StoppableComponent interface {
	Stop()
	ShouldStop() bool
}

type Stoppable struct {
	stop bool
}

func (s *Stoppable) Stop() {
	s.stop = true
}

func (s *Stoppable) ShouldStop() bool {
	return s.stop
}

func createSrcRtUserPluginServiceManager(sourceRtDetails *coreConfig.ServerDetails) (*srcUserPluginService, error) {
	serviceManager, err := utils.CreateServiceManager(sourceRtDetails, retries, retriesWaitMilliSecs, false)
	if err != nil {
		return nil, err
	}
	return NewSrcUserPluginService(serviceManager.GetConfig().GetServiceDetails(), serviceManager.Client()), nil
}

func runAql(sourceRtDetails *coreConfig.ServerDetails, query string) (result *serviceUtils.AqlSearchResult, err error) {
	serviceManager, err := utils.CreateServiceManager(sourceRtDetails, retries, retriesWaitMilliSecs, false)
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

func createTargetAuth(targetRtDetails *coreConfig.ServerDetails) TargetAuth {
	targetAuth := TargetAuth{TargetArtifactoryUrl: targetRtDetails.ArtifactoryUrl,
		TargetToken: targetRtDetails.AccessToken}
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
func pollUploads(phaseBase *phaseBase, srcUpService *srcUserPluginService, uploadTokensChan chan string, doneChan chan bool, errorsChannelMng *ErrorsChannelMng, progressbar *TransferProgressMng) {
	curTokensBatch := UploadChunksStatusBody{}
	awaitingStatusChunksSet := datastructures.MakeSet[string]()
	deletedChunksSet := datastructures.MakeSet[string]()
	curProcessedUploadChunks = 0
	for {
		if ShouldStop(phaseBase, nil, errorsChannelMng) {
			return
		}
		time.Sleep(waitTimeBetweenChunkStatusSeconds * time.Second)
		// 'Working threads' are determined by how many upload chunks are currently being processed by the source Artifactory instance.
		if progressbar != nil {
			progressbar.SetRunningThreads(curProcessedUploadChunks)
		}
		// Each uploading thread receive a token from the source via the uploadTokensChan, so this go routine can poll on its status.

		fillTokensBatch(awaitingStatusChunksSet, uploadTokensChan)
		// awaitingStatusChunksSet is used to keep all the uploaded chunks tokens in order to request their upload status from the source.
		// deletedChunksSet is used to notify the source that these chunks can be deleted from the source's status map.
		// After we receive 'DONE', we inform the source that the 'DONE' message has been received, and it no longer has to keep those chunks UUIDs.
		// When both deletedChunksSet and awaitingStatusChunksSet length is zero, it means that all the tokens are uploaded,
		// we received 'DONE' for all of them, and we notified the source that they can be deleted from the memory.
		if awaitingStatusChunksSet.Size() == 0 && deletedChunksSet.Size() == 0 {
			if shouldStopPolling(doneChan) {
				return
			}
			continue
		}

		// Send and handle.
		curTokensBatch.AwaitingStatusChunks = awaitingStatusChunksSet.ToSlice()
		curTokensBatch.ChunksToDelete = deletedChunksSet.ToSlice()
		chunksStatus, err := srcUpService.syncChunks(curTokensBatch)
		if err != nil {
			log.Error("error returned when getting upload chunks statuses: " + err.Error())
			continue
		}
		// Clear body for the next request
		curTokensBatch = UploadChunksStatusBody{}
		removeDeletedChunksFromSet(chunksStatus.DeletedChunks, deletedChunksSet)
		toStop := handleChunksStatuses(phaseBase, chunksStatus.ChunksStatus, progressbar, awaitingStatusChunksSet, deletedChunksSet, errorsChannelMng)
		if toStop {
			return
		}
	}
}

func removeDeletedChunksFromSet(deletedChunks []string, deletedChunksSet *datastructures.Set[string]) {
	for _, deletedChunk := range deletedChunks {
		err := deletedChunksSet.Remove(deletedChunk)
		if err != nil {
			log.Error(err.Error())
			continue
		}
	}
}

func handleChunksStatuses(phase *phaseBase, chunksStatus []ChunkStatus, progressbar *TransferProgressMng, awaitingStatusChunksSet *datastructures.Set[string], deletedChunksSet *datastructures.Set[string], errorsChannelMng *ErrorsChannelMng) bool {
	for _, chunk := range chunksStatus {
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
			if progressbar != nil && phase != nil && phase.phaseId == FullTransferPhase {
				err := progressbar.IncrementPhaseBy(phase.phaseId, len(chunk.Files))
				if err != nil {
					log.Error("Progressbar unexpected error: " + err.Error())
					continue
				}
			}
			err := awaitingStatusChunksSet.Remove(chunk.UuidToken)
			if err != nil {
				log.Error(err.Error())
			}
			deletedChunksSet.Add(chunk.UuidToken)
			stopped := handleFilesOfCompletedChunk(chunk.Files, errorsChannelMng)
			// In case an error occurred while writing errors status's to the errors file - stop transferring.
			if stopped {
				return true
			}
		}
	}
	return false
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
func uploadChunkWhenPossible(phaseBase *phaseBase, chunk UploadChunk, uploadTokensChan chan string, errorsChannelMng *ErrorsChannelMng) (stopped bool) {
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
func uploadChunkAndAddToken(sup *srcUserPluginService, chunk UploadChunk, uploadTokensChan chan string) error {
	uuidToken, err := sup.uploadChunk(chunk)
	if err != nil {
		return err
	}

	// Add token to polling.
	log.Debug("Chunk sent. Adding chunk token '" + uuidToken + "' to poll on for status.")
	uploadTokensChan <- uuidToken
	return nil
}

func GetThreads() int {
	return curThreads
}

// Periodically reads settings file and updates the number of threads.
// Number of threads in the settings files is expected to change by running a separate command.
// The new number of threads should be almost immediately (checked every waitTimeBetweenThreadsUpdateSeconds) reflected on
// the CLI side (by updating the producer consumer if used and the local variable) and as a result reflected on the Artifactory User Plugin side.
func periodicallyUpdateThreads(producerConsumer parallel.Runner, doneChan chan bool, buildInfoRepo bool) {
	for {
		time.Sleep(waitTimeBetweenThreadsUpdateSeconds * time.Second)
		if shouldStopPolling(doneChan) {
			return
		}
		err := updateThreads(producerConsumer, buildInfoRepo)
		if err != nil {
			log.Error(err)
		}
	}
}

func updateThreads(producerConsumer parallel.Runner, buildInfoRepo bool) error {
	settings, err := utils.LoadTransferSettings()
	if err != nil || settings == nil {
		return err
	}
	calculatedNumberOfThreads := settings.CalcNumberOfThreads(buildInfoRepo)
	if settings != nil && curThreads != calculatedNumberOfThreads {
		curThreads = calculatedNumberOfThreads
		if producerConsumer != nil {
			producerConsumer.SetMaxParallel(calculatedNumberOfThreads)
		}
		log.Info("Number of threads have been updated to " + strconv.Itoa(curThreads))
	}
	return nil
}

func shouldStopPolling(doneChan chan bool) bool {
	select {
	case done := <-doneChan:
		return done
	default:
	}
	return false
}

func uploadChunkWhenPossibleHandler(phaseBase *phaseBase, chunk UploadChunk, uploadTokensChan chan string, errorsChannelMng *ErrorsChannelMng) parallel.TaskFunc {
	return func(threadId int) error {
		logMsgPrefix := clientUtils.GetLogMsgPrefix(threadId, false)
		log.Debug(logMsgPrefix + "Handling chunk upload")
		shouldStop := uploadChunkWhenPossible(phaseBase, chunk, uploadTokensChan, errorsChannelMng)
		if shouldStop {
			// The specific error that triggered the stop is already in the errors channel
			return errorutils.CheckErrorf("%s stopped.", logMsgPrefix)
		}
		return nil
	}
}

// Collects files in chunks of size uploadChunkSize and sends them to be uploaded whenever possible (the amount of chunks uploaded is limited by the number of threads).
// An uuid token is returned after the chunk is sent and is being polled on for status.
func uploadByChunks(files []FileRepresentation, uploadTokensChan chan string, base phaseBase, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng, pcWrapper *producerConsumerWrapper) (shouldStop bool, err error) {
	curUploadChunk := UploadChunk{
		TargetAuth:                createTargetAuth(base.targetRtDetails),
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
		curUploadChunk.appendUploadCandidate(file)
		if len(curUploadChunk.UploadCandidates) == uploadChunkSize {
			_, err = pcWrapper.chunkUploaderProducerConsumer.AddTaskWithError(uploadChunkWhenPossibleHandler(&base, curUploadChunk, uploadTokensChan, errorsChannelMng), pcWrapper.errorsQueue.AddError)
			if err != nil {
				return
			}
			// Empty the uploaded chunk.
			curUploadChunk.UploadCandidates = []FileRepresentation{}
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

func getRunningNodes(sourceRtDetails *coreConfig.ServerDetails) ([]string, error) {
	serviceManager, err := utils.CreateServiceManager(sourceRtDetails, retries, retriesWaitMilliSecs, false)
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
func getMaxUniqueSnapshots(rtDetails *coreConfig.ServerDetails, repoSummary *serviceUtils.RepositorySummary) (maxUniqueSnapshots int, err error) {
	maxUniqueSnapshots = -1
	serviceManager, err := utils.CreateServiceManager(rtDetails, retries, retriesWaitMilliSecs, false)
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
func updateMaxUniqueSnapshots(rtDetails *coreConfig.ServerDetails, repoSummary *serviceUtils.RepositorySummary, newMaxUniqueSnapshots int) error {
	serviceManager, err := utils.CreateServiceManager(rtDetails, retries, retriesWaitMilliSecs, false)
	if err != nil {
		return err
	}
	switch repoSummary.PackageType {
	case maven:
		mavenLocalRepoParams := services.MavenLocalRepositoryParams{}
		mavenLocalRepoParams.Key = repoSummary.RepoKey
		mavenLocalRepoParams.MaxUniqueSnapshots = &newMaxUniqueSnapshots
		err = serviceManager.UpdateLocalRepository().Maven(mavenLocalRepoParams)
		if err != nil {
			return err
		}
	case gradle:
		gradleLocalRepoParams := services.GradleLocalRepositoryParams{}
		gradleLocalRepoParams.Key = repoSummary.RepoKey
		gradleLocalRepoParams.MaxUniqueSnapshots = &newMaxUniqueSnapshots
		err = serviceManager.UpdateLocalRepository().Gradle(gradleLocalRepoParams)
		if err != nil {
			return err
		}
	case nuget:
		nugetLocalRepoParams := services.NugetLocalRepositoryParams{}
		nugetLocalRepoParams.Key = repoSummary.RepoKey
		nugetLocalRepoParams.MaxUniqueSnapshots = &newMaxUniqueSnapshots
		err = serviceManager.UpdateLocalRepository().Nuget(nugetLocalRepoParams)
		if err != nil {
			return err
		}
	case ivy:
		ivyLocalRepoParams := services.IvyLocalRepositoryParams{}
		ivyLocalRepoParams.Key = repoSummary.RepoKey
		ivyLocalRepoParams.MaxUniqueSnapshots = &newMaxUniqueSnapshots
		err = serviceManager.UpdateLocalRepository().Ivy(ivyLocalRepoParams)
		if err != nil {
			return err
		}
	case sbt:
		sbtLocalRepoParams := services.SbtLocalRepositoryParams{}
		sbtLocalRepoParams.Key = repoSummary.RepoKey
		sbtLocalRepoParams.MaxUniqueSnapshots = &newMaxUniqueSnapshots
		err = serviceManager.UpdateLocalRepository().Sbt(sbtLocalRepoParams)
		if err != nil {
			return err
		}
	case docker:
		dockerLocalRepoParams := services.DockerLocalRepositoryParams{}
		dockerLocalRepoParams.Key = repoSummary.RepoKey
		dockerLocalRepoParams.MaxUniqueTags = &newMaxUniqueSnapshots
		err = serviceManager.UpdateLocalRepository().Docker(dockerLocalRepoParams)
		if err != nil {
			return err
		}
	}
	return nil
}

func stopTransferInArtifactory(serverDetails *coreConfig.ServerDetails, srcUpService *srcUserPluginService) error {
	runningNodes, err := getRunningNodes(serverDetails)
	if err != nil {
		return err
	} else {
		stopTransferInArtifactoryNodes(srcUpService, runningNodes)
	}
	return nil
}
