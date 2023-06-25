package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/status"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/pipelines"
	"github.com/jfrog/jfrog-client-go/pipelines/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/slices"
)

const (
	RunStatus  = "runStatus"
	SyncStatus = "syncStatus"
	Sync       = "sync"
	Validate   = "validate"
)

type WorkspaceCommand struct {
	serverDetails *config.ServerDetails
	pathToFile    string
	data          []byte
	project       string
	values        string
}

type StepStatus struct {
	Name             string `col-name:"Name"`
	StatusCode       int    `col-name:"StatusCode"`
	TriggeredAt      string `col-name:"Triggered At"`
	ExternalBuildUrl string
	StatusString     string `col-name:"Status"`
	Id               int
}

type PipelineDefinition struct {
	FileName string `json:"fileName,omitempty"`
	Content  string `json:"content,omitempty"`
	YmlType  string `json:"ymlType,omitempty"`
}

type WorkSpaceValidation struct {
	ProjectId   string               `json:"-"`
	Files       []PipelineDefinition `json:"files,omitempty"`
	ProjectName string               `json:"projectName,omitempty"`
	Name        string               `json:"name,omitempty"`
}

func NewWorkspaceCommand() *WorkspaceCommand {
	return &WorkspaceCommand{}
}

func (wc *WorkspaceCommand) ServerDetails() (*config.ServerDetails, error) {
	return wc.serverDetails, nil
}

func (wc *WorkspaceCommand) SetServerDetails(serverDetails *config.ServerDetails) *WorkspaceCommand {
	wc.serverDetails = serverDetails
	return wc
}

func (wc *WorkspaceCommand) SetPipeResourceFiles(f string) *WorkspaceCommand {
	wc.pathToFile = f
	return wc
}

func (wc *WorkspaceCommand) SetProject(p string) *WorkspaceCommand {
	wc.project = p
	return wc
}

func (wc *WorkspaceCommand) SetValues(valuesYaml string) *WorkspaceCommand {
	wc.values = valuesYaml
	return wc
}

func (wc *WorkspaceCommand) CommandName() string {
	return "pl_workspace"
}

func (wc *WorkspaceCommand) Run() error {
	serviceManager, err := manager.CreateServiceManager(wc.serverDetails)
	if err != nil {
		return err
	}
	pipelineFiles := wc.pathToFile
	pipelineDefinitions := strings.Split(pipelineFiles, ",")
	payload, err := wc.getWorkspaceRunPayload(pipelineDefinitions, wc.project)
	if err != nil {
		return err
	}
	log.Info("Performing validation on pipeline resources")
	fmt.Printf("%+v \n", string(payload))
	err = serviceManager.ValidateWorkspace(payload)
	if err != nil {
		return err
	}
	log.Info(coreutils.PrintTitle("Pipeline resources validation completed successfully"))
	err = wc.pollSyncStatusAndTriggerRun(serviceManager)
	if err != nil {
		return err
	}
	return nil
}

func (wc *WorkspaceCommand) pollSyncStatusAndTriggerRun(serviceManager *pipelines.PipelinesServicesManager) error {
	err := wc.WorkspaceActions(SyncStatus)
	if err != nil {
		return err
	}
	pipelinesBranch, err := serviceManager.WorkspacePipelines()
	if err != nil {
		return err
	}
	pipelineNames := make([]string, len(pipelinesBranch))
	for pipName, branch := range pipelinesBranch {
		log.Info(coreutils.PrintTitle("Triggering pipeline run for "), pipName)
		pipelineNames = append(pipelineNames, pipName)
		err := serviceManager.TriggerPipelineRun(branch, pipName, false)
		if err != nil {
			return err
		}
	}
	log.Debug("Collecting run ids from pipelines defined in workspace")
	pipeRunIDs, err := serviceManager.WorkspaceRunIDs(pipelineNames)
	if err != nil {
		return err
	}

	for _, runId := range pipeRunIDs {
		log.Debug(coreutils.PrintTitle("Fetching run status for run id "), runId.LatestRunID)
		time.Sleep(5 * time.Second)
		_, err := serviceManager.WorkspaceRunStatus(runId.LatestRunID)
		if err != nil {
			return err
		}
		err = wc.getStepStatus(runId, serviceManager)
		if err != nil {
			return err
		}
	}
	return nil
}

func (wc *WorkspaceCommand) WorkspaceActions(action string) error {
	serviceManager, err := manager.CreateServiceManager(wc.serverDetails)
	if err != nil {
		return err
	}
	switch action {
	case Validate:
		pipelineFiles := wc.pathToFile
		pipelines := strings.Split(pipelineFiles, ",")
		payload, err := wc.getWorkspaceRunPayload(pipelines, wc.project)
		if err != nil {
			return err
		}
		log.Info("Performing validation on pipeline resources")
		err = serviceManager.ValidateWorkspace(payload)
		if err != nil {
			return err
		}
		log.Info(coreutils.PrintTitle("Pipeline resources validation completed successfully"))
	case Sync:
		err := serviceManager.WorkspaceSync(wc.project)
		if err != nil {
			return err
		}
		log.Info("Triggered pipelines sync successfully")
	case SyncStatus:
		response, err := serviceManager.WorkspacePollSyncStatus()
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(response, "-->", "  ")
		if err != nil {
			return err
		}
		log.Info("Workspace sync status : \n", string(data))
	case RunStatus:
		pipelinesBranch, err := serviceManager.WorkspacePipelines()
		if err != nil {
			return err
		}
		pipelineNames := make([]string, len(pipelinesBranch))
		for pipName, _ := range pipelinesBranch {
			pipelineNames = append(pipelineNames, pipName)
		}
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
			err = wc.getStepStatus(runId, serviceManager)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return nil
}

// getWorkspaceRunPayload prepares request body for workspace validation
func (wc *WorkspaceCommand) getWorkspaceRunPayload(resources []string, project string) ([]byte, error) {
	var pipelineDefinitions []PipelineDefinition
	for _, pathToFile := range resources {
		fileContent, fileInfo, err := getFileContentAndBaseName(pathToFile)
		if err != nil {
			return nil, err
		}
		pipeDefinition := PipelineDefinition{
			FileName: fileInfo.Name(),
			Content:  string(fileContent),
			YmlType:  "pipelines",
		}
		pipelineDefinitions = append(pipelineDefinitions, pipeDefinition)
	}
	if wc.values != "" {
		fileContent, fileInfo, err := getFileContentAndBaseName(wc.values)
		if err != nil {
			return nil, err
		}
		pipeDefinition := PipelineDefinition{
			FileName: fileInfo.Name(),
			Content:  string(fileContent),
			YmlType:  "pipelines",
		}
		pipelineDefinitions = append(pipelineDefinitions, pipeDefinition)
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

func getFileContentAndBaseName(pathToFile string) ([]byte, os.FileInfo, error) {
	fileContent, err := os.ReadFile(pathToFile)
	if err != nil {
		return nil, nil, err
	}
	fileInfo, err := os.Stat(pathToFile)
	if err != nil {
		return nil, nil, err
	}
	return fileContent, fileInfo, nil
}

// getStepStatus for the given pipeline run fetch associated steps
// and print status in table format
func (wc *WorkspaceCommand) getStepStatus(runId services.PipelinesRunID, serviceManager *pipelines.PipelinesServicesManager) error {
	log.Info("Fetching step status for pipeline ", runId.Name)
	for {
		time.Sleep(5 * time.Second)
		stopCapturingStepStatus := true
		log.Debug("Fetching step status for run id ", runId.LatestRunID)
		stepStatus, err := serviceManager.WorkspaceStepStatus(runId.LatestRunID)
		if err != nil {
			return err
		}
		stepTable := make([]StepStatus, 0)
		err = json.Unmarshal(stepStatus, &stepTable)
		if err != nil {
			return err
		}
		// Cloning to preserve original response when deletes are performed
		endState := slices.Clone(stepTable)
		for i := 0; i < len(stepTable); i++ {
			stepTable[i].StatusString = string(status.GetPipelineStatus(stepTable[i].StatusCode))
			endState[i].StatusString = string(status.GetPipelineStatus(stepTable[i].StatusCode))
			stopCapturingStepStatus = stopCapturingStepStatus && isStepCompleted(status.PipelineStatus(stepTable[i].StatusString))
			log.Debug(stepTable[i].Name, " step status ", stepTable[i].StatusString)
			/*if !slices.Contains(status.GetWaitingForRunAndRunningSteps(), status.PipelineStatus(stepTable[i].StatusString)) {
				stepTable = slices.Delete(stepTable, i, i+1)
				i--
			}*/
			switch stepTable[i].StatusCode {
			case 4002, 4003, 4004, 4008:
				log.Info("step " + stepTable[i].Name + " completed with status " + stepTable[i].StatusString)
			}

		}
		/*err = coreutils.PrintTable(stepTable, coreutils.PrintTitle(runId.Name+" Step Status"), "All steps reached end state", true)
		if err != nil {
			return err
		}*/
		if !stopCapturingStepStatus {
			// Keep polling for steps status until all steps are processed
			continue
		}
		fmt.Printf("%+v \n", stepTable)
		err = coreutils.PrintTable(endState, coreutils.PrintTitle(runId.Name+" Step Status"), "No Pipeline steps available", true)
		if err != nil {
			return err
		}
		// Get Step Logs and print
		for i := 0; i < len(endState); i++ {
			endState[i].StatusString = string(status.GetPipelineStatus(endState[i].StatusCode))
			log.Output(coreutils.PrintTitle("Logs for step: " + endState[i].Name))
			err := wc.getPipelineStepLogs(strconv.Itoa(endState[i].Id), serviceManager)
			if err != nil {
				return err
			}
			err = wc.getPipelineStepletLogs(strconv.Itoa(endState[i].Id), serviceManager)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

// getPipelineStepLogs invokes to fetch pipeline step logs for the given step ID
func (wc *WorkspaceCommand) getPipelineStepLogs(stepID string, serviceManager *pipelines.PipelinesServicesManager) error {
	consoles, err := serviceManager.GetStepConsoles(stepID)
	if err != nil {
		return err
	}
	for _, v := range consoles {
		for _, console := range v {
			if console.Message != "" && console.IsSuccess != nil {
				log.Output(time.UnixMicro(console.Timestamp).Format("15:04:05.00000"), "  ", console.Message, " ", *console.IsSuccess)
			}
		}
	}
	return nil
}

func (wc *WorkspaceCommand) getPipelineStepletLogs(stepID string, serviceManager *pipelines.PipelinesServicesManager) error {
	consoles, err := serviceManager.GetStepletConsoles(stepID)
	if err != nil {
		return err
	}
	for _, v := range consoles {
		for _, console := range v {
			if console.Message != "" && console.IsSuccess != nil {
				log.Output(time.UnixMicro(console.Timestamp).Format("15:04:05.00000"), "  ", console.Message, " ", *console.IsSuccess)
			}
		}
	}
	return nil
}

func isStepCompleted(stepStatus status.PipelineStatus) bool {
	return slices.Contains(status.GetRunCompletedStatusList(), stepStatus)
}

// ListWorkspaces retrieves all workspaces
func (wc *WorkspaceCommand) ListWorkspaces() error {
	serviceManager, err := manager.CreateServiceManager(wc.serverDetails)
	if err != nil {
		return err
	}
	workspaces, err := serviceManager.GetWorkspace()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(workspaces, "-->", "  ")
	if err != nil {
		return err
	}
	log.Output(coreutils.PrintTitle("Workspaces List:"))
	log.Output(string(data))
	return nil
}

// DeleteWorkspace retrieves all workspaces
func (wc *WorkspaceCommand) DeleteWorkspace() error {
	serviceManager, err := manager.CreateServiceManager(wc.serverDetails)
	if err != nil {
		return err
	}
	log.Info("Project name received is ", wc.project)
	err = serviceManager.DeleteWorkspace(wc.project)
	if err != nil {
		return err
	}
	log.Info("Deleted workspace for ", wc.project)
	return nil
}

func (wc *WorkspaceCommand) WorkspaceLastRunStatus() error {
	return wc.WorkspaceActions(RunStatus)
}

func (wc *WorkspaceCommand) WorkspaceLastSyncStatus() error {
	return wc.WorkspaceActions(SyncStatus)
}

func (wc *WorkspaceCommand) WorkspaceSync() error {
	err := wc.WorkspaceActions(Sync)
	if err != nil {
		return err
	}
	serviceManager, err := manager.CreateServiceManager(wc.serverDetails)
	if err != nil {
		return err
	}
	err = wc.pollSyncStatusAndTriggerRun(serviceManager)
	return err
}
