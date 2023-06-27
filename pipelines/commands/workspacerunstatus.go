package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/pipelines/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type WorkspaceRunStatusCommand struct {
	serverDetails *config.ServerDetails
	project       string
}

func NewWorkspaceRunStatusCommand() *WorkspaceRunStatusCommand {
	return &WorkspaceRunStatusCommand{}
}

func (wrs *WorkspaceRunStatusCommand) ServerDetails() (*config.ServerDetails, error) {
	return wrs.serverDetails, nil
}

func (wrs *WorkspaceRunStatusCommand) SetServerDetails(serverDetails *config.ServerDetails) *WorkspaceRunStatusCommand {
	wrs.serverDetails = serverDetails
	return wrs
}

func (wrs *WorkspaceRunStatusCommand) SetProject(p string) *WorkspaceRunStatusCommand {
	wrs.project = p
	return wrs
}

func (wrs *WorkspaceRunStatusCommand) CommandName() string {
	return "pl_workspace_sync_status"
}

func (wrs *WorkspaceRunStatusCommand) Run() error {
	serviceManager, err := manager.CreateServiceManager(wrs.serverDetails)
	if err != nil {
		return err
	}
	var pipelinesBranch map[string]string
	pipelinesBranch, err = serviceManager.WorkspacePipelines()
	if err != nil {
		return err
	}
	pipelineNames := make([]string, len(pipelinesBranch))
	for pipName := range pipelinesBranch {
		pipelineNames = append(pipelineNames, pipName)
	}
	var pipeRunIDs []services.PipelinesRunID
	pipeRunIDs, err = serviceManager.WorkspaceRunIDs(pipelineNames)
	if err != nil {
		return err
	}
	for _, runId := range pipeRunIDs {
		log.Debug(coreutils.PrintTitle("Fetching run status for run id "), runId.LatestRunID)
		_, err := serviceManager.WorkspaceRunStatus(runId.LatestRunID)
		if err != nil {
			return err
		}
		err = utils.GetStepStatus(runId, serviceManager)
		if err != nil {
			return err
		}
	}
	return nil
}
