package transferfiles

import (
	"context"
	"errors"
	"time"

	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	serviceUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
)

const numberOfPhases = 3

const (
	FullTransferPhase   int = 0
	FilesDiffPhase      int = 1
	PropertiesDiffPhase int = 2
)

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
	setProgressBar(*TransferProgressMng)
	initProgressBar() error
	StopGracefully()
}

type phaseBase struct {
	context                   context.Context
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

func (pb *phaseBase) ShouldStop() bool {
	return pb.context.Err() != nil
}

// Return InterruptionError, if stop is true
func (pb *phaseBase) getInterruptionErr() error {
	if errors.Is(pb.context.Err(), context.Canceled) {
		return new(InterruptionErr)
	}
	return nil
}

// Indicate graceful stopping in the progress bar
func (pb *phaseBase) StopGracefully() {
	if pb.progressBar != nil {
		pb.progressBar.StopGracefully()
	}
}

func getPhaseByNum(context context.Context, i int, repoKey string, buildInfoRepo bool) transferPhase {
	switch i {
	case 0:
		return &fullTransferPhase{phaseBase: phaseBase{context: context, repoKey: repoKey, phaseId: FullTransferPhase, buildInfoRepo: buildInfoRepo}}
	case 1:
		return &filesDiffPhase{phaseBase: phaseBase{context: context, repoKey: repoKey, phaseId: FilesDiffPhase, buildInfoRepo: buildInfoRepo}}
	case 2:
		return &propertiesDiffPhase{phaseBase: phaseBase{context: context, repoKey: repoKey, phaseId: PropertiesDiffPhase}}
	}
	return nil
}
