package transferfiles

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	commandsUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	serviceUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/artifactory/usage"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/slices"
)

const (
	uploadChunkSize = 16
	// Size of the channel where the transfer's go routines write the transfer errors
	fileWritersChannelSize       = 500000
	retries                      = 600
	retriesWaitMilliSecs         = 5000
	dataTransferPluginMinVersion = "1.7.0"
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
	proxyKey                  string
	status                    bool
	stop                      bool
	stopSignal                chan os.Signal
	stateManager              *state.TransferStateManager
	preChecks                 bool
	locallyGeneratedFilter    *locallyGeneratedFilter
}

func NewTransferFilesCommand(sourceServer, targetServer *config.ServerDetails) (*TransferFilesCommand, error) {
	stateManager, err := state.NewTransferStateManager(false)
	if err != nil {
		return nil, err
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &TransferFilesCommand{
		context:             ctx,
		cancelFunc:          cancelFunc,
		sourceServerDetails: sourceServer,
		targetServerDetails: targetServer,
		timeStarted:         time.Now(),
		stateManager:        stateManager,
		stopSignal:          make(chan os.Signal, 1),
	}, nil
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

func (tdc *TransferFilesCommand) SetStatus(status bool) {
	tdc.status = status
}

func (tdc *TransferFilesCommand) SetStop(stop bool) {
	tdc.stop = stop
}

func (tdc *TransferFilesCommand) SetPreChecks(check bool) {
	tdc.preChecks = check
}

func (tdc *TransferFilesCommand) Run() (err error) {
	if tdc.status {
		return ShowStatus()
	}
	if tdc.stop {
		return tdc.signalStop()
	}
	if err := tdc.stateManager.TryLockTransferStateManager(); err != nil {
		return err
	}
	defer func() {
		unlockErr := tdc.stateManager.UnlockTransferStateManager()
		if err == nil {
			err = unlockErr
		}
	}()

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

	if tdc.preChecks {
		if runner, err := tdc.NewTransferDataPreChecksRunner(); err != nil {
			return err
		} else {
			return runner.Run(tdc.context, tdc.sourceServerDetails)
		}
	}

	if err = tdc.initTransferDir(); err != nil {
		return err
	}

	if err = assertSupportedTransferDirStructure(); err != nil {
		return err
	}

	if err = tdc.initStorageInfoManagers(); err != nil {
		return err
	}

	sourceLocalRepos, sourceBuildInfoRepos, err := tdc.getAllLocalRepos(tdc.sourceServerDetails, tdc.sourceStorageInfoManager)
	if err != nil {
		return err
	}
	allSourceLocalRepos := append(slices.Clone(sourceLocalRepos), sourceBuildInfoRepos...)
	targetLocalRepos, targetBuildInfoRepos, err := tdc.getAllLocalRepos(tdc.targetServerDetails, tdc.targetStorageInfoManager)
	if err != nil {
		return err
	}

	if err = tdc.initLocallyGeneratedFilter(); err != nil {
		return err
	}

	// Handle interruptions
	finishStopping, newPhase := tdc.handleStop(srcUpService)
	defer finishStopping()

	if err = tdc.removeOldFilesIfNeeded(allSourceLocalRepos); err != nil {
		return err
	}

	if err = tdc.initStateManager(allSourceLocalRepos, sourceBuildInfoRepos); err != nil {
		return err
	}

	// Init and Set progress bar with the length of the source local and build info repositories
	err = initTransferProgressMng(allSourceLocalRepos, tdc, 0)
	if err != nil {
		return err
	}

	go tdc.reportTransferFilesUsage()

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

func (tdc *TransferFilesCommand) initStateManager(allSourceLocalRepos, sourceBuildInfoRepos []string) error {
	_, totalBiFiles, err := tdc.sourceStorageInfoManager.GetReposTotalSizeAndFiles(sourceBuildInfoRepos...)
	if err != nil {
		return err
	}
	totalSizeBytes, totalFiles, err := tdc.sourceStorageInfoManager.GetReposTotalSizeAndFiles(allSourceLocalRepos...)
	if err != nil {
		return err
	}
	// Init State Manager's fields values
	tdc.stateManager.OverallTransfer.TotalSizeBytes = totalSizeBytes
	tdc.stateManager.OverallTransfer.TotalUnits = totalFiles
	tdc.stateManager.TotalRepositories.TotalUnits = int64(len(allSourceLocalRepos))
	tdc.stateManager.OverallBiFiles.TotalUnits = totalBiFiles
	if !tdc.ignoreState {
		numberInitialErrors, e := getRetryErrorCount(allSourceLocalRepos)
		if e != nil {
			return e
		}
		tdc.stateManager.TransferFailures = uint(numberInitialErrors)
	} else {
		tdc.stateManager.TransferFailures = 0
	}
	return nil
}

func (tdc *TransferFilesCommand) reportTransferFilesUsage() {
	log.Debug(usage.ReportUsagePrefix + "Sending Transfer Files info...")
	sourceStorageInfo, err := tdc.sourceStorageInfoManager.GetStorageInfo()
	if err != nil {
		log.Debug(err.Error())
		return
	}
	sourceServiceId, err := tdc.sourceStorageInfoManager.GetServiceId()
	if err != nil {
		log.Debug(err.Error())
		return
	}

	reportUsageAttributes := []usage.ReportUsageAttribute{
		{
			AttributeName:  "sourceServiceId",
			AttributeValue: sourceServiceId,
		},
		{
			AttributeName:  "sourceStorageSize",
			AttributeValue: sourceStorageInfo.BinariesSize,
		},
	}
	err = usage.SendReportUsage(coreutils.GetCliUserAgent(), tdc.CommandName(), tdc.targetStorageInfoManager.GetServiceManager(), reportUsageAttributes...)
	if err != nil {
		log.Debug(err.Error())
	}
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

// Creates the Pre-checks runner for the data transfer command
func (tdc *TransferFilesCommand) NewTransferDataPreChecksRunner() (runner *commandsUtils.PreCheckRunner, err error) {
	// Get relevant repos
	serviceManager, err := createTransferServiceManager(tdc.context, tdc.sourceServerDetails)
	if err != nil {
		return
	}
	localRepos, err := utils.GetFilteredRepositoriesByNameAndType(serviceManager, tdc.includeReposPatterns, tdc.excludeReposPatterns, utils.Local)
	if err != nil {
		return
	}
	federatedRepos, err := utils.GetFilteredRepositoriesByNameAndType(serviceManager, tdc.includeReposPatterns, tdc.excludeReposPatterns, utils.Federated)
	if err != nil {
		return
	}

	runner = commandsUtils.NewPreChecksRunner()

	// Add pre checks here
	runner.AddCheck(NewLongPropertyCheck(append(localRepos, federatedRepos...)))

	return
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

	if err = tdc.updateRepoState(repoSummary, buildInfoRepo); err != nil {
		return
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
	for currentPhaseId := 0; currentPhaseId < NumberOfPhases; currentPhaseId++ {
		if tdc.shouldStop() {
			return
		}
		// Ensure the data structure which stores the upload tasks on Artifactory's side is wiped clean,
		// in case some requests to delete handles tasks sent by JFrog CLI did not reach Artifactory.
		err = stopTransferInArtifactory(tdc.sourceServerDetails, srcUpService)
		if err != nil {
			log.Error(err)
		}
		*newPhase = createTransferPhase(currentPhaseId)
		if err = tdc.stateManager.SetRepoPhase(currentPhaseId); err != nil {
			return
		}
		if err = tdc.startPhase(newPhase, sourceRepoKey, buildInfoRepo, *repoSummary, srcUpService); err != nil {
			return
		}
	}
	return tdc.stateManager.IncRepositoriesTransferred()
}

func (tdc *TransferFilesCommand) updateRepoState(repoSummary *serviceUtils.RepositorySummary, buildInfoRepo bool) error {
	filesCount, err := utils.GetFilesCountFromRepositorySummary(repoSummary)
	if err != nil {
		return err
	}

	usedSpaceInBytes, err := utils.GetUsedSpaceInBytes(repoSummary)
	if err != nil {
		return err
	}

	return tdc.stateManager.SetRepoState(repoSummary.RepoKey, usedSpaceInBytes, filesCount, buildInfoRepo, tdc.ignoreState)
}

func (tdc *TransferFilesCommand) initTransferDir() error {
	transferDir, err := coreutils.GetJfrogTransferDir()
	if err != nil {
		return err
	}
	exist, err := fileutils.IsDirExists(transferDir, false)
	if err != nil {
		return err
	}
	if exist {
		// Remove ~/.jfrog/transfer/stop if exist
		if err = os.RemoveAll(filepath.Join(transferDir, StopFileName)); err != nil {
			return errorutils.CheckError(err)
		}
	}
	return errorutils.CheckError(os.MkdirAll(transferDir, 0777))
}

func (tdc *TransferFilesCommand) removeOldFilesIfNeeded(repos []string) error {
	// If we ignore the old state, we need to remove all the old unused files so the process can start clean
	if tdc.ignoreState {
		errFiles, err := getErrorsFiles(repos, true)
		if err != nil {
			return err
		}
		for _, file := range errFiles {
			err = os.Remove(file)
			if err != nil {
				return errorutils.CheckError(err)
			}
		}
		delayFiles, err := getDelayFiles(repos)
		if err != nil {
			return err
		}
		for _, file := range delayFiles {
			err = os.Remove(file)
			if err != nil {
				return errorutils.CheckError(err)
			}
		}
	}
	return nil
}

func (tdc *TransferFilesCommand) startPhase(newPhase *transferPhase, repo string, buildInfoRepo bool, repoSummary serviceUtils.RepositorySummary, srcUpService *srcUserPluginService) error {
	tdc.initNewPhase(*newPhase, srcUpService, repoSummary, repo, buildInfoRepo)
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
		return err
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
	signal.Notify(tdc.stopSignal, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer close(finishStop)
		// Wait for the stop signal or close(stopSignal) to happen
		if <-tdc.stopSignal == nil {
			// The stopSignal channel is closed
			return
		}
		tdc.cancelFunc()
		if newPhase != nil {
			newPhase.StopGracefully()
		}
		log.Info("Gracefully stopping files transfer...")
		err := stopTransferInArtifactory(tdc.sourceServerDetails, srcUpService)
		if err != nil {
			log.Error(err)
		}
	}()

	// Return a cleanup function that closes the stopSignal channel and wait for close if needed
	return func() {
		// Close the stop signal channel
		close(tdc.stopSignal)
		if tdc.shouldStop() {
			// If we should stop, wait for stop to happen
			<-finishStop
		}
	}, &newPhase
}

func (tdc *TransferFilesCommand) initNewPhase(newPhase transferPhase, srcUpService *srcUserPluginService, repoSummary serviceUtils.RepositorySummary, repoKey string, buildInfoRepo bool) {
	newPhase.setContext(tdc.context)
	newPhase.setRepoKey(repoKey)
	newPhase.setCheckExistenceInFilestore(tdc.checkExistenceInFilestore)
	newPhase.setSourceDetails(tdc.sourceServerDetails)
	newPhase.setTargetDetails(tdc.targetServerDetails)
	newPhase.setSrcUserPluginService(srcUpService)
	newPhase.setRepoSummary(repoSummary)
	newPhase.setProgressBar(tdc.progressbar)
	newPhase.setProxyKey(tdc.proxyKey)
	newPhase.setStateManager(tdc.stateManager)
	newPhase.setBuildInfo(buildInfoRepo)
	newPhase.setPackageType(repoSummary.PackageType)
	newPhase.setLocallyGeneratedFilter(tdc.locallyGeneratedFilter)
	newPhase.setStopSignal(tdc.stopSignal)
}

// Get all local and build-info repositories of the input server
// serverDetails      - Source or target server details
// storageInfoManager - Source or target storage info manager
func (tdc *TransferFilesCommand) getAllLocalRepos(serverDetails *config.ServerDetails, storageInfoManager *utils.StorageInfoManager) ([]string, []string, error) {
	serviceManager, err := createTransferServiceManager(tdc.context, serverDetails)
	if err != nil {
		return []string{}, []string{}, err
	}
	excludeRepoPatternsWithBuildInfo := tdc.excludeReposPatterns
	excludeRepoPatternsWithBuildInfo = append(excludeRepoPatternsWithBuildInfo, "*-build-info")
	localRepos, err := utils.GetFilteredRepositoriesByNameAndType(serviceManager, tdc.includeReposPatterns, excludeRepoPatternsWithBuildInfo, utils.Local)
	if err != nil {
		return []string{}, []string{}, err
	}
	federatedRepos, err := utils.GetFilteredRepositoriesByNameAndType(serviceManager, tdc.includeReposPatterns, excludeRepoPatternsWithBuildInfo, utils.Federated)
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

func (tdc *TransferFilesCommand) initLocallyGeneratedFilter() error {
	servicesManager, err := createTransferServiceManager(tdc.context, tdc.targetServerDetails)
	if err != nil {
		return err
	}
	targetArtifactoryVersion, err := servicesManager.GetVersion()
	if err != nil {
		return err
	}
	tdc.locallyGeneratedFilter = NewLocallyGenerated(tdc.context, servicesManager, targetArtifactoryVersion)
	return err
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
	if tdc.stateManager.CurrentRepo.Name != "" {
		e := tdc.stateManager.SaveStateAndSnapshots()
		if e != nil {
			log.Error("Couldn't save transfer state", e)
			if err == nil {
				err = e
			}
		}
	}

	csvErrorsFile, e := createErrorsCsvSummary(sourceRepos, tdc.timeStarted)
	if e != nil {
		log.Error("Couldn't create the errors CSV file", e)
		if err == nil {
			err = e
		}
	}

	if csvErrorsFile != "" {
		log.Info(fmt.Sprintf("Errors occurred during the transfer. Check the errors summary CSV file in: %s", csvErrorsFile))
	}
	return
}

// handleMaxUniqueSnapshots handles special cases regarding the Max Unique Snapshots/Tags setting of repositories of
// these package types: Maven, Gradle, NuGet, Ivy, SBT and Docker.
// TL;DR: we might have repositories in the source with more snapshots than the maximum (in Max Unique Snapshots/Tags),
// so we turn it off at the beginning of the transfer and turn it back on at the end.
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

// Create the '~/.jfrog/transfer/stop' file to mark the transfer-file process to stop
func (tdc *TransferFilesCommand) signalStop() error {
	_, isRunning, err := state.GetRunningTime()
	if err != nil {
		return err
	}
	if !isRunning {
		return errorutils.CheckErrorf("There is no active file transfer process.")
	}

	transferDir, err := coreutils.GetJfrogTransferDir()
	if err != nil {
		return err
	}

	exist, err := fileutils.IsFileExists(filepath.Join(transferDir, StopFileName), false)
	if err != nil {
		return err
	}
	if exist {
		return errorutils.CheckErrorf("Graceful stop is already in progress. Please wait...")
	}

	if stopFile, err := os.Create(filepath.Join(transferDir, StopFileName)); err != nil {
		return errorutils.CheckError(err)
	} else if err = stopFile.Close(); err != nil {
		return errorutils.CheckError(err)
	}
	log.Info("Gracefully stopping files transfer...")
	return nil
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
	return coreutils.ValidateMinimumVersion(coreutils.DataTransfer, currentVersion, dataTransferPluginMinVersion)
}

// Verify connection to the source Artifactory instance, and that the user plugin is installed, responsive, and stands in the minimal version requirement.
func getAndValidateDataTransferPlugin(srcUpService *srcUserPluginService) error {
	verifyResponse, err := srcUpService.verifyCompatibilityRequest()
	if err != nil {
		errMsg := err.Error()
		reason := ""
		if strings.Contains(errMsg, "The execution name '") && strings.Contains(errMsg, "' could not be found") {
			start := strings.Index(errMsg, "'")
			missingApi := errMsg[start+1 : strings.Index(errMsg[start+1:], "'")+start+1]
			reason = fmt.Sprintf(" This is because the '%s' API exposed by the plugin returns a '404 Not Found' response.", missingApi)
		}
		return errorutils.CheckErrorf("%s;\nIt looks like the 'data-transfer' user plugin isn't installed on the source instance."+
			"%s Please refer to the documentation available at "+coreutils.JFrogHelpUrl+"jfrog-hosting-models-documentation/transfer-artifactory-configuration-and-files-to-jfrog-cloud for installation instructions",
			errMsg, reason)
	}

	err = validateDataTransferPluginMinimumVersion(verifyResponse.Version)
	if err != nil {
		return err
	}
	log.Info("data-transfer plugin version: " + verifyResponse.Version)
	return nil
}

// Loop on json files containing FilesErrors and collect them to one FilesErrors object.
func parseErrorsFromLogFiles(logPaths []string) (allErrors FilesErrors, err error) {
	for _, logPath := range logPaths {
		var exists bool
		exists, err = fileutils.IsFileExists(logPath, false)
		if err != nil {
			return
		}
		if !exists {
			err = fmt.Errorf("log file: %s does not exist", logPath)
			return
		}
		var content []byte
		content, err = fileutils.ReadFile(logPath)
		if err != nil {
			return
		}
		fileErrors := new(FilesErrors)
		err = errorutils.CheckError(json.Unmarshal(content, &fileErrors))
		if err != nil {
			return
		}
		allErrors.Errors = append(allErrors.Errors, fileErrors.Errors...)
	}
	return
}

func assertSupportedTransferDirStructure() error {
	return state.VerifyTransferRunStatusVersion()
}
