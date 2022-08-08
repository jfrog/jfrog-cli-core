package transferfiles

import (
	"time"

	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	serviceUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
)

const numberOfPhases = 3

type transferPhase interface {
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
	setProgressBar(*progressbar.TransferProgressMng)
	initProgressBar() error
	stopGracefully()
}

type phaseBase struct {
	repoKey                   string
	buildInfoRepo             bool
	phaseId                   int
	checkExistenceInFilestore bool
	startTime                 time.Time
	srcUpService              *srcUserPluginService
	srcRtDetails              *coreConfig.ServerDetails
	targetRtDetails           *coreConfig.ServerDetails
	progressBar               *progressbar.TransferProgressMng
	repoSummary               serviceUtils.RepositorySummary
	stop                      *bool
}

func (pb *phaseBase) shouldStop() bool {
	return *pb.stop
}

// Return InterruptionError, if stop is true
func (pb *phaseBase) getInterruptionErr() error {
	if pb.shouldStop() {
		return new(InterruptionErr)
	}
	return nil
}

// Stop the phase gracefully and show it in the progressbar
func (pb *phaseBase) stopGracefully() {
	*pb.stop = true
	if pb.progressBar != nil {
		pb.progressBar.StopGracefully()
	}
}

func getPhaseByNum(i int, repoKey string, buildInfoRepo bool) transferPhase {
	stopValue := false
	switch i {
	case 0:
		return &fullTransferPhase{phaseBase: phaseBase{repoKey: repoKey, phaseId: 0, buildInfoRepo: buildInfoRepo, stop: &stopValue}}
	case 1:
		return &filesDiffPhase{phaseBase: phaseBase{repoKey: repoKey, phaseId: 1, buildInfoRepo: buildInfoRepo, stop: &stopValue}}
	case 2:
		return &propertiesDiffPhase{phaseBase: phaseBase{repoKey: repoKey, phaseId: 2, stop: &stopValue}}
	}
	return nil
}
