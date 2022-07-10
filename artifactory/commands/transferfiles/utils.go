package transferfiles

import (
	"encoding/json"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	serviceUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io/ioutil"
	"strconv"
	"sync"
	"time"
)

const (
	waitTimeBetweenChunkStatusSeconds            = 3
	waitTimeBetweenThreadsUpdateSeconds          = 20
	assumeProducerConsumerDoneWhenIdleForSeconds = 15
)

var curThreads int

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
func pollUploads(srcUpService *srcUserPluginService, uploadTokensChan chan string, doneChan chan bool, errorsChannelMng *ErrorsChannelMng) error {
	curTokensBatch := UploadChunksStatusBody{}
	curProcessedUploadChunks = 0

	for {
		time.Sleep(waitTimeBetweenChunkStatusSeconds * time.Second)

		curTokensBatch.fillTokensBatch(uploadTokensChan)

		if len(curTokensBatch.UuidTokens) == 0 {
			if shouldStopPolling(doneChan) {
				return nil
			}
			continue
		}

		// Send and handle.
		chunksStatus, err := srcUpService.getUploadChunksStatus(curTokensBatch)
		if err != nil {
			return err
		}
		for _, chunk := range chunksStatus.ChunksStatus {
			switch chunk.Status {
			case InProgress:
				continue
			case Done:
				reduceCurProcessedChunks()
				curTokensBatch.UuidTokens = removeTokenFromBatch(curTokensBatch.UuidTokens, chunk.UuidToken)
				succeed := handleFilesOfCompletedChunk(chunk.Files, errorsChannelMng)
				// In case an error occurred while writing errors status's to the errors file we will stop transferring.
				if !succeed {
					return nil
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

func handleFilesOfCompletedChunk(chunkFiles []FileUploadStatusResponse, errorsChannelMng *ErrorsChannelMng) (succeed bool) {
	for _, file := range chunkFiles {
		switch file.Status {
		case Success:
		case SkippedMetadataFile:
			// Skipping metadata on purpose - no need to write error.
		case Fail:
			if !errorsChannelMng.add(file) {
				log.Debug("Stop transferring data - error occurred while handling transfer's errors files.")
				return false
			}
		case SkippedLargeProps:
			if !errorsChannelMng.add(file) {
				log.Debug("Stop transferring data - error occurred while handling transfer's errors files.")
				return false
			}
		}
	}
	return true
}

// Uploads chunk when there is room in queue.
// This is a blocking method.
func uploadChunkWhenPossible(sup *srcUserPluginService, chunk UploadChunk, uploadTokensChan chan string, errorsChannelMng *ErrorsChannelMng) (shouldStop bool, err error) {
	for {
		// If increment done, this go routine can proceed to upload the chunk. Otherwise, sleep and try again.
		isIncr := incrCurProcessedChunksWhenPossible()
		if !isIncr {
			time.Sleep(waitTimeBetweenChunkStatusSeconds * time.Second)
			continue
		}
		var isChecksumDeployed bool
		isChecksumDeployed, err = uploadChunkAndAddTokenIfNeeded(sup, chunk, uploadTokensChan)
		if err != nil {
			// Chunk not uploaded due to error. Reduce processed chunks count and send all chunk content to error channel, so that the files could be uploaded on next run.
			reduceCurProcessedChunks()
			shouldStop = sendAllChunkToErrorChannel(chunk, errorsChannelMng, err)
			return
		}
		if isChecksumDeployed {
			// Chunk does not require polling.
			reduceCurProcessedChunks()
		}
		return
	}
}

func sendAllChunkToErrorChannel(chunk UploadChunk, errorsChannelMng *ErrorsChannelMng, err error) (shouldStop bool) {
	for _, file := range chunk.UploadCandidates {
		err := FileUploadStatusResponse{
			FileRepresentation: file,
			Reason:             err.Error(),
		}
		// In case an error occurred while handling errors files - stop transferring.
		if !errorsChannelMng.add(err) {
			log.Debug("Stop transferring data - error occurred while handling transfer's errors files.")
			return true
		}
	}
	return false
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
		// In case an error occurred while handling errors/delayed artifacts files - stop transferring.
		if delayHelper.delayedArtifactsChannelMng.shouldStop() || errorsChannelMng.shouldStop() {
			log.Debug("Stop transferring data - error occurred while handling transfer's errors/delayed artifacts files.")
			return
		}
		file := FileRepresentation{Repo: item.Repo, Path: item.Path, Name: item.Name}
		delayed, ShouldStop := delayHelper.delayUploadIfNecessary(file)
		if ShouldStop {
			return
		}
		if delayed {
			continue
		}
		curUploadChunk.appendUploadCandidate(file)
		if len(curUploadChunk.UploadCandidates) == uploadChunkSize {
			shouldStop, err = uploadChunkWhenPossible(base.srcUpService, curUploadChunk, uploadTokensChan, errorsChannelMng)
			if err != nil || shouldStop {
				return
			}
			// Empty the uploaded chunk.
			curUploadChunk.UploadCandidates = []FileRepresentation{}
		}
	}
	// Chunk didn't reach full size. Upload the remaining files.
	if len(curUploadChunk.UploadCandidates) > 0 {
		shouldStop, err = uploadChunkWhenPossible(base.srcUpService, curUploadChunk, uploadTokensChan, errorsChannelMng)
	}
	return
}
