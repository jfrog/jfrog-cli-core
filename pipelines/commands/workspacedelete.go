package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type WorkspaceDeleteCommand struct {
	serverDetails *config.ServerDetails
	project       string
}

func NewWorkspaceDeleteCommand() *WorkspaceDeleteCommand {
	return &WorkspaceDeleteCommand{}
}

func (wd *WorkspaceDeleteCommand) ServerDetails() (*config.ServerDetails, error) {
	return wd.serverDetails, nil
}

func (wd *WorkspaceDeleteCommand) SetServerDetails(serverDetails *config.ServerDetails) *WorkspaceDeleteCommand {
	wd.serverDetails = serverDetails
	return wd
}

func (wd *WorkspaceDeleteCommand) SetProject(p string) *WorkspaceDeleteCommand {
	wd.project = p
	return wd
}

func (wd *WorkspaceDeleteCommand) CommandName() string {
	return "pl_workspace_delete"
}

func (wd *WorkspaceDeleteCommand) Run() error {
	serviceManager, err := manager.CreateServiceManager(wd.serverDetails)
	if err != nil {
		return err
	}
	log.Info("Project name received is ", wd.project)
	err = serviceManager.DeleteWorkspace(wd.project)
	if err != nil {
		return err
	}
	log.Info("Deleted workspace for ", wd.project)
	return nil
}
