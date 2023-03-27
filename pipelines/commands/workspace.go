package commands

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/status"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/pipelines"
	"github.com/jfrog/jfrog-client-go/pipelines/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/slices"
	"strconv"
)

type WorkspaceCommand struct {
	serverDetails *config.ServerDetails
	pathToFile    string
	data          []byte
}

type StepStatus struct {
	Name             string `col-name:"Name"`
	StatusCode       int
	TriggeredAt      string                `col-name:"Triggered At"`
	ExternalBuildUrl string                `col-name:"Build URL"`
	StatusString     status.PipelineStatus `col-name:"Status Code"`
	Id               int
}

func NewWorkspaceCommand() *WorkspaceCommand {
	return &WorkspaceCommand{}
}

func (ws *WorkspaceCommand) ServerDetails() (*config.ServerDetails, error) {
	return ws.serverDetails, nil
}

func (ws *WorkspaceCommand) SetServerDetails(serverDetails *config.ServerDetails) *WorkspaceCommand {
	ws.serverDetails = serverDetails
	return ws
}

func (ws *WorkspaceCommand) SetPipeResourceFiles(f string) *WorkspaceCommand {
	ws.pathToFile = f
	return ws
}

func (ws *WorkspaceCommand) Run() error {
	serviceManager, err := manager.CreateServiceManager(ws.serverDetails)
	if err != nil {
		return err
	}
	vc := NewValidateCommand()
	vc.SetServerDetails(ws.serverDetails)
	vc.SetPipeResourceFiles(ws.pathToFile)
	fileContent, err := vc.ValidateResources()
	if err != nil {
		return err
	}
	log.Info("Performing validation on pipeline resources")
	err = serviceManager.ValidateWorkspace(fileContent)
	if err != nil {
		return err
	}
	log.Info(coreutils.PrintTitle("Pipeline resources validation completed successfully"))
	pipelinesBranch, err := serviceManager.WorkspacePipelines()
	if err != nil {
		return err
	}
	for pipName, branch := range pipelinesBranch {
		log.Info(coreutils.PrintTitle("Triggering pipeline run for "), pipName)
		err := serviceManager.TriggerPipelineRun(branch, pipName, false)
		if err != nil {
			return err
		}
	}
	pipelineNames := make([]string, len(pipelinesBranch))
	i := 0
	for k := range pipelinesBranch {
		pipelineNames[i] = k
		i++
	}
	log.Debug("Collecting run ids from pipelines defined in workspace")
	pipeRunIDs, err := serviceManager.WorkspaceRunIDs(pipelineNames)
	if err != nil {
		return err
	}

	for _, runId := range pipeRunIDs {
		log.Debug(coreutils.PrintTitle("Fetching run status for run id "), runId.LatestRunID)
		_, err := serviceManager.WorkspaceRunStatus(runId.LatestRunID)
		if err != nil {
			return err
		}
		_, err = ws.getStepStatus(runId, serviceManager)
		if err != nil {
			return err
		}
	}
	return nil
}

// getStepStatus for the given pipeline run fetch associated steps
// and print status in table format
func (ws *WorkspaceCommand) getStepStatus(runId services.PipelinesRunID, serviceManager *pipelines.PipelinesServicesManager) (string, error) {
	for {
		stopCapturingStepStatus := true
		log.Debug("Fetching step status for run id ", runId.LatestRunID)
		stepStatus, err := serviceManager.WorkspaceStepStatus(runId.LatestRunID)
		if err != nil {
			return "", err
		}
		stepTable := make([]StepStatus, 0)
		err = json.Unmarshal(stepStatus, &stepTable)
		if err != nil {
			return "", err
		}
		// Cloning to preserve original response when deletes are performed
		endState := slices.Clone(stepTable)
		for i := 0; i < len(stepTable); i++ {
			stepTable[i].StatusString = status.GetPipelineStatus(stepTable[i].StatusCode)
			stopCapturingStepStatus = stopCapturingStepStatus && isStepCompleted(stepTable[i].StatusString)
			if !slices.Contains(status.GetWaitingForRunAndRunningSteps(), stepTable[i].StatusString) {
				stepTable = slices.Delete(stepTable, i, i+1)
				i--
			}
		}
		err = coreutils.PrintTable(stepTable, coreutils.PrintTitle(runId.Name+" Step Status"), "All steps reached end state", true)
		if err != nil {
			return "", err
		}
		if !stopCapturingStepStatus {
			// Keep polling for steps status until all steps are processed
			continue
		}
		err = coreutils.PrintTable(endState, coreutils.PrintTitle(runId.Name+" Step Status"), "No Pipeline steps available", true)
		if err != nil {
			return "", err
		}
		// Get Step Logs and print
		for i := 0; i < len(endState); i++ {
			endState[i].StatusString = status.GetPipelineStatus(endState[i].StatusCode)
			log.Output(coreutils.PrintTitle("Fetching logs for step " + endState[i].Name))
			err := ws.getPipelineStepLogs(strconv.Itoa(endState[i].Id), serviceManager)
			if err != nil {
				return "", err
			}
		}
		return "", nil
	}
}

// getPipelineStepLogs invokes to fetch pipeline step logs for the given step ID
func (ws *WorkspaceCommand) getPipelineStepLogs(stepID string, serviceManager *pipelines.PipelinesServicesManager) error {
	consoles, err := serviceManager.GetStepConsoles(stepID)
	if err != nil {
		return err
	}
	for _, v := range consoles {
		for _, console := range v {
			if console.IsShown {
				log.Output(console.CreatedAt, "  ", console.Message)
			}
		}
	}
	return nil
}

func isStepCompleted(stepStatus status.PipelineStatus) bool {
	return slices.Contains(status.GetRunCompletedStatusList(), stepStatus)
}
