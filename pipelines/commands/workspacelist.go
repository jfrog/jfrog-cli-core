package commands

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/pipelines/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type WorkspaceListCommand struct {
	serverDetails *config.ServerDetails
}

func NewWorkspaceListCommand() *WorkspaceListCommand {
	return &WorkspaceListCommand{}
}

func (wl *WorkspaceListCommand) ServerDetails() (*config.ServerDetails, error) {
	return wl.serverDetails, nil
}

func (wl *WorkspaceListCommand) SetServerDetails(serverDetails *config.ServerDetails) *WorkspaceListCommand {
	wl.serverDetails = serverDetails
	return wl
}

func (wl *WorkspaceListCommand) CommandName() string {
	return "pl_workspace_list"
}

func (wl *WorkspaceListCommand) Run() error {
	serviceManager, err := manager.CreateServiceManager(wl.serverDetails)
	if err != nil {
		return err
	}
	var workspaces []services.WorkspacesResponse
	workspaces, err = serviceManager.GetWorkspace()
	if err != nil {
		return err
	}
	var jsonResponse []byte
	jsonResponse, err = json.MarshalIndent(workspaces, "", "  ")
	if err != nil {
		return err
	}
	log.Output(coreutils.PrintTitle("Workspaces: "))
	log.Output(string(jsonResponse))
	return nil
}
