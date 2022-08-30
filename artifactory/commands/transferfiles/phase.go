package transferfiles

import (
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/repostate"
	serviceUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"time"
)

const numberOfPhases = 3

const (
	FullTransferPhase   int = 0
	FilesDiffPhase      int = 1
	PropertiesDiffPhase int = 2
)

type transferPhase interface {
	StoppableComponent
	run() error
	phaseStarted() error
	phaseDone() error
	shouldCheckExistenceInFilestore(bool)
	shouldSkipPhase() (bool, error)
	setSrcUserPluginService(*srcUserPluginService)
	setSourceDetails(*coreConfig.ServerDetails)
	getSourceDetails() *coreConfig.ServerDetails
	setTargetDetails(*coreConfig.ServerDetails)
	setRepoSummary(serviceUtils.RepositorySummary)
	getPhaseName() string
	setProgressBar(*TransferProgressMng)
	initProgressBar() error
}

type phaseBase struct {
	*Stoppable
	repoKey                   string
	buildInfoRepo             bool
	phaseId                   int
	checkExistenceInFilestore bool
	startTime                 time.Time
	srcUpService              *srcUserPluginService
	srcRtDetails              *coreConfig.ServerDetails
	targetRtDetails           *coreConfig.ServerDetails
	progressBar               *TransferProgressMng
	repoSummary               serviceUtils.RepositorySummary
	stateManager              *repostate.RepoStateManager
}

// Return InterruptionError, if stop is true
func (pb *phaseBase) getInterruptionErr() error {
	if pb.ShouldStop() {
		return new(InterruptionErr)
	}
	return nil
}

// Stop the phase gracefully and show it in the progressbar
func (pb *phaseBase) Stop() {
	pb.Stoppable.Stop()
	if pb.progressBar != nil {
		pb.progressBar.StopGracefully()
	}
}

func getPhaseByNum(i int, repoKey string, buildInfoRepo bool) transferPhase {
	stoppable := new(Stoppable)
	switch i {
	case 0:
		return &fullTransferPhase{phaseBase: phaseBase{repoKey: repoKey, phaseId: FullTransferPhase, buildInfoRepo: buildInfoRepo, Stoppable: stoppable}}
	case 1:
		return &filesDiffPhase{phaseBase: phaseBase{repoKey: repoKey, phaseId: FilesDiffPhase, buildInfoRepo: buildInfoRepo, Stoppable: stoppable}}
	case 2:
		return &propertiesDiffPhase{phaseBase: phaseBase{repoKey: repoKey, phaseId: PropertiesDiffPhase, Stoppable: stoppable}}
	}
	return nil
}
