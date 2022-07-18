package transferfiles

import (
	"fmt"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	serviceUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"strconv"
)

const (
	tasksMaxCapacity = 10000
	uploadChunkSize  = 100
	// Default number of threads working while transferring Artifactory's data
	defaultThreads = 16
	// Size of the channel where the transfer's go routines write the transfer errors
	fileWritersChannelSize = 500000
	retries                = 3
	retriesWait            = 0
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

	// Verify connection to the source Artifactory instance and that the user plugin is installed and responsive.
	version, err := srcUpService.version()
	if err != nil {
		return err
	}
	log.Info("data-transfer plugin version: " + version)

	transferDir, err := coreutils.GetJfrogTransferDir()
	if err != nil {
		return err
	}
	err = os.MkdirAll(transferDir, 0777)
	if err != nil {
		return errorutils.CheckError(err)
	}

	err = tdc.initCurThreads()
	if err != nil {
		return err
	}

	if !isPropertiesPhaseDisabled() {
		cleanStart, err := isCleanStart()
		if err != nil {
			return err
		}
		if cleanStart {
			err = nodeDetection(srcUpService)
			if err != nil {
				return err
			}
		}
	}

	srcRepos, err := tdc.getSrcLocalRepositories()
	if err != nil {
		return err
	}

	targetRepos, err := tdc.getTargetLocalRepositories()
	if err != nil {
		return err
	}

	storageInfo, err := tdc.getSourceStorageInfo()
	if err != nil {
		return err
	}

	// Set progress bar
	tdc.progressbar, err = progressbar.NewTransferProgressMng(int64(len(srcRepos)))
	if err != nil {
		return err
	}

	for _, repo := range srcRepos {
		exists := verifyRepoExistsInTarget(targetRepos, repo)
		if !exists {
			log.Error("repository '" + repo + "' does not exist in target. Skipping...")
			continue
		}

		repoSummary, err := getRepoSummaryFromList(storageInfo.RepositoriesSummaryList, repo)
		if err != nil {
			log.Error(err.Error() + ". Skipping...")
			continue
		}

		if tdc.progressbar != nil {
			tdc.progressbar.NewRepository(repo)
		}
		for currentPhaseId := 0; currentPhaseId < numberOfPhases; currentPhaseId++ {
			err = tdc.startPhase(currentPhaseId, repo, repoSummary, srcUpService)
			if err != nil {
				return tdc.cleanup(err)
			}
		}
	}
	// Close progressBar and create CSV errors summary file
	return tdc.cleanup(nil)
}

func (tdc *TransferFilesCommand) startPhase(currentPhaseId int, repo string, repoSummary serviceUtils.RepositorySummary, srcUpService *srcUserPluginService) error {
	newPhase := getPhaseByNum(currentPhaseId, repo)
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

func (tdc *TransferFilesCommand) initNewPhase(newPhase transferPhase, srcUpService *srcUserPluginService, repoSummary serviceUtils.RepositorySummary) {
	newPhase.shouldCheckExistenceInFilestore(tdc.checkExistenceInFilestore)
	newPhase.setSourceDetails(tdc.sourceServerDetails)
	newPhase.setTargetDetails(tdc.targetServerDetails)
	newPhase.setSrcUserPluginService(srcUpService)
	newPhase.setRepoSummary(repoSummary)
	newPhase.setProgressBar(tdc.progressbar)
}

func (tdc *TransferFilesCommand) getSrcLocalRepositories() ([]string, error) {
	serviceManager, err := utils.CreateServiceManager(tdc.sourceServerDetails, retries, retriesWait, false)
	if err != nil {
		return nil, err
	}
	return utils.GetFilteredRepositories(serviceManager, tdc.includeReposPatterns, tdc.excludeReposPatterns, utils.LOCAL)
}

func (tdc *TransferFilesCommand) getTargetLocalRepositories() ([]string, error) {
	serviceManager, err := utils.CreateServiceManager(tdc.targetServerDetails, retries, retriesWait, false)
	if err != nil {
		return nil, err
	}
	return utils.GetFilteredRepositories(serviceManager, tdc.includeReposPatterns, tdc.excludeReposPatterns, utils.LOCAL)
}

func (tdc *TransferFilesCommand) getSourceStorageInfo() (*serviceUtils.StorageInfo, error) {
	serviceManager, err := utils.CreateServiceManager(tdc.sourceServerDetails, retries, retriesWait, false)
	if err != nil {
		return nil, err
	}
	return serviceManager.StorageInfo(true)
}

func (tdc *TransferFilesCommand) initCurThreads() error {
	// Use default threads if settings file doesn't exist or an error occurred.
	curThreads = defaultThreads
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
