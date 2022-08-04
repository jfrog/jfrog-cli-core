package transferfiles

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"strconv"

	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	serviceUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	tasksMaxCapacity = 10000
	uploadChunkSize  = 16
	// Size of the channel where the transfer's go routines write the transfer errors
	fileWritersChannelSize       = 500000
	retries                      = 3
	retriesWait                  = 0
	dataTransferPluginMinVersion = "1.3.0"
)

type TransferFilesCommand struct {
	sourceServerDetails       *config.ServerDetails
	targetServerDetails       *config.ServerDetails
	checkExistenceInFilestore bool
	progressbar               *progressbar.TransferProgressMng
	includeReposPatterns      []string
	excludeReposPatterns      []string
}

func NewTransferFilesCommand(sourceServer, targetServer *config.ServerDetails) *TransferFilesCommand {
	return &TransferFilesCommand{sourceServerDetails: sourceServer, targetServerDetails: targetServer}
}

func (tdc *TransferFilesCommand) CommandName() string {
	return "rt_transfer_files"
}

func (tdc *TransferFilesCommand) SetFilestore(filestore bool) {
	tdc.checkExistenceInFilestore = filestore
}

func (tdc *TransferFilesCommand) SetIncludeReposPatterns(includeReposPatterns []string) {
	tdc.includeReposPatterns = includeReposPatterns
}

func (tdc *TransferFilesCommand) SetExcludeReposPatterns(excludeReposPatterns []string) {
	tdc.excludeReposPatterns = excludeReposPatterns
}

func (tdc *TransferFilesCommand) Run() (err error) {
	srcUpService, err := createSrcRtUserPluginServiceManager(tdc.sourceServerDetails)
	if err != nil {
		return err
	}

	// Verify connection to the source Artifactory instance, and that the user plugin is installed, responsive, and stands in the minimal version requirement.
	if err = getAndValidateDataTransferPlugin(srcUpService); err != nil {
		return err
	}

	transferDir, err := coreutils.GetJfrogTransferDir()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(transferDir, 0777); err != nil {
		return errorutils.CheckError(err)
	}

	if err = tdc.initCurThreads(); err != nil {
		return err
	}

	sourceStorageInfo, targetStorageInfo, err := tdc.getSourceAndTargetStorageInfo()
	if err != nil {
		return err
	}

	sourceRepos, targetRepos, err := tdc.getSourceAndTargetlocalRepositories(sourceStorageInfo, targetStorageInfo)
	if err != nil {
		return err
	}

	// Set progress bar
	tdc.progressbar, err = progressbar.NewTransferProgressMng(int64(len(sourceRepos)))
	if err != nil {
		return err
	}

	// Handle interruptions
	shouldStop := false
	var newPhase transferPhase
	finishStopping := tdc.handleStop(&shouldStop, &newPhase, srcUpService)
	defer finishStopping()

	for _, repo := range sourceRepos {
		if shouldStop {
			break
		}

		exists := verifyRepoExistsInTarget(targetRepos, repo)
		if !exists {
			log.Error("repository '" + repo + "' does not exist in target. Skipping...")
			continue
		}

		repoSummary, err := getRepoSummaryFromList(sourceStorageInfo.RepositoriesSummaryList, repo)
		if err != nil {
			log.Error(err.Error() + ". Skipping...")
			continue
		}

		if tdc.progressbar != nil {
			tdc.progressbar.NewRepository(repo)
		}
		for currentPhaseId := 0; currentPhaseId < numberOfPhases; currentPhaseId++ {
			if shouldStop {
				break
			}
			newPhase = getPhaseByNum(currentPhaseId, repo)
			err = tdc.startPhase(newPhase, repo, repoSummary, srcUpService)
			if err != nil {
				return tdc.cleanup(err)
			}
		}
	}
	// Close progressBar and create CSV errors summary file
	return tdc.cleanup(nil)
}

func (tdc *TransferFilesCommand) startPhase(newPhase transferPhase, repo string, repoSummary serviceUtils.RepositorySummary, srcUpService *srcUserPluginService) error {
	tdc.initNewPhase(newPhase, srcUpService, repoSummary)
	skip, err := newPhase.shouldSkipPhase()
	if err != nil || skip {
		return err
	}
	err = newPhase.phaseStarted()
	if err != nil {
		return err
	}
	err = newPhase.initProgressBar()
	if err != nil {
		return err
	}
	printPhaseChange("Running '" + newPhase.getPhaseName() + "' for repo '" + repo + "'...")
	err = newPhase.run()
	if err != nil {
		return err
	}
	printPhaseChange("Done running '" + newPhase.getPhaseName() + "' for repo '" + repo + "'.")
	return newPhase.phaseDone()
}

// Handle interrupted signal.
// shouldStop - Pointer to boolean variable, if the process gets interrupted shouldStop will be set to true
// newPhase - The current running phase
// srcUpService - Source plugin service
func (tdc *TransferFilesCommand) handleStop(shouldStop *bool, newPhase *transferPhase, srcUpService *srcUserPluginService) func() {
	finishStop := make(chan bool)
	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer close(finishStop)
		// Wait for the stop signal or close(stopSignal) to happen
		if <-stopSignal == nil {
			// The stopSignal channel is closed
			return
		}
		*shouldStop = true
		if newPhase != nil {
			(*newPhase).stopGracefully()
		}
		log.Info("Gracefully stopping files transfer...")
		runningNodes, err := getRunningNodes(tdc.sourceServerDetails)
		if err != nil {
			log.Error(err)
		} else {
			stopTransferOnArtifactoryNodes(srcUpService, runningNodes)
		}
	}()

	// Return a cleanup function that closes the stopSignal channel and wait for close if needed
	return func() {
		// Close the stop signal channel
		close(stopSignal)
		if *shouldStop {
			// If should stop, wait for stop to happen
			<-finishStop
		}
	}
}

func (tdc *TransferFilesCommand) initNewPhase(newPhase transferPhase, srcUpService *srcUserPluginService, repoSummary serviceUtils.RepositorySummary) {
	newPhase.shouldCheckExistenceInFilestore(tdc.checkExistenceInFilestore)
	newPhase.setSourceDetails(tdc.sourceServerDetails)
	newPhase.setTargetDetails(tdc.targetServerDetails)
	newPhase.setSrcUserPluginService(srcUpService)
	newPhase.setRepoSummary(repoSummary)
	newPhase.setProgressBar(tdc.progressbar)
}

func (tdc *TransferFilesCommand) getSourceAndTargetlocalRepositories(sourceStorageInfo *serviceUtils.StorageInfo, targetStorageInfo *serviceUtils.StorageInfo) ([]string, []string, error) {
	sourceRepos, err := tdc.getLocalRepositories(sourceStorageInfo, tdc.sourceServerDetails)
	if err != nil {
		return []string{}, []string{}, err
	}
	targetRepos, err := tdc.getLocalRepositories(targetStorageInfo, tdc.targetServerDetails)
	return sourceRepos, targetRepos, err
}

func (tdc *TransferFilesCommand) getLocalRepositories(storageInfo *serviceUtils.StorageInfo, serverDetails *config.ServerDetails) ([]string, error) {
	var repoKeys []string
	serviceManager, err := utils.CreateServiceManager(serverDetails, retries, retriesWait, false)
	if err != nil {
		return repoKeys, err
	}
	repoKeys, err = utils.GetFilteredRepositories(serviceManager, tdc.includeReposPatterns, tdc.excludeReposPatterns, utils.Local)
	if err != nil {
		return repoKeys, err
	}

	buildInfoRepoKeys, err := utils.GetFilteredBuildInfoRepostories(storageInfo, tdc.includeReposPatterns, tdc.excludeReposPatterns)
	if err != nil {
		return repoKeys, err
	}
	return append(buildInfoRepoKeys, repoKeys...), nil
}

func (tdc *TransferFilesCommand) getSourceAndTargetStorageInfo() (*serviceUtils.StorageInfo, *serviceUtils.StorageInfo, error) {
	// Get source storage info
	sourceStorageInfo, err := tdc.getStorageInfo(tdc.sourceServerDetails)
	if err != nil {
		return nil, nil, err
	}

	// Get target storage info
	targetStorageInfo, err := tdc.getStorageInfo(tdc.targetServerDetails)
	if err != nil {
		return nil, nil, err
	}

	return sourceStorageInfo, targetStorageInfo, err
}

func (tdc *TransferFilesCommand) getStorageInfo(serverDetails *config.ServerDetails) (*serviceUtils.StorageInfo, error) {
	serviceManager, err := utils.CreateServiceManager(serverDetails, retries, retriesWait, false)
	if err != nil {
		return nil, err
	}
	return serviceManager.StorageInfo(true)
}

func (tdc *TransferFilesCommand) initCurThreads() error {
	// Use default threads if settings file doesn't exist or an error occurred.
	curThreads = utils.DefaultThreads
	settings, err := utils.LoadTransferSettings()
	if err != nil {
		return err
	}
	if settings != nil {
		curThreads = settings.ThreadsNumber
	}
	log.Info("Running with " + strconv.Itoa(curThreads) + " threads...")
	return nil
}

func printPhaseChange(message string) {
	log.Info("========== " + message + " ==========")
}

type producerConsumerDetails struct {
	producerConsumer parallel.Runner
	errorsQueue      *clientUtils.ErrorsQueue
}

// If an error occurred cleanup will:
// 1. Close progressBar
// 2. Create CSV errors summary file
func (tdc *TransferFilesCommand) cleanup(originalErr error) (err error) {
	err = originalErr
	// Quit progress bar (before printing logs)
	if tdc.progressbar != nil {
		e := tdc.progressbar.Quit()
		if err == nil {
			err = e
		}
	}
	// Transferring finished successfully
	if originalErr == nil {
		log.Info("Files transfer is complete!")
	}
	// Create csv errors summary file
	csvErrorsFile, e := createErrorsCsvSummary()
	if err == nil {
		err = e
	}
	if csvErrorsFile != "" {
		log.Info(fmt.Sprintf("Errors occurred during the transfer. Check the errors summary CSV file in: %s", csvErrorsFile))
	}
	return
}

func validateDataTransferPluginMinimumVersion(currentVersion string) error {
	curVer := version.NewVersion(currentVersion)
	if !curVer.AtLeast(dataTransferPluginMinVersion) {
		return errorutils.CheckErrorf(getMinimalVersionErrorMsg(currentVersion))
	}
	return nil
}

func getMinimalVersionErrorMsg(currentVersion string) string {
	return "You are currently using data-transfer plugin version '" +
		currentVersion + "' on your source instance, while the minimum required version is '" + dataTransferPluginMinVersion + "' or higher."
}

func getAndValidateDataTransferPlugin(srcUpService *srcUserPluginService) error {
	dataPluginVer, err := srcUpService.version()
	if err != nil {
		return err
	}
	err = validateDataTransferPluginMinimumVersion(dataPluginVer)
	if err != nil {
		return err
	}
	log.Info("data-transfer plugin version: " + dataPluginVer)
	return nil
}
