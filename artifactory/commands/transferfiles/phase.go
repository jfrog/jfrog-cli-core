package transferfiles

import (
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	"time"
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
	getPhaseName() string
	getPhaseId() int
	setProgressBar(*progressbar.TransferProgressMng)
	initProgressBar() error
}

type phaseBase struct {
	repoKey                   string
	checkExistenceInFilestore bool
	startTime                 time.Time
	srcUpService              *srcUserPluginService
	srcRtDetails              *coreConfig.ServerDetails
	targetRtDetails           *coreConfig.ServerDetails
	progressBar               *progressbar.TransferProgressMng
}

func getPhaseByNum(i int, repoKey string) transferPhase {
	switch i {
	case 0:
		return &fullTransferPhase{phaseBase: phaseBase{repoKey: repoKey}}
	case 1:
		return &filesDiffPhase{phaseBase: phaseBase{repoKey: repoKey}}
	case 2:
		return &propertiesDiffPhase{phaseBase: phaseBase{repoKey: repoKey}}
	}
	return nil
}
