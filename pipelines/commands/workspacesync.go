package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type WorkspaceSyncCommand struct {
	serverDetails *config.ServerDetails
	project       string
}

func NewWorkspaceSyncCommand() *WorkspaceSyncCommand {
	return &WorkspaceSyncCommand{}
}

func (ws *WorkspaceSyncCommand) ServerDetails() (*config.ServerDetails, error) {
	return ws.serverDetails, nil
}

func (ws *WorkspaceSyncCommand) SetServerDetails(serverDetails *config.ServerDetails) *WorkspaceSyncCommand {
	ws.serverDetails = serverDetails
	return ws
}

func (ws *WorkspaceSyncCommand) SetProject(p string) *WorkspaceSyncCommand {
	ws.project = p
	return ws
}

func (ws *WorkspaceSyncCommand) CommandName() string {
	return "pl_workspace_sync"
}

func (ws *WorkspaceSyncCommand) Run() error {
	serviceManager, err := manager.CreateServiceManager(ws.serverDetails)
	if err != nil {
		return err
	}
	err = serviceManager.WorkspaceSync(ws.project)
	if err != nil {
		return err
	}
	log.Info("Triggered pipelines sync successfully")
	return nil
}
