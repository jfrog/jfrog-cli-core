package commands

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/status"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientlog "github.com/jfrog/jfrog-client-go/utils/log"
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
	return "sync"
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
	for _, pipeSyncStatus := range pipelineSyncStatuses {
		pipeSyncStatusCode := status.GetPipelineStatus(pipeSyncStatus.LastSyncStatusCode)
		if clientlog.IsStdErrTerminal() && clientlog.IsColorsSupported() {
			colorCode := status.GetStatusColorCode(pipeSyncStatusCode)
			clientlog.Output(colorCode.Sprintf(PipeStatusFormat, pipeSyncStatusCode, *pipelineSyncStatuses[0].IsSyncing,
				pipelineSyncStatuses[0].LastSyncStartedAt, pipelineSyncStatuses[0].LastSyncEndedAt, pipelineSyncStatuses[0].CommitData.CommitSha,
				pipelineSyncStatuses[0].CommitData.Committer, pipelineSyncStatuses[0].CommitData.CommitMsg, pipelineSyncStatuses[0].LastSyncLogs))
			return nil
		}
		clientlog.Output(fmt.Sprintf(PipeStatusFormat, pipeSyncStatusCode, *pipelineSyncStatuses[0].IsSyncing, pipelineSyncStatuses[0].LastSyncStartedAt,
			pipelineSyncStatuses[0].LastSyncEndedAt, pipelineSyncStatuses[0].CommitData.CommitSha, pipelineSyncStatuses[0].CommitData.Committer,
			pipelineSyncStatuses[0].CommitData.CommitMsg, pipelineSyncStatuses[0].LastSyncLogs))
	}
	return nil
}
