package transferfiles

import (
	"context"
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	serviceUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
)

const NumberOfPhases = 3

type transferPhase interface {
	run() error
	phaseStarted() error
	phaseDone() error
	setContext(context context.Context)
	setRepoKey(repoKey string)
	setCheckExistenceInFilestore(bool)
	shouldSkipPhase() (bool, error)
	setSrcUserPluginService(*srcUserPluginService)
	setSourceDetails(*coreConfig.ServerDetails)
	getSourceDetails() *coreConfig.ServerDetails
	setTargetDetails(*coreConfig.ServerDetails)
	setRepoSummary(serviceUtils.RepositorySummary)
	getPhaseName() string
	setProgressBar(*TransferProgressMng)
	setStateManager(stateManager *state.TransferStateManager)
	initProgressBar() error
	setProxyKey(proxyKey string)
	setBuildInfo(setBuildInfo bool)
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
	proxyKey                  string
	pcDetails                 *producerConsumerWrapper
	transferManager           *transferManager
	stateManager              *state.TransferStateManager
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

// Stop and indicate graceful stopping in the progress bar
func (pb *phaseBase) StopGracefully() {
	if pb.progressBar != nil {
		pb.progressBar.StopGracefully()
	}
	if pb.pcDetails != nil {
		pb.pcDetails.chunkBuilderProducerConsumer.Cancel()
		pb.pcDetails.chunkUploaderProducerConsumer.Cancel()
	}
}

func (pb *phaseBase) getSourceDetails() *coreConfig.ServerDetails {
	return pb.srcRtDetails
}

func (pb *phaseBase) setContext(context context.Context) {
	pb.context = context
}

func (pb *phaseBase) setRepoKey(repoKey string) {
	pb.repoKey = repoKey
}

func (pb *phaseBase) setCheckExistenceInFilestore(shouldCheck bool) {
	pb.checkExistenceInFilestore = shouldCheck
}

func (pb *phaseBase) setSrcUserPluginService(service *srcUserPluginService) {
	pb.srcUpService = service
}

func (pb *phaseBase) setSourceDetails(details *coreConfig.ServerDetails) {
	pb.srcRtDetails = details
}

func (pb *phaseBase) setTargetDetails(details *coreConfig.ServerDetails) {
	pb.targetRtDetails = details
}

func (pb *phaseBase) setRepoSummary(repoSummary serviceUtils.RepositorySummary) {
	pb.repoSummary = repoSummary
}

func (pb *phaseBase) setProgressBar(progressbar *TransferProgressMng) {
	pb.progressBar = progressbar
}

func (pb *phaseBase) setProxyKey(proxyKey string) {
	pb.proxyKey = proxyKey
}

func (pb *phaseBase) setStateManager(stateManager *state.TransferStateManager) {
	pb.stateManager = stateManager
}

func (pb *phaseBase) setBuildInfo(buildInfoRepo bool) {
	pb.buildInfoRepo = buildInfoRepo
}

func createTransferPhase(i int) transferPhase {
	curPhaseBase := phaseBase{phaseId: i}
	switch i {
	case api.FullTransferPhase:
		return &fullTransferPhase{phaseBase: curPhaseBase}
	case api.FilesDiffPhase:
		return &filesDiffPhase{phaseBase: curPhaseBase}
	case api.ErrorsPhase:
		return &errorsRetryPhase{phaseBase: curPhaseBase}
	}
	return nil
}
