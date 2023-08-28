package commands

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/utils"
	"github.com/jfrog/jfrog-client-go/pipelines/services"
	"strings"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/pipelines"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	RunStatus  = "runStatus"
	SyncStatus = "syncStatus"
	Sync       = "sync"
	Validate   = "validate"
)

type WorkspaceRunCommand struct {
	serverDetails *config.ServerDetails
	pathToFiles   string
	project       string
	values        string
}

type WorkSpaceValidation struct {
	ProjectId   string               `json:"-"`
	Files       []PipelineDefinition `json:"files,omitempty"`
	ProjectName string               `json:"projectName,omitempty"`
	Name        string               `json:"name,omitempty"`
}

func NewWorkspaceCommand() *WorkspaceRunCommand {
	return &WorkspaceRunCommand{}
}

func (wc *WorkspaceRunCommand) ServerDetails() (*config.ServerDetails, error) {
	return wc.serverDetails, nil
}

func (wc *WorkspaceRunCommand) SetServerDetails(serverDetails *config.ServerDetails) *WorkspaceRunCommand {
	wc.serverDetails = serverDetails
	return wc
}

func (wc *WorkspaceRunCommand) SetPipeResourceFiles(f string) *WorkspaceRunCommand {
	wc.pathToFiles = f
	return wc
}

func (wc *WorkspaceRunCommand) SetProject(p string) *WorkspaceRunCommand {
	wc.project = p
	return wc
}

func (wc *WorkspaceRunCommand) SetValues(valuesYaml string) *WorkspaceRunCommand {
	wc.values = valuesYaml
	return wc
}

func (wc *WorkspaceRunCommand) CommandName() string {
	return "pl_workspace_run"
}

func (wc *WorkspaceRunCommand) Run() error {
	serviceManager, err := manager.CreateServiceManager(wc.serverDetails)
	if err != nil {
		return err
	}
	err = wc.workspaceActions(Validate)
	if err != nil {
		return err
	}
	err = wc.pollSyncStatusAndTriggerRun(serviceManager)
	if err != nil {
		return err
	}
	return nil
}

func (wc *WorkspaceRunCommand) workspaceActions(action string) error {
	serviceManager, err := manager.CreateServiceManager(wc.serverDetails)
	if err != nil {
		return err
	}
	switch action {
	case Validate:
		pathToFiles := strings.Split(wc.pathToFiles, ",")
		payload, err := wc.getWorkspaceRunPayload(pathToFiles, wc.project)
		if err != nil {
			return err
		}
		log.Info("Performing validation on pipelines defined")
		err = serviceManager.ValidateWorkspace(payload)
		if err != nil {
			return err
		}
		log.Info(coreutils.PrintTitle("Pipeline resources validation completed successfully"))
	case Sync:
		ws := NewWorkspaceSyncCommand()
		ws.SetServerDetails(wc.serverDetails).
			SetProject(wc.project)
		return commands.Exec(ws)
	case SyncStatus:
		wss := NewWorkspaceSyncStatusCommand()
		wss.SetServerDetails(wc.serverDetails).
			SetProject(wc.project)
		return commands.Exec(wss)
	case RunStatus:
		wrs := NewWorkspaceRunStatusCommand()
		wrs.SetServerDetails(wc.serverDetails).
			SetProject(wc.project)
		return commands.Exec(wrs)
	}
	return nil
}

// getWorkspaceRunPayload prepares request body for workspace validation
func (wc *WorkspaceRunCommand) getWorkspaceRunPayload(pipelineFiles []string, project string) ([]byte, error) {
	pipelineDefinitions, err := structureFileContentAsPipelineDefinition(pipelineFiles, wc.values)
	if err != nil {
		return []byte{}, err
	}
	if len(project) == 0 {
		project = "default"
	}
	workSpaceValidation := WorkSpaceValidation{
		Files:       pipelineDefinitions,
		ProjectName: project,
		Name:        project,
	}
	return json.Marshal(workSpaceValidation)
}

func (wc *WorkspaceRunCommand) pollSyncStatusAndTriggerRun(serviceManager *pipelines.PipelinesServicesManager) error {
	err := wc.workspaceActions(SyncStatus)
	if err != nil {
		return err
	}
	var pipelinesBranch map[string]string
	pipelinesBranch, err = serviceManager.WorkspacePipelines()
	if err != nil {
		return err
	}
	pipelineNames := make([]string, 0)
	for pipName, branch := range pipelinesBranch {
		log.Info(coreutils.PrintTitle("Triggering pipeline run for: "), pipName)
		pipelineNames = append(pipelineNames, pipName)
		err := serviceManager.TriggerPipelineRun(branch, pipName, false)
		if err != nil {
			return err
		}
	}
	log.Debug("Collecting run ids from pipelines defined in workspace")
	var pipeRunIDs []services.PipelinesRunID
	pipeRunIDs, err = serviceManager.WorkspaceRunIDs(pipelineNames)
	if err != nil {
		return err
	}

	for _, runId := range pipeRunIDs {
		log.Debug("Fetching run status for run id: ", runId.LatestRunID)
		time.Sleep(5 * time.Second)
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
