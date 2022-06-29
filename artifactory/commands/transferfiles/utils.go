package transferfiles

import (
	"encoding/json"
	biUtils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	artifactoryUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
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
	serviceManager, err := utils.CreateServiceManager(sourceRtDetails, 0, 0, false)
	if err != nil {
		return nil, err
	}
	return NewSrcUserPluginService(serviceManager.GetConfig().GetServiceDetails(), serviceManager.Client()), nil
}

func runAql(sourceRtDetails *coreConfig.ServerDetails, query string) (result *artifactoryUtils.AqlSearchResult, err error) {
	serviceManager, err := utils.CreateServiceManager(sourceRtDetails, -1, 0, false)
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

	result = &artifactoryUtils.AqlSearchResult{}
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
func pollUploads(srcUpService *srcUserPluginService, uploadTokensChan chan string, doneChan chan bool, errorChannel chan FileUploadStatusResponse) error {
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
				handleFilesOfCompletedChunk(chunk.Files, errorChannel)
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

func handleFilesOfCompletedChunk(chunkFiles []FileUploadStatusResponse, errorChannel chan FileUploadStatusResponse) {
	for _, file := range chunkFiles {
		switch file.Status {
		case Success:
		case Fail:
			errorChannel <- file
		case SkippedLargeProps:
			errorChannel <- file
		}
	}
}

// Uploads chunk when there is room in queue.
// This is a blocking method.
func uploadChunkWhenPossible(sup *srcUserPluginService, chunk UploadChunk, uploadTokensChan chan string) error {
	for {
		// If increment done, this go routine can proceed to upload the chunk. Otherwise, sleep and try again.
		isIncr := incrCurProcessedChunksWhenPossible()
		if !isIncr {
			time.Sleep(waitTimeBetweenChunkStatusSeconds * time.Second)
			continue
		}
		isChecksumDeployed, err := uploadChunkAndAddTokenIfNeeded(sup, chunk, uploadTokensChan)
		if err != nil || isChecksumDeployed {
			// Chunk not uploaded or does not require polling.
			reduceCurProcessedChunks()
		}
		return err
	}
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

// Gets a list of all errors files from the CLI's cache.
// Errors-files contain files that were failed to upload or actions that were skipped because of known limitations.
func getErrorsFiles(repoKey string, isRetry bool) (filesPaths []string, err error) {
	var dirPath string
	if isRetry {
		dirPath, err = coreutils.GetJfrogTransferRetryableDir()
	} else {
		dirPath, err = coreutils.GetJfrogTransferSkippedDir()
	}
	if err != nil {
		return []string{}, err
	}
	exist, err := biUtils.IsDirExists(dirPath, false)
	if !exist || err != nil {
		return []string{}, err
	}

	filesNames, err := biUtils.ListFiles(dirPath, false)
	if err != nil {
		return nil, err
	}

	for _, file := range filesNames {
		if strings.HasPrefix(filepath.Base(file), repoKey) {
			filesPaths = append(filesPaths, file)
		}
	}
	return
}
