package transferfiles

import (
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
)

const (
	tasksMaxCapacity = 10000
	uploadChunkSize  = 10
	defaultThreads   = 16
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
	return "rt_transfer_data"
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
	transferDir, err := coreutils.GetJfrogTransferDir()
	if err != nil {
		return err
	}
	err = os.MkdirAll(transferDir, 0777)
	if err != nil {
		return errorutils.CheckError(err)
	}

	srcUpService, err := createSrcRtUserPluginServiceManager(tdc.sourceServerDetails)
	if err != nil {
		return err
	}

	cleanStart, err := isCleanStart()
	if err != nil {
		return err
	}
	if cleanStart && !isPropertiesPhaseDisabled() {
		err = nodeDetection(srcUpService)
		if err != nil {
			return err
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

	// Set progress bar
	progressBarMng, err := progressbar.NewTransferProgressMng(int64(len(srcRepos)))
	if err != nil {
		return err
	}
	tdc.progressbar = progressBarMng

	for _, repo := range srcRepos {
		exists := verifyRepoExistsInTarget(targetRepos, repo)
		if !exists {
			log.Error("Repo '" + repo + "' does not exist in target. Skipping...")
			continue
		}
		if tdc.progressbar != nil {
			tdc.progressbar.NewRepository(repo)
		}
		for phaseI := 0; phaseI < numberOfPhases; phaseI++ {
			newPhase := getPhaseByNum(phaseI, repo)
			tdc.initNewPhase(newPhase, srcUpService)
			skip, err := newPhase.shouldSkipPhase()
			if err != nil {
				return err
			}
			if skip {
				continue
			}
			err = newPhase.phaseStarted()
			if err != nil {
				return err
			}
			err = newPhase.initProgressBar()
			if err != nil {
				return err
			}
			log.Debug("Running '" + newPhase.getPhaseName() + "' for repo '" + repo + "'")
			err = newPhase.run()
			if err != nil {
				return err
			}
			err = newPhase.phaseDone()
			if err != nil {
				return err
			}
			log.Debug("Done running '" + newPhase.getPhaseName() + "' for repo '" + repo + "'")
		}
	}
	if tdc.progressbar != nil {
		err = tdc.progressbar.Quit()
	}
	return err
}

func (tdc *TransferFilesCommand) initNewPhase(newPhase transferPhase, srcUpService *srcUserPluginService) {
	newPhase.shouldCheckExistenceInFilestore(tdc.checkExistenceInFilestore)
	newPhase.setSourceDetails(tdc.sourceServerDetails)
	newPhase.setTargetDetails(tdc.targetServerDetails)
	newPhase.setSrcUserPluginService(srcUpService)
	newPhase.setProgressBar(tdc.progressbar)
}

func (tdc *TransferFilesCommand) getSrcLocalRepositories() ([]string, error) {
	serviceManager, err := utils.CreateServiceManager(tdc.sourceServerDetails, -1, 0, false)
	if err != nil {
		return nil, err
	}
	return utils.GetFilteredRepositories(serviceManager, tdc.includeReposPatterns, tdc.excludeReposPatterns, utils.LOCAL)
}

func (tdc *TransferFilesCommand) getTargetLocalRepositories() ([]string, error) {
	serviceManager, err := utils.CreateServiceManager(tdc.targetServerDetails, -1, 0, false)
	if err != nil {
		return nil, err
	}
	return utils.GetFilteredRepositories(serviceManager, tdc.includeReposPatterns, tdc.excludeReposPatterns, utils.LOCAL)
}

type producerConsumerDetails struct {
	producerConsumer parallel.Runner
	expectedChan     chan int
	errorsQueue      *clientUtils.ErrorsQueue
	uploadTokensChan chan string
}
