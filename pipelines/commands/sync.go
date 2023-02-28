package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type SyncCommand struct {
	serverDetails      *config.ServerDetails
	branch             string
	repositoryFullName string
}

func NewSyncCommand() *SyncCommand {
	return &SyncCommand{}
}

func (sc *SyncCommand) ServerDetails() (*config.ServerDetails, error) {
	return sc.serverDetails, nil
}

func (sc *SyncCommand) SetServerDetails(serverDetails *config.ServerDetails) *SyncCommand {
	sc.serverDetails = serverDetails
	return sc
}

func (sc *SyncCommand) CommandName() string {
	return "pl_sync"
}

func (sc *SyncCommand) SetBranch(br string) *SyncCommand {
	sc.branch = br
	return sc
}

func (sc *SyncCommand) SetRepositoryFullName(rfn string) *SyncCommand {
	sc.repositoryFullName = rfn
	return sc
}

func (sc *SyncCommand) Run() error {
	serviceManager, err := manager.CreateServiceManager(sc.serverDetails)
	if err != nil {
		return err
	}
	return serviceManager.SyncPipelineResource(sc.branch, sc.repositoryFullName)
}
