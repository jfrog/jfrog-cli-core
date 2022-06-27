package transferfiles

import coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"

const numberOfPhases = 3

type transferPhase interface {
	run() error
	phaseStarted() error
	phaseDone() error
	shouldCheckExistenceInFilestore(bool)
	shouldSkipPhase() (bool, error)
	setSrcUserPluginService(*srcUserPluginService)
	setSourceDetails(*coreConfig.ServerDetails)
	setTargetDetails(*coreConfig.ServerDetails)
	getPhaseName() string
}

func getPhaseByNum(i int, repoKey string) transferPhase {
	switch i {
	case 0:
		return &migrationPhase{repoKey: repoKey}
	case 1:
		return &filesDiffPhase{repoKey: repoKey}
	case 2:
		return &propertiesDiffPhase{repoKey: repoKey}
	}
	return nil
}
