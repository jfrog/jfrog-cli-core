package transferdata

import (
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	tasksMaxCapacity = 500000
	// TODO change defaults:
	uploadChunkSize = 2
	defaultThreads  = 2
	// TODO temporary repo:
	singleRepo = "transfer-small-local"
)

type TransferDataCommand struct {
	sourceServerDetails       *config.ServerDetails
	targetServerDetails       *config.ServerDetails
	checkExistenceInFilestore bool
	progressbar               *progressbar.TransferProgressMng
}

func NewTransferDataCommand(sourceServer, targetServer *config.ServerDetails) *TransferDataCommand {
	return &TransferDataCommand{sourceServerDetails: sourceServer, targetServerDetails: targetServer}
}

func (tdc *TransferDataCommand) CommandName() string {
	return "rt_transfer_data"
}

func (tdc *TransferDataCommand) Run() (err error) {
	srcUpService, err := createSrcRtUserPluginServiceManager(tdc.sourceServerDetails)
	if err != nil {
		return err
	}

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

	srcRepos, err := tdc.getAllSrcLocalRepositories()
	if err != nil {
		return err
	}
	// TODO replace with include/exclude repos.
	srcRepos = &[]services.RepositoryDetails{{Key: singleRepo}}

	targetRepos, err := tdc.getAllTargetLocalRepositories()
	if err != nil {
		return err
	}

	// Set progress bar
	// TODO: check int64
	progressBarMng, err := progressbar.NewTransferProgressMng(int64(len(*srcRepos)))
	if err != nil {
		return err
	}
	tdc.progressbar = progressBarMng

	for _, repo := range *srcRepos {
		exists := verifyRepoExistsInTarget(targetRepos, repo.Key)
		if !exists {
			log.Error("Repo '" + repo.Key + "' does not exist in target. Skipping...")
			continue
		}
		progressBarMng.NewRepository(repo.Key)
		for phaseI := 0; phaseI < numberOfPhases; phaseI++ {
			newPhase := getPhaseByNum(phaseI, repo.Key)
			skip, err := newPhase.shouldSkipPhase()
			if err != nil {
				return err
			}
			if skip {
				continue
			}

			tdc.initNewPhase(newPhase, srcUpService)
			err = newPhase.phaseStarted()
			newPhase.initProgressBar()
			if err != nil {
				return err
			}
			log.Debug("Running '" + newPhase.getPhaseName() + "' for repo '" + repo.Key + "'")
			err = newPhase.run()
			if err != nil {
				return err
			}
			err = newPhase.phaseDone()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (tdc *TransferDataCommand) initProgressbar(repoSummaryList *[]utils.RepositorySummary, phaseI int, repoKey string) error {
	switch phaseI {
	case 0:

	case 1:
		// TODO gai

	}
	return nil
}

func (tdc *TransferDataCommand) initNewPhase(newPhase transferPhase, srcUpService *srcUserPluginService) {
	newPhase.shouldCheckExistenceInFilestore(tdc.checkExistenceInFilestore)
	newPhase.setSourceDetails(tdc.sourceServerDetails)
	newPhase.setTargetDetails(tdc.targetServerDetails)
	newPhase.setSrcUserPluginService(srcUpService)
	newPhase.setProgressBar(tdc.progressbar)
}

type producerConsumerDetails struct {
	producerConsumer parallel.Runner
	expectedChan     chan int
	errorsQueue      *clientUtils.ErrorsQueue
	uploadTokensChan chan string
}

func getThreads() int {
	// TODO implement
	return defaultThreads
}
