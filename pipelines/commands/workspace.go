package commands

import (
	"bytes"
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
	TriggeredAt      string `col-name:"Triggered At"`
	ExternalBuildUrl string `col-name:"Build URL"`
	StatusString     string `col-name:"Status Code"`
	Id               int
}

// create Github integration
// 1. Workspace provided resource file
// 2. Poll for sync status until sync is completed
// 3. Get workspace pipelines
// 4. Trigger all pipelines
// 5. Get workspace run ids
// 6. Get pipeline run status and steps status
// 7. Get Step Logs from console api
// 8. Display error logs in case of error
// 9. Ask Query whether to continue JFrog CLI execution or to exit
// 10. Provide an option to download all the step logs

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

func (ws *WorkspaceCommand) Run() (string, error) {
	serviceManager, err := manager.CreateServiceManager(ws.serverDetails)
	if err != nil {
		return "", err
	}
	vc := NewValidateCommand()
	vc.SetServerDetails(ws.serverDetails)
	vc.SetPipeResourceFiles(ws.pathToFile)
	fileContent, err := vc.ValidateResources()
	if err != nil {
		return "", err
	}
	log.Info("performing validation on pipeline resources")
	valErr := serviceManager.ValidateWorkspace(fileContent)
	if valErr != nil {
		return "", valErr
	}
	log.Info(coreutils.PrintTitle("Pipeline resources validation completed successfully"))

	pipelinesBranch, pipErr := serviceManager.WorkspacePipelines()
	if pipErr != nil {
		return "", pipErr
	}
	log.Info("fetching pipelines")
	for pipName, branch := range pipelinesBranch {
		log.Info(coreutils.PrintTitle("triggering pipeline run for "), pipName)
		trigErr := serviceManager.TriggerPipelineRun(branch, pipName, false)
		if trigErr != nil {
			return "", trigErr
		}
	}

	pipelineNames := make([]string, len(pipelinesBranch))

	i := 0
	for k := range pipelinesBranch {
		pipelineNames[i] = k
		i++
	}
	log.Info("collecting run ids from pipelines defined in workspace")
	pipeRunIDs, wsRunErr := serviceManager.WorkspaceRunIDs(pipelineNames)
	if wsRunErr != nil {
		return "", wsRunErr
	}

	for _, runId := range pipeRunIDs {
		log.Info(coreutils.PrintTitle("fetching run status for run id "), runId.LatestRunID)
		_, runErr := serviceManager.WorkspaceRunStatus(runId.LatestRunID)
		if runErr != nil {
			return "", runErr
		}
		s, err2 := ws.getStepStatus(runId, serviceManager)
		if err2 != nil {
			return s, err2
		}
	}
	return "", nil
}

// getStepStatus for the given pipeline run fetch associated steps
// and print status in table format
func (ws *WorkspaceCommand) getStepStatus(runId services.PipelinesRunID, serviceManager *pipelines.PipelinesServicesManager) (string, error) {

	for {
		stopCapturingStepStatus := true
		log.Info("fetching step status for run id ", runId.LatestRunID)
		stepstat, stepErr := serviceManager.WorkspaceStepStatus(runId.LatestRunID)
		if stepErr != nil {
			return "", stepErr
		}
		stepTable := make([]StepStatus, 0)
		err := json.Unmarshal(stepstat, &stepTable)
		if err != nil {
			return "", err
		}
		//log.Output(PrettyString(string(stepstat)))
		endState := slices.Clone(stepTable) // Cloning to preserve original response when deletes are performed
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
			continue // All steps processing is not completed, keep polling for step status
		}
		err = coreutils.PrintTable(endState, coreutils.PrintTitle(runId.Name+" Step Status"), "No Pipeline steps available", true)
		if err != nil {
			return "", err
		}
		// Get Step Logs and print
		for i := 0; i < len(endState); i++ {
			endState[i].StatusString = status.GetPipelineStatus(endState[i].StatusCode)
			log.Output(coreutils.PrintTitle("Fetching logs for step " + endState[i].Name))
			consoleErr := ws.getPipelineStepLogs(strconv.Itoa(endState[i].Id), serviceManager)
			if consoleErr != nil {
				return "", consoleErr
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
	//rootConsole := consoles["root"]
	for _, v := range consoles {
		for _, console := range v {
			if console.IsShown {
				log.Output(console.CreatedAt, "  ", console.Message)
			}
		}
	}
	return nil
}

func isStepCompleted(stepStatus string) bool {
	return slices.Contains(status.GetRunCompletedStatusList(), stepStatus)
}

func PrettyString(str string) (string, error) {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(str), "", "    "); err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
}
