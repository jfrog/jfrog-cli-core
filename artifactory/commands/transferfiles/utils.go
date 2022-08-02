package transferfiles

import (
	"encoding/json"
	"io/ioutil"
	"strconv"
	"sync"
	"time"

	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	serviceUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	waitTimeBetweenChunkStatusSeconds            = 3
	waitTimeBetweenThreadsUpdateSeconds          = 20
	assumeProducerConsumerDoneWhenIdleForSeconds = 15
	aqlPaginationLimit                           = 10000
)

var curThreads int

type InterruptionErr struct{}

func (m *InterruptionErr) Error() string {
	return "Files transfer was interrupted by user"
}

func createSrcRtUserPluginServiceManager(sourceRtDetails *coreConfig.ServerDetails) (*srcUserPluginService, error) {
	serviceManager, err := utils.CreateServiceManager(sourceRtDetails, retries, retriesWait, false)
	if err != nil {
		return nil, err
	}
	return NewSrcUserPluginService(serviceManager.GetConfig().GetServiceDetails(), serviceManager.Client()), nil
}

func runAql(sourceRtDetails *coreConfig.ServerDetails, query string) (result *serviceUtils.AqlSearchResult, err error) {
	serviceManager, err := utils.CreateServiceManager(sourceRtDetails, retries, retriesWait, false)
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

// This function polls on chunks of files that were uploaded during one of the phases and were not checksum deployed.
// It does so by requesting the status of each chunk, by sending the uuid token that was returned when the chunk was uploaded.
// Number of chunks is limited by the number of threads.
// Whenever the status of a chunk was received and is DONE, its token is removed from the tokens batch, making room for a new chunk to be uploaded
// and a new token to be polled on.
func pollUploads(phaseBase *phaseBase, srcUpService *srcUserPluginService, uploadTokensChan chan string, doneChan chan bool, errorsChannelMng *ErrorsChannelMng, progressbar *progressbar.TransferProgressMng) {
	curTokensBatch := UploadChunksStatusBody{}
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
		curTokensBatch.fillTokensBatch(uploadTokensChan)

		if len(curTokensBatch.UuidTokens) == 0 {
			if shouldStopPolling(doneChan) {
				return
			}
			continue
		}

		// Send and handle.
		chunksStatus, err := srcUpService.getUploadChunksStatus(curTokensBatch)
		if err != nil {
			log.Error("error returned when getting upload chunks statuses: " + err.Error())
			continue
		}
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
				curTokensBatch.UuidTokens = removeTokenFromBatch(curTokensBatch.UuidTokens, chunk.UuidToken)
				stopped := handleFilesOfCompletedChunk(chunk.Files, errorsChannelMng)
				// In case an error occurred while writing errors status's to the errors file - stop transferring.
				if stopped {
					return
				}
			}
		}
	}
}

// Checks whether the total number of upload chunks sent is lower than the number of threads, and if so, increments it.
// Returns true if the total number was indeed incremented.
func incrCurProcessedChunksWhenPossible() bool {
	processedUploadChunksMutex.Lock()
	defer processedUploadChunksMutex.Unlock()
	if curProcessedUploadChunks < getThreads() {
		curProcessedUploadChunks++
		return true
	}
	return false
}

// Reduces the current total number of upload chunks processed. Called when an upload chunks doesn't require polling for status -
// if it's checksum deployed, or done processing.
func reduceCurProcessedChunks() {
	processedUploadChunksMutex.Lock()
	defer processedUploadChunksMutex.Unlock()
	curProcessedUploadChunks--
}

func removeTokenFromBatch(uuidTokens []string, token string) []string {
	for i := 0; i < len(uuidTokens); i++ {
		if token == uuidTokens[i] {
			return append(uuidTokens[:i], uuidTokens[i+1:]...)
		}
	}
	log.Error("Unexpected uuid token found: " + token)
	return uuidTokens
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
func uploadChunkWhenPossible(sup *srcUserPluginService, chunk UploadChunk, uploadTokensChan chan string, errorsChannelMng *ErrorsChannelMng) (stopped bool) {
	for {
		// If increment done, this go routine can proceed to upload the chunk. Otherwise, sleep and try again.
		isIncr := incrCurProcessedChunksWhenPossible()
		if !isIncr {
			time.Sleep(waitTimeBetweenChunkStatusSeconds * time.Second)
			continue
		}
		isChecksumDeployed, err := uploadChunkAndAddTokenIfNeeded(sup, chunk, uploadTokensChan)
		if err != nil {
			// Chunk not uploaded due to error. Reduce processed chunks count and send all chunk content to error channel, so that the files could be uploaded on next run.
			reduceCurProcessedChunks()
			return sendAllChunkToErrorChannel(chunk, errorsChannelMng, err)
		}
		if isChecksumDeployed {
			// Chunk does not require polling.
			reduceCurProcessedChunks()
		}
		return
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

// Sends an upload chunk to the source Artifactory instance.
// If not all files were successfully checksum deployed, an uuid token is returned in order to poll on it for status.
// If such token is indeed returned, this function sends it to the uploadTokensChan for the pollUploads function to read and poll.
func uploadChunkAndAddTokenIfNeeded(sup *srcUserPluginService, chunk UploadChunk, uploadTokensChan chan string) (bool, error) {
	uuidToken, err := sup.uploadChunk(chunk)
	if err != nil {
		return false, err
	}
	// Empty token is returned if all files were checksum deployed.
	if uuidToken == "" {
		return true, nil
	}

	// Add token to polling.
	uploadTokensChan <- uuidToken
	return false, nil
}

func verifyRepoExistsInTarget(targetRepos []string, srcRepoKey string) bool {
	for _, targetRepo := range targetRepos {
		if targetRepo == srcRepoKey {
			return true
		}
	}
	return false
}

func getThreads() int {
	return curThreads
}

// Periodically reads settings file and updates the number of threads.
// Number of threads in the settings files is expected to change by running a separate command.
// The new number of threads should be almost immediately (checked every waitTimeBetweenThreadsUpdateSeconds) reflected on
// the CLI side (by updating the producer consumer if used and the local variable) and as a result reflected on the Artifactory User Plugin side.
func periodicallyUpdateThreads(producerConsumer parallel.Runner, doneChan chan bool) {
	for {
		time.Sleep(waitTimeBetweenThreadsUpdateSeconds * time.Second)
		if shouldStopPolling(doneChan) {
			return
		}
		err := updateThreads(producerConsumer)
		if err != nil {
			log.Error(err)
		}
	}
}

func updateThreads(producerConsumer parallel.Runner) error {
	settings, err := utils.LoadTransferSettings()
	if err != nil {
		return err
	}
	if settings != nil && curThreads != settings.ThreadsNumber {
		curThreads = settings.ThreadsNumber
		if producerConsumer != nil {
			producerConsumer.SetMaxParallel(settings.ThreadsNumber)
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

func getRepoSummaryFromList(repoSummaryList []serviceUtils.RepositorySummary, repoKey string) (serviceUtils.RepositorySummary, error) {
	for i := range repoSummaryList {
		if repoKey == repoSummaryList[i].RepoKey {
			return repoSummaryList[i], nil
		}
	}
	return serviceUtils.RepositorySummary{}, errorutils.CheckErrorf("could not find repository '%s' in the repositories summary of the source instance", repoKey)
}

// Collects files in chunks of size uploadChunkSize and uploads them whenever possible (the amount of chunks uploaded is limited by the number of threads).
// If not all files were checksum deployed, an uuid token is returned and is being polled on for status.
func uploadByChunks(files []FileRepresentation, uploadTokensChan chan string, base phaseBase, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) (shouldStop bool, err error) {
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
			shouldStop = uploadChunkWhenPossible(base.srcUpService, curUploadChunk, uploadTokensChan, errorsChannelMng)
			if shouldStop {
				return
			}
			// Empty the uploaded chunk.
			curUploadChunk.UploadCandidates = []FileRepresentation{}
		}
	}
	// Chunk didn't reach full size. Upload the remaining files.
	if len(curUploadChunk.UploadCandidates) > 0 {
		shouldStop = uploadChunkWhenPossible(base.srcUpService, curUploadChunk, uploadTokensChan, errorsChannelMng)
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

// Stop transferring if one of the following happened:
// * Error occurred while handling errors (for example - not enough space in file system)
// * Error occureed during delayed artifacts handling
// * User interrupted the process (ctrl+c)
func ShouldStop(phase *phaseBase, delayHelper *delayUploadHelper, errorsChannelMng *ErrorsChannelMng) bool {
	if phase != nil && phase.shouldStop() {
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
	serviceManager, err := utils.CreateServiceManager(sourceRtDetails, retries, retriesWait, false)
	if err != nil {
		return nil, err
	}
	return serviceManager.GetRunningNodes()
}

func stopAllRunningNodes(srcUpService *srcUserPluginService, runningNodes []string) {
	remainingNodesToStop := make(map[string]string)
	for _, s := range runningNodes {
		remainingNodesToStop[s] = s
	}
	log.Debug("Running nodes to stop:", remainingNodesToStop)
	for i := 0; i < len(runningNodes)*3; i++ {
		if len(remainingNodesToStop) == 0 {
			log.Debug("All running nodes stopped successfully")
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
