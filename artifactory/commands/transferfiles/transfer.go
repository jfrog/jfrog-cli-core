package transferfiles

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"strconv"

	buildInfoUtils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	serviceUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	uploadChunkSize = 16
	// Size of the channel where the transfer's go routines write the transfer errors
	fileWritersChannelSize       = 500000
	retries                      = 1000
	retriesWaitMilliSecs         = 1000
	dataTransferPluginMinVersion = "1.4.0"
)

type TransferFilesCommand struct {
	Stoppable
	sourceServerDetails       *config.ServerDetails
	targetServerDetails       *config.ServerDetails
	sourceStorageInfoManager  *utils.StorageInfoManager
	targetStorageInfoManager  *utils.StorageInfoManager
	checkExistenceInFilestore bool
	progressbar               *TransferProgressMng
	includeReposPatterns      []string
	excludeReposPatterns      []string
	timeStarted               time.Time
	ignoreState               bool
}

func NewTransferFilesCommand(sourceServer, targetServer *config.ServerDetails) *TransferFilesCommand {
	return &TransferFilesCommand{sourceServerDetails: sourceServer, targetServerDetails: targetServer, timeStarted: time.Now()}
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

func (tdc *TransferFilesCommand) SetIgnoreState(ignoreState bool) {
	tdc.ignoreState = ignoreState
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

	if err := tdc.createTransferDir(); err != nil {
		return err
	}

	if err = tdc.initStorageInfoManagers(); err != nil {
		return err
	}

	sourceLocalRepos, err := tdc.getSourceLocalRepositories()
	if err != nil {
		return err
	}
	targetAllLocalRepos, err := tdc.getAllTargetLocalRepositories()
	if err != nil {
		return err
	}

	// Handle interruptions
	finishStopping, newPhase := tdc.handleStop(srcUpService)
	defer finishStopping()

	// Set progress bar with the length of the taget local and build info repositories
	tdc.progressbar, err = NewTransferProgressMng(int64(len(targetAllLocalRepos)))
	if err != nil {
		return err
	}

	// Transfer local repositories
	if err := tdc.transferRepos(sourceLocalRepos, targetAllLocalRepos, false, newPhase, srcUpService); err != nil {
		return tdc.cleanup(err, sourceLocalRepos)
	}

	// Transfer build-info repositories
	sourceLocalRepos, err = tdc.transferBuildInfoRepos(sourceLocalRepos, targetAllLocalRepos, newPhase, srcUpService)

	// Close progressBar and create CSV errors summary file
	return tdc.cleanup(err, sourceLocalRepos)
}

func (tdc *TransferFilesCommand) initStorageInfoManagers() error {
	// Init source storage info manager
	storageInfoManager, err := utils.NewStorageInfoManager(tdc.sourceServerDetails)
	if err != nil {
		return err
	}
	tdc.sourceStorageInfoManager = storageInfoManager
	if err := storageInfoManager.CalculateStorageInfo(); err != nil {
		return err
	}

	// Init target storage info manager
	storageInfoManager, err = utils.NewStorageInfoManager(tdc.targetServerDetails)
	if err != nil {
		return err
	}
	tdc.targetStorageInfoManager = storageInfoManager
	return storageInfoManager.CalculateStorageInfo()
}

func (tdc *TransferFilesCommand) transferRepos(sourceRepos []string, targetRepos []string,
	buildInfoRepo bool, newPhase *transferPhase, srcUpService *srcUserPluginService) error {
	for _, repoKey := range sourceRepos {
		if tdc.ShouldStop() {
			return nil
		}
		err := tdc.transferSingleRepo(repoKey, targetRepos, buildInfoRepo, newPhase, srcUpService)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tdc *TransferFilesCommand) transferSingleRepo(sourceRepoKey string, targetRepos []string,
	buildInfoRepo bool, newPhase *transferPhase, srcUpService *srcUserPluginService) (err error) {
	if !buildInfoUtils.IsStringInSlice(sourceRepoKey, targetRepos) {
		log.Error("repository '" + sourceRepoKey + "' does not exist in target. Skipping...")
		return
	}

	repoSummary, err := tdc.sourceStorageInfoManager.GetRepoSummary(sourceRepoKey)
	if err != nil {
		log.Error(err.Error() + ". Skipping...")
		return nil
	}

	if tdc.progressbar != nil {
		tdc.progressbar.NewRepository(sourceRepoKey)
	}

	if tdc.ignoreState {
		err = resetRepoState(sourceRepoKey)
		if err != nil {
			return
		}
	}

	restoreFunc, err := tdc.handleMaxUniqueSnapshots(repoSummary)
	if err != nil {
		return
	}
	defer func() {
		e := restoreFunc()
		if err == nil {
			err = e
		}
	}()

	if err = tdc.initCurThreads(buildInfoRepo); err != nil {
		return
	}
	for currentPhaseId := 0; currentPhaseId < numberOfPhases; currentPhaseId++ {
		if tdc.ShouldStop() {
			return
		}

		*newPhase = getPhaseByNum(currentPhaseId, sourceRepoKey, buildInfoRepo)
		if err = tdc.startPhase(newPhase, sourceRepoKey, *repoSummary, srcUpService); err != nil {
			return
		}
	}
	// Performing a reset on the source if chunks were not removed
	runningNodes, err := getRunningNodes(tdc.sourceServerDetails)
	if err != nil {
		log.Error(err)
	} else {
		stopTransferOnArtifactoryNodes(srcUpService, runningNodes)
	}
	return
}

func (tdc *TransferFilesCommand) transferBuildInfoRepos(sourceRepos []string, targetRepos []string, newPhase *transferPhase, srcUpService *srcUserPluginService) ([]string, error) {
	sourceStorageInfo, err := tdc.sourceStorageInfoManager.GetStorageInfo()
	if err != nil {
		return sourceRepos, err
	}

	sourceBuildInfoRepoKeys, err := utils.GetFilteredBuildInfoRepositories(sourceStorageInfo, tdc.includeReposPatterns, tdc.excludeReposPatterns)
	if err != nil {
		return sourceRepos, err
	}
	allSourceRepos := append(sourceRepos, sourceBuildInfoRepoKeys...)

	return allSourceRepos, tdc.transferRepos(sourceBuildInfoRepoKeys, targetRepos, true, newPhase, srcUpService)
}

func (tdc *TransferFilesCommand) createTransferDir() error {
	transferDir, err := coreutils.GetJfrogTransferDir()
	if err != nil {
		return err
	}
	return errorutils.CheckError(os.MkdirAll(transferDir, 0777))
}

func (tdc *TransferFilesCommand) startPhase(newPhase *transferPhase, repo string, repoSummary serviceUtils.RepositorySummary, srcUpService *srcUserPluginService) error {
	tdc.initNewPhase(*newPhase, srcUpService, repoSummary)
	skip, err := (*newPhase).shouldSkipPhase()
	if err != nil || skip {
		return err
	}
	err = (*newPhase).phaseStarted()
	if err != nil {
		return err
	}
	err = (*newPhase).initProgressBar()
	if err != nil {
		return err
	}
	printPhaseChange("Running '" + (*newPhase).getPhaseName() + "' for repo '" + repo + "'...")
	err = (*newPhase).run()
	if err != nil {
		// We do not return the error returned from the phase's run function,
		// because the phase is expected to recover from some errors, such as HTTP connection errors.
		log.Error(err.Error())
	}
	printPhaseChange("Done running '" + (*newPhase).getPhaseName() + "' for repo '" + repo + "'.")
	return (*newPhase).phaseDone()
}

// Handle interrupted signal.
// shouldStop - Pointer to boolean variable, if the process gets interrupted shouldStop will be set to true
// newPhase - The current running phase
// srcUpService - Source plugin service
func (tdc *TransferFilesCommand) handleStop(srcUpService *srcUserPluginService) (func(), *transferPhase) {
	var newPhase transferPhase
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
		tdc.Stop()
		if newPhase != nil {
			newPhase.Stop()
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
		if tdc.ShouldStop() {
			// If should stop, wait for stop to happen
			<-finishStop
		}
	}, &newPhase
}

func (tdc *TransferFilesCommand) initNewPhase(newPhase transferPhase, srcUpService *srcUserPluginService, repoSummary serviceUtils.RepositorySummary) {
	newPhase.shouldCheckExistenceInFilestore(tdc.checkExistenceInFilestore)
	newPhase.setSourceDetails(tdc.sourceServerDetails)
	newPhase.setTargetDetails(tdc.targetServerDetails)
	newPhase.setSrcUserPluginService(srcUpService)
	newPhase.setRepoSummary(repoSummary)
	newPhase.setProgressBar(tdc.progressbar)
}

func (tdc *TransferFilesCommand) getSourceLocalRepositories() ([]string, error) {
	serviceManager, err := utils.CreateServiceManager(tdc.sourceServerDetails, retries, retriesWaitMilliSecs, false)
	if err != nil {
		return []string{}, err
	}
	return utils.GetFilteredRepositoriesByNameAndType(serviceManager, tdc.includeReposPatterns, tdc.excludeReposPatterns, utils.Local)
}

func (tdc *TransferFilesCommand) getAllTargetLocalRepositories() ([]string, error) {
	serviceManager, err := utils.CreateServiceManager(tdc.targetServerDetails, retries, retriesWaitMilliSecs, false)
	if err != nil {
		return []string{}, err
	}
	targetRepos, err := utils.GetFilteredRepositoriesByNameAndType(serviceManager, tdc.includeReposPatterns, tdc.excludeReposPatterns, utils.Local)
	if err != nil {
		return []string{}, err
	}

	targetStorageInfo, err := tdc.targetStorageInfoManager.GetStorageInfo()
	if err != nil {
		return []string{}, err
	}

	targetBuildInfoRepoKeys, err := utils.GetFilteredBuildInfoRepositories(targetStorageInfo, tdc.includeReposPatterns, tdc.excludeReposPatterns)
	if err != nil {
		return []string{}, err
	}

	return append(targetRepos, targetBuildInfoRepoKeys...), err
}

func (tdc *TransferFilesCommand) initCurThreads(buildInfoRepo bool) error {
	// Use default threads if settings file doesn't exist or an error occurred.
	curThreads = utils.DefaultThreads
	settings, err := utils.LoadTransferSettings()
	if err != nil {
		return err
	}
	if settings != nil {
		curThreads = settings.CalcNumberOfThreads(buildInfoRepo)
		if buildInfoRepo && curThreads < settings.ThreadsNumber {
			log.Info("Build info transferring - using reduced number of threads")
		}
	}

	log.Info("Running with maximum", strconv.Itoa(curThreads), "working threads...")
	return nil
}

func printPhaseChange(message string) {
	log.Info("========== " + message + " ==========")
}

// If an error occurred cleanup will:
// 1. Close progressBar
// 2. Create CSV errors summary file
func (tdc *TransferFilesCommand) cleanup(originalErr error, sourceRepos []string) (err error) {
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
	csvErrorsFile, e := createErrorsCsvSummary(sourceRepos, tdc.timeStarted)
	if err == nil {
		err = e
	}
	if csvErrorsFile != "" {
		log.Info(fmt.Sprintf("Errors occurred during the transfer. Check the errors summary CSV file in: %s", csvErrorsFile))
	}
	return
}

// handleMaxUniqueSnapshots handles special cases regarding the Max Unique Snapshots/Tags setting of repositories of
// these package types: Maven, Gradle, NuGet, Ivy, SBT and Docker.
// TL;DR: we might have repositories in the source with more snapshots than the maximum (in Max Unique Snapshots/Tags),
// so he turn it off at the beginning of the transfer and turn it back on at the end.
//
// And in more detail:
// The cleanup of old snapshots in Artifactory is triggered by uploading a new snapshot only, so we might have
// repositories with more snapshots than the maximum (by setting the Max Unique Snapshots/Tags on a repository with more
// snapshots than the maximum without uploading a new snapshot afterwards).
// In such repositories, the transfer process uploads the snapshots to the target instance and triggers the cleanup, so
// eventually the repository in the target might have fewer snapshots than in the source.
// To handle this, we turn off the Max Unique Snapshots/Tags setting (by setting it 0) at the beginning of the transfer
// of the repository, and copy it from the source at the end.
func (tdc *TransferFilesCommand) handleMaxUniqueSnapshots(repoSummary *serviceUtils.RepositorySummary) (restoreFunc func() error, err error) {
	// Get the source repository's max unique snapshots setting
	srcMaxUniqueSnapshots, err := getMaxUniqueSnapshots(tdc.sourceServerDetails, repoSummary)
	if err != nil {
		return
	}

	// If it's a Maven, Gradle, NuGet, Ivy, SBT or Docker repository, update its max unique snapshots setting to 0.
	// srcMaxUniqueSnapshots == -1 means it's a repository of another package type.
	if srcMaxUniqueSnapshots != -1 {
		err = updateMaxUniqueSnapshots(tdc.targetServerDetails, repoSummary, 0)
		if err != nil {
			return
		}
	}

	restoreFunc = func() (err error) {
		// Update the target repository's max unique snapshots setting to be the same as in the source, only if it's not 0.
		if srcMaxUniqueSnapshots > 0 {
			err = updateMaxUniqueSnapshots(tdc.targetServerDetails, repoSummary, srcMaxUniqueSnapshots)
		}
		return
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
