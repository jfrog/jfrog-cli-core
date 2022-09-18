package transferfiles

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jfrog/gofrog/version"
	"golang.org/x/exp/slices"

	"strconv"

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
	dataTransferPluginMinVersion = "1.5.0"
)

type TransferFilesCommand struct {
	context                   context.Context
	cancelFunc                context.CancelFunc
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
	timeEstMng                *timeEstimationManager
	proxyKey                  string
}

func NewTransferFilesCommand(sourceServer, targetServer *config.ServerDetails) *TransferFilesCommand {
	context, cancelFunc := context.WithCancel(context.Background())
	return &TransferFilesCommand{
		context:             context,
		cancelFunc:          cancelFunc,
		sourceServerDetails: sourceServer,
		targetServerDetails: targetServer,
		timeStarted:         time.Now(),
	}
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

func (tdc *TransferFilesCommand) SetProxyKey(proxyKey string) {
	tdc.proxyKey = proxyKey
}

func (tdc *TransferFilesCommand) Run() (err error) {
	srcUpService, err := createSrcRtUserPluginServiceManager(tdc.context, tdc.sourceServerDetails)
	if err != nil {
		return err
	}

	if err = getAndValidateDataTransferPlugin(srcUpService); err != nil {
		return err
	}

	if err = tdc.verifySourceTargetConnectivity(srcUpService); err != nil {
		return err
	}

	if err := tdc.createTransferDir(); err != nil {
		return err
	}

	if err = tdc.initStorageInfoManagers(); err != nil {
		return err
	}

	sourceLocalRepos, sourceBuildInfoRepos, err := tdc.getAllLocalRepos(tdc.sourceServerDetails, tdc.sourceStorageInfoManager)
	if err != nil {
		return err
	}
	allSourceLocalRepos := append(sourceLocalRepos, sourceBuildInfoRepos...)
	targetLocalRepos, targetBuildInfoRepos, err := tdc.getAllLocalRepos(tdc.targetServerDetails, tdc.targetStorageInfoManager)
	if err != nil {
		return err
	}

	// Handle interruptions
	finishStopping, newPhase := tdc.handleStop(srcUpService)
	defer finishStopping()

	if isTimeEstimationEnabled() {
		if err = tdc.initTimeEstimationManager(sourceLocalRepos); err != nil {
			return err
		}
	}

	// Set progress bar with the length of the source local and build info repositories
	tdc.progressbar, err = NewTransferProgressMng(int64(len(allSourceLocalRepos)), tdc.timeEstMng)
	if err != nil {
		return err
	}

	// Transfer local repositories
	if err := tdc.transferRepos(sourceLocalRepos, targetLocalRepos, false, newPhase, srcUpService); err != nil {
		return tdc.cleanup(err, sourceLocalRepos)
	}

	// Transfer build-info repositories
	if err := tdc.transferRepos(sourceBuildInfoRepos, targetBuildInfoRepos, true, newPhase, srcUpService); err != nil {
		return tdc.cleanup(err, allSourceLocalRepos)
	}

	// Close progressBar and create CSV errors summary file
	return tdc.cleanup(err, allSourceLocalRepos)
}

func (tdc *TransferFilesCommand) initStorageInfoManagers() error {
	// Init source storage info manager
	storageInfoManager, err := utils.NewStorageInfoManager(tdc.context, tdc.sourceServerDetails)
	if err != nil {
		return err
	}
	tdc.sourceStorageInfoManager = storageInfoManager
	if err := storageInfoManager.CalculateStorageInfo(); err != nil {
		return err
	}

	// Init target storage info manager
	storageInfoManager, err = utils.NewStorageInfoManager(tdc.context, tdc.targetServerDetails)
	if err != nil {
		return err
	}
	tdc.targetStorageInfoManager = storageInfoManager
	return storageInfoManager.CalculateStorageInfo()
}

func (tdc *TransferFilesCommand) initTimeEstimationManager(sourceLocalRepos []string) error {
	totalSize, err := tdc.sourceStorageInfoManager.GetReposTotalSize(sourceLocalRepos...)
	if err != nil {
		return err
	}
	transferredSize, err := getReposTransferredSizeBytes(sourceLocalRepos...)
	if err != nil {
		return err
	}
	tdc.timeEstMng = newTimeEstimationManager(totalSize, transferredSize)
	return nil
}

func (tdc *TransferFilesCommand) transferRepos(sourceRepos []string, targetRepos []string,
	buildInfoRepo bool, newPhase *transferPhase, srcUpService *srcUserPluginService) error {
	for _, repoKey := range sourceRepos {
		if tdc.shouldStop() {
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
	if !slices.Contains(targetRepos, sourceRepoKey) {
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
		if tdc.shouldStop() {
			return
		}
		// Ensure the data structure which stores the upload tasks on Artifactory's side is wiped clean,
		// in case some of the requests to delete handles tasks sent by JFrog CLI did not reach Artifactory.
		err = stopTransferInArtifactory(tdc.context, tdc.sourceServerDetails, srcUpService)
		if err != nil {
			log.Error(err)
		}
		*newPhase = getPhaseByNum(tdc.context, currentPhaseId, sourceRepoKey, tdc.proxyKey, buildInfoRepo)
		if err = tdc.startPhase(newPhase, sourceRepoKey, *repoSummary, srcUpService); err != nil {
			return
		}
	}
	return
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
		tdc.cancelFunc()
		if newPhase != nil {
			newPhase.StopGracefully()
		}
		log.Info("Gracefully stopping files transfer...")
		err := stopTransferInArtifactory(tdc.context, tdc.sourceServerDetails, srcUpService)
		if err != nil {
			log.Error(err)
		}
	}()

	// Return a cleanup function that closes the stopSignal channel and wait for close if needed
	return func() {
		// Close the stop signal channel
		close(stopSignal)
		if tdc.shouldStop() {
			// If should stop, wait for stop to happen
			<-finishStop
		}
	}, &newPhase
}

func (tdc *TransferFilesCommand) initNewPhase(newPhase transferPhase, srcUpService *srcUserPluginService, repoSummary serviceUtils.RepositorySummary) {
	newPhase.setCheckExistenceInFilestore(tdc.checkExistenceInFilestore)
	newPhase.setSourceDetails(tdc.sourceServerDetails)
	newPhase.setTargetDetails(tdc.targetServerDetails)
	newPhase.setSrcUserPluginService(srcUpService)
	newPhase.setRepoSummary(repoSummary)
	newPhase.setProgressBar(tdc.progressbar)
	newPhase.setTimeEstMng(tdc.timeEstMng)
}

// Get all local and build-info repositories of the input server
// serverDetails      - Source or target server details
// storageInfoManager - Source or target storage info manager
func (tdc *TransferFilesCommand) getAllLocalRepos(serverDetails *config.ServerDetails, storageInfoManager *utils.StorageInfoManager) ([]string, []string, error) {
	serviceManager, err := createTransferServiceManager(tdc.context, serverDetails)
	if err != nil {
		return []string{}, []string{}, err
	}
	localRepos, err := utils.GetFilteredRepositoriesByNameAndType(serviceManager, tdc.includeReposPatterns, tdc.excludeReposPatterns, utils.Local)
	if err != nil {
		return []string{}, []string{}, err
	}
	federatedRepos, err := utils.GetFilteredRepositoriesByNameAndType(serviceManager, tdc.includeReposPatterns, tdc.excludeReposPatterns, utils.Federated)
	if err != nil {
		return []string{}, []string{}, err
	}

	storageInfo, err := storageInfoManager.GetStorageInfo()
	if err != nil {
		return []string{}, []string{}, err
	}

	buildInfoRepoKeys, err := utils.GetFilteredBuildInfoRepositories(storageInfo, tdc.includeReposPatterns, tdc.excludeReposPatterns)
	if err != nil {
		return []string{}, []string{}, err
	}

	return append(localRepos, federatedRepos...), buildInfoRepoKeys, err
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
	srcMaxUniqueSnapshots, err := getMaxUniqueSnapshots(tdc.context, tdc.sourceServerDetails, repoSummary)
	if err != nil {
		return
	}

	// If it's a Maven, Gradle, NuGet, Ivy, SBT or Docker repository, update its max unique snapshots setting to 0.
	// srcMaxUniqueSnapshots == -1 means it's a repository of another package type.
	if srcMaxUniqueSnapshots != -1 {
		err = updateMaxUniqueSnapshots(tdc.context, tdc.targetServerDetails, repoSummary, 0)
		if err != nil {
			return
		}
	}

	restoreFunc = func() (err error) {
		// Update the target repository's max unique snapshots setting to be the same as in the source, only if it's not 0.
		if srcMaxUniqueSnapshots > 0 {
			err = updateMaxUniqueSnapshots(tdc.context, tdc.targetServerDetails, repoSummary, srcMaxUniqueSnapshots)
		}
		return
	}
	return
}

func (tdc *TransferFilesCommand) shouldStop() bool {
	return tdc.context.Err() != nil
}

func (tdc *TransferFilesCommand) verifySourceTargetConnectivity(srcUpService *srcUserPluginService) error {
	log.Info("Verifying source to target Artifactory servers connectivity...")
	targetAuth := createTargetAuth(tdc.targetServerDetails, tdc.proxyKey)
	err := srcUpService.verifyConnectivityRequest(targetAuth)
	if err == nil {
		log.Info("Connectivity check passed!")
	}
	return err
}

func validateDataTransferPluginMinimumVersion(currentVersion string) error {
	if strings.Contains(currentVersion, "SNAPSHOT") {
		return nil
	}
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

// Verify connection to the source Artifactory instance, and that the user plugin is installed, responsive, and stands in the minimal version requirement.
func getAndValidateDataTransferPlugin(srcUpService *srcUserPluginService) error {
	verifyResponse, err := srcUpService.verifyCompatibilityRequest()
	if err != nil {
		return err
	}

	err = validateDataTransferPluginMinimumVersion(verifyResponse.Version)
	if err != nil {
		return err
	}
	log.Info("data-transfer plugin version: " + verifyResponse.Version)
	return nil
}
