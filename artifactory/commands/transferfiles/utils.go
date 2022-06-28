package transferfiles

import (
	"encoding/json"
	biUtils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
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

const waitTimeBetweenChunkStatusSeconds = 3
const waitTimeBetweenThreadsUpdateSeconds = 20
const phase1Id = 0
const phase2Id = 1

var curThreads int
var threadsMutex sync.Mutex

func init() {
	// Use default threads if settings file doesn't exist or an error occurred.
	curThreads = defaultThreads
	settings, err := utils.LoadTransferSettings()
	if err != nil {
		log.Error(err)
		return
	}
	if settings != nil {
		curThreads = settings.ThreadsNumber
	}
}

func createSrcRtUserPluginServiceManager(sourceRtDetails *coreConfig.ServerDetails) (*srcUserPluginService, error) {
	serviceManager, err := utils.CreateServiceManager(sourceRtDetails, 0, 0, false)
	if err != nil {
		return nil, err
	}
	return NewSrcUserPluginService(serviceManager.GetConfig().GetServiceDetails(), serviceManager.Client()), nil
}

func (tdc *TransferFilesCommand) getStorageInfo() (*artifactoryUtils.StorageInfo, error) {
	serviceManager, err := utils.CreateServiceManager(tdc.sourceServerDetails, -1, 0, false)
	if err != nil {
		return nil, err
	}
	return serviceManager.StorageInfo()
}

func (tdc *TransferFilesCommand) createTargetUploadServiceManager() (*services.UploadService, error) {
	serviceManager, err := utils.CreateServiceManager(tdc.targetServerDetails, 0, 0, false)
	if err != nil {
		return nil, err
	}
	uploadService := services.NewUploadService(serviceManager.Client())
	uploadService.ArtDetails = serviceManager.GetConfig().GetServiceDetails()
	uploadService.Threads = serviceManager.GetConfig().GetThreads()
	return uploadService, nil
}

func (tdc *TransferFilesCommand) createSourceDownloadServiceManager() (*services.DownloadService, error) {
	serviceManager, err := utils.CreateServiceManager(tdc.sourceServerDetails, 0, 0, false)
	if err != nil {
		return nil, err
	}
	downloadService := services.NewDownloadService(serviceManager.GetConfig().GetServiceDetails(), serviceManager.Client())
	downloadService.Threads = serviceManager.GetConfig().GetThreads()
	return downloadService, nil
}

func (tdc *TransferFilesCommand) createSourcePropsServiceManager() (*services.PropsService, error) {
	return createPropsServiceManager(tdc.sourceServerDetails)
}

func (tdc *TransferFilesCommand) createTargetPropsServiceManager() (*services.PropsService, error) {
	return createPropsServiceManager(tdc.targetServerDetails)
}

func createPropsServiceManager(serverDetails *coreConfig.ServerDetails) (*services.PropsService, error) {
	serviceManager, err := utils.CreateServiceManager(serverDetails, 0, 0, false)
	if err != nil {
		return nil, err
	}
	propsService := services.NewPropsService(serviceManager.Client())
	propsService.ArtDetails = serviceManager.GetConfig().GetServiceDetails()
	return propsService, nil
}

func (tdc *TransferFilesCommand) getAllSrcLocalRepositories() (*[]services.RepositoryDetails, error) {
	return tdc.getAllLocalRepositories(tdc.sourceServerDetails)
}

func (tdc *TransferFilesCommand) getAllTargetLocalRepositories() (*[]services.RepositoryDetails, error) {
	return tdc.getAllLocalRepositories(tdc.targetServerDetails)
}

func (tdc *TransferFilesCommand) getAllLocalRepositories(serverDetails *coreConfig.ServerDetails) (*[]services.RepositoryDetails, error) {
	serviceManager, err := utils.CreateServiceManager(serverDetails, -1, 0, false)
	if err != nil {
		return nil, err
	}

	params := services.RepositoriesFilterParams{RepoType: "local"}
	return serviceManager.GetAllRepositoriesFiltered(params)
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

// This variable holds the number of upload chunk that was sent to the user plugin to process.
// Together with this mutex, they control the load on the user plugin and couple it to the local number of threads.
var curProcessedUploadChunks = 0
var processedUploadChunksMutex sync.Mutex

func pollUploads(srcUpService *srcUserPluginService, uploadTokensChan chan string, doneChan chan bool, progressbar *progressbar.TransferProgressMng, phaseId int) error {
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
				progressbar.IncrementPhase(phaseId)
				curTokensBatch.UuidTokens = removeTokenFromBatch(curTokensBatch.UuidTokens, chunk.UuidToken)
				handleFilesOfCompletedChunk(chunk.Files)
			}
		}
	}
}

func incrCurProcessedChunksWhenPossible() bool {
	processedUploadChunksMutex.Lock()
	defer processedUploadChunksMutex.Unlock()
	if curProcessedUploadChunks < getThreads() {
		curProcessedUploadChunks++
		return true
	}
	return false
}

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

func handleFilesOfCompletedChunk(chunkFiles []FileUploadStatusResponse) {
	for _, file := range chunkFiles {
		switch file.Status {
		case Success:
			// TODO update summary.
		case Fail:
			// TODO update summary.
			addFailuresToConsumableFile(file)
		case SkippedLargeProps:
			// TODO update summary.
			addToSkippedFile(file)
		}
	}
}

func addFailuresToConsumableFile(file FileUploadStatusResponse) {
	// TODO implement
}

func addToSkippedFile(file FileUploadStatusResponse) {
	// TODO implement
}

// Uploads chunk when there is room in queue.
// This is a blocking method.
func uploadChunkWhenPossible(sup *srcUserPluginService, chunk UploadChunk, uploadTokensChan chan string, progressbar *progressbar.TransferProgressMng, phaseId int) error {
	for {
		// If increment done, this go routine can proceed to upload the chunk. Otherwise, sleep and try again.
		isIncr := incrCurProcessedChunksWhenPossible()
		if !isIncr {
			time.Sleep(waitTimeBetweenChunkStatusSeconds * time.Second)
			continue
		}
		isChecksumDeployed, err := uploadChunkAndAddTokenIfNeeded(sup, chunk, uploadTokensChan, progressbar, phaseId)
		if err != nil || isChecksumDeployed {
			// Chunk not uploaded or does not require polling.
			reduceCurProcessedChunks()
		}
		return err
	}
}

func uploadChunkAndAddTokenIfNeeded(sup *srcUserPluginService, chunk UploadChunk, uploadTokensChan chan string, progressbar *progressbar.TransferProgressMng, phaseId int) (bool, error) {
	uuidToken, err := sup.uploadChunk(chunk)
	if err != nil {
		return false, err
	}
	// Empty token is returned if all files were checksum deployed.
	if uuidToken == "" {
		progressbar.IncrementPhaseBy(phaseId, len(chunk.UploadCandidates))
		return true, nil
	}

	// Add token to polling.
	uploadTokensChan <- uuidToken
	return false, nil
}

func verifyRepoExistsInTarget(targetRepos *[]services.RepositoryDetails, srcRepoKey string) bool {
	for _, targetRepo := range *targetRepos {
		if targetRepo.Key == srcRepoKey {
			return true
		}
	}
	return false
}

func getThreads() int {
	threadsMutex.Lock()
	defer threadsMutex.Unlock()
	return curThreads
}

// Periodically reads settings file and updates the number of threads.
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
	threadsMutex.Lock()
	defer threadsMutex.Unlock()

	settings, err := utils.LoadTransferSettings()
	if err != nil {
		return err
	}
	if settings != nil && curThreads != settings.ThreadsNumber {
		curThreads = settings.ThreadsNumber
		producerConsumer.SetMaxParallel(settings.ThreadsNumber)
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
