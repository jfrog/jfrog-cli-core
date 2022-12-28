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

func (sc *SyncStatusCommand) Run() (string, error) {
	serviceManager, err := manager.CreateServiceManager(sc.serverDetails)
	if err != nil {
		return "", err
	}

	pipelineSyncStatuses, syncServErr := serviceManager.GetSyncStatusForPipelineResource(sc.repoPath, sc.branch)
	if err != nil {
		return "", syncServErr
	}
	pipSyncStatus := status.GetPipelineStatus(pipelineSyncStatuses[0].LastSyncStatusCode)
	if clientlog.IsStdErrTerminal() && clientlog.IsColorsSupported() {
		colorCode := status.GetStatusColorCode(pipSyncStatus)
		s := colorCode.Sprintf("Status: %s\nIsSyncing: %t\nLastSyncStartedAt: %s\nLastSyncEndedAt: %s\nCommitSHA: %s\nCommitter: %s\nCommitMessage: %s\nSyncSummary: %s\n", pipSyncStatus, *pipelineSyncStatuses[0].IsSyncing, pipelineSyncStatuses[0].LastSyncStartedAt, pipelineSyncStatuses[0].LastSyncEndedAt, pipelineSyncStatuses[0].CommitData.CommitSha, pipelineSyncStatuses[0].CommitData.Committer, pipelineSyncStatuses[0].CommitData.CommitMsg, pipelineSyncStatuses[0].LastSyncLogs)
		return s, nil
	}
	s := fmt.Sprintf("Status: %s\nIsSyncing: %t\nLastSyncStartedAt: %s\nLastSyncEndedAt: %s\nCommitSHA: %s\nCommitter: %s\nCommitMessage: %s\n", pipSyncStatus, *pipelineSyncStatuses[0].IsSyncing, pipelineSyncStatuses[0].LastSyncStartedAt, pipelineSyncStatuses[0].LastSyncEndedAt, pipelineSyncStatuses[0].CommitData.CommitSha, pipelineSyncStatuses[0].CommitData.Committer, pipelineSyncStatuses[0].CommitData.CommitMsg)
	return s, nil
}
