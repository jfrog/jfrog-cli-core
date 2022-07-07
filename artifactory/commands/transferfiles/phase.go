package transferfiles

import (
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	serviceUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
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
	setRepoSummary(serviceUtils.RepositorySummary)
	getPhaseName() string
	setProgressBar(*progressbar.TransferProgressMng)
	initProgressBar() error
	stopGracefully()
}

type phaseBase struct {
	repoKey                   string
	phaseId                   int
	checkExistenceInFilestore bool
	startTime                 time.Time
	srcUpService              *srcUserPluginService
	srcRtDetails              *coreConfig.ServerDetails
	targetRtDetails           *coreConfig.ServerDetails
	progressBar               *progressbar.TransferProgressMng
	repoSummary               serviceUtils.RepositorySummary
	stop                      bool
}

func getPhaseByNum(i int, repoKey string) transferPhase {
	switch i {
	case 0:
		return &fullTransferPhase{phaseBase: phaseBase{repoKey: repoKey, phaseId: 0}}
	case 1:
		return &filesDiffPhase{phaseBase: phaseBase{repoKey: repoKey, phaseId: 1}}
	case 2:
		return &propertiesDiffPhase{phaseBase: phaseBase{repoKey: repoKey, phaseId: 2}}
	}
	return nil
}
