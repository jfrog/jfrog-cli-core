package commands

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/status"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/pipelines/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type SyncStatusCommand struct {
	serverDetails *config.ServerDetails
	branch        string
	repoPath      string
}

const (
	PipeStatusFormat = "Status: %s\nIsSyncing: %t\nLastSyncStartedAt: %s\nLastSyncEndedAt: %s\n" +
		"CommitSHA: %s\nCommitter: %s\nCommitMessage: %s\nSyncSummary: %s\n"
)

func NewSyncStatusCommand() *SyncStatusCommand {
	return &SyncStatusCommand{}
}

func (sc *SyncStatusCommand) ServerDetails() (*config.ServerDetails, error) {
	return sc.serverDetails, nil
}

func (sc *SyncStatusCommand) SetServerDetails(serverDetails *config.ServerDetails) *SyncStatusCommand {
	sc.serverDetails = serverDetails
	return sc
}

func (sc *SyncStatusCommand) CommandName() string {
	return "pl_sync_status"
}

func (sc *SyncStatusCommand) SetBranch(br string) *SyncStatusCommand {
	sc.branch = br
	return sc
}

func (sc *SyncStatusCommand) SetRepoPath(repo string) *SyncStatusCommand {
	sc.repoPath = repo
	return sc
}

func (sc *SyncStatusCommand) Run() error {
	serviceManager, err := manager.CreateServiceManager(sc.serverDetails)
	if err != nil {
		return err
	}
	// Filter pipeline resources sync status with repository name and branch name
	pipelineSyncStatuses, err := serviceManager.GetSyncStatusForPipelineResource(sc.repoPath, sc.branch)
	if err != nil {
		return err
	}
	sc.displaySyncStatus(pipelineSyncStatuses)
	return nil
}

// displaySyncStatus outputs to stdout the sync status
func (sc *SyncStatusCommand) displaySyncStatus(pipelineSyncStatuses []services.PipelineSyncStatus) {
	for index, pipeSyncStatus := range pipelineSyncStatuses {
		pipeSyncStatusCode := status.GetPipelineStatus(pipeSyncStatus.LastSyncStatusCode)
		if log.IsStdErrTerminal() && log.IsColorsSupported() {
			colorCode := status.GetStatusColorCode(pipeSyncStatusCode)
			log.Output(colorCode.Sprintf(PipeStatusFormat, pipeSyncStatusCode, *pipelineSyncStatuses[index].IsSyncing,
				pipelineSyncStatuses[index].LastSyncStartedAt, pipelineSyncStatuses[index].LastSyncEndedAt, pipelineSyncStatuses[index].CommitData.CommitSha,
				pipelineSyncStatuses[index].CommitData.Committer, pipelineSyncStatuses[index].CommitData.CommitMsg, pipelineSyncStatuses[index].LastSyncLogs))
			return
		}
		log.Output(fmt.Sprintf(PipeStatusFormat, pipeSyncStatusCode, *pipelineSyncStatuses[index].IsSyncing, pipelineSyncStatuses[index].LastSyncStartedAt,
			pipelineSyncStatuses[index].LastSyncEndedAt, pipelineSyncStatuses[index].CommitData.CommitSha, pipelineSyncStatuses[index].CommitData.Committer,
			pipelineSyncStatuses[index].CommitData.CommitMsg, pipelineSyncStatuses[index].LastSyncLogs))
	}
}
