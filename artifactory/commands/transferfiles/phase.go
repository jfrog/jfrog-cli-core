package transferfiles

import (
	"time"

	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	serviceUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
)

const numberOfPhases = 3

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
		return &fullTransferPhase{phaseBase: phaseBase{repoKey: repoKey, phaseId: 0, buildInfoRepo: buildInfoRepo, Stoppable: stoppable}}
	case 1:
		return &filesDiffPhase{phaseBase: phaseBase{repoKey: repoKey, phaseId: 1, buildInfoRepo: buildInfoRepo, Stoppable: stoppable}}
	case 2:
		return &propertiesDiffPhase{phaseBase: phaseBase{repoKey: repoKey, phaseId: 2, Stoppable: stoppable}}
	}
	return nil
}
