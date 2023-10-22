package transferfiles

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"

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
	setLocallyGeneratedFilter(locallyGeneratedFilter *locallyGeneratedFilter)
	initProgressBar() error
	setProxyKey(proxyKey string)
	setBuildInfo(setBuildInfo bool)
	setPackageType(packageType string)
	setStopSignal(stopSignal chan os.Signal)
	StopGracefully()
}

type phaseBase struct {
	context                   context.Context
	repoKey                   string
	buildInfoRepo             bool
	packageType               string
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
	locallyGeneratedFilter    *locallyGeneratedFilter
	stopSignal                chan os.Signal
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
		pb.pcDetails.chunkBuilderProducerConsumer.Cancel(true)
		pb.pcDetails.chunkUploaderProducerConsumer.Cancel(true)
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

func (pb *phaseBase) setLocallyGeneratedFilter(locallyGeneratedFilter *locallyGeneratedFilter) {
	pb.locallyGeneratedFilter = locallyGeneratedFilter
}

func (pb *phaseBase) setBuildInfo(buildInfoRepo bool) {
	pb.buildInfoRepo = buildInfoRepo
}

func (pb *phaseBase) setPackageType(packageType string) {
	pb.packageType = packageType
}

func (pb *phaseBase) setStopSignal(stopSignal chan os.Signal) {
	pb.stopSignal = stopSignal
}

func createTransferPhase(i int) transferPhase {
	// Initialize a pointer to an empty producerConsumerWrapper to allow access the real value in StopGracefully
	curPhaseBase := phaseBase{phaseId: i, pcDetails: &producerConsumerWrapper{}}
	switch i {
	case api.Phase1:
		return &fullTransferPhase{phaseBase: curPhaseBase}
	case api.Phase2:
		return &filesDiffPhase{phaseBase: curPhaseBase}
	case api.Phase3:
		return &errorsRetryPhase{phaseBase: curPhaseBase}
	}
	return nil
}
