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
	FullTransferPhase int = 0
	FilesDiffPhase    int = 1
	ErrorsPhase       int = 2
)

type transferPhase interface {
	run() error
	phaseStarted() error
	phaseDone() error
	setCheckExistenceInFilestore(bool)
	shouldSkipPhase() (bool, error)
	setSrcUserPluginService(*srcUserPluginService)
	setSourceDetails(*coreConfig.ServerDetails)
	getSourceDetails() *coreConfig.ServerDetails
	setTargetDetails(*coreConfig.ServerDetails)
	setRepoSummary(serviceUtils.RepositorySummary)
	getPhaseName() string
	setProgressBar(*TransferProgressMng)
	setTimeEstMng(timeEstMng *timeEstimationManager)
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
	timeEstMng                *timeEstimationManager
	proxyKey                  string
	pcDetails                 *producerConsumerWrapper
	transferManager           *transferManager
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
		pb.pcDetails.notifyIfBuilderFinished(true)
		pb.pcDetails.notifyIfUploaderFinished(true)
	}
}

func (pb *phaseBase) getSourceDetails() *coreConfig.ServerDetails {
	return pb.srcRtDetails
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

func (pb *phaseBase) setTimeEstMng(timeEstMng *timeEstimationManager) {
	pb.timeEstMng = timeEstMng
}

func (pb *phaseBase) setProgressBar(progressbar *TransferProgressMng) {
	pb.progressBar = progressbar
}

func getPhaseByNum(context context.Context, i int, repoKey, proxyKey string, buildInfoRepo bool) transferPhase {
	switch i {
	case 0:
		return &fullTransferPhase{phaseBase: phaseBase{context: context, repoKey: repoKey, proxyKey: proxyKey, phaseId: FullTransferPhase, buildInfoRepo: buildInfoRepo}}
	case 1:
		return &filesDiffPhase{phaseBase: phaseBase{context: context, repoKey: repoKey, proxyKey: proxyKey, phaseId: FilesDiffPhase, buildInfoRepo: buildInfoRepo}}
	case 2:
		return &errorsRetryPhase{phaseBase: phaseBase{context: context, repoKey: repoKey, proxyKey: proxyKey, phaseId: ErrorsPhase, buildInfoRepo: buildInfoRepo}}
	}
	return nil
}
