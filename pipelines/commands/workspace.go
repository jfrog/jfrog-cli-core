package commands

import (
	"bytes"
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/status"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
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
}

// create Github integration
// 1. Workspace provided resource file
// 2. Poll for sync status until sync is completed
// 3. Get workspace pipelines
// 4. Trigger all pipelines
// 5. Get workspace run ids
// 6. Get pipeline run status and steps status

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
		//log.Output(string(runStat))
		log.Info("fetching step status for run id ", runId.LatestRunID)
		stepstat, stepErr := serviceManager.WorkspaceStepStatus(runId.LatestRunID)
		if stepErr != nil {
			return "", stepErr
		}
		log.Output(PrettyString(string(stepstat)))
		stepTable := make([]StepStatus, 0)
		err := json.Unmarshal(stepstat, &stepTable)
		if err != nil {
			return "", err
		}
		for i, step := range stepTable {
			stepTable[i].StatusString = status.GetPipelineStatus(step.StatusCode)
		}
		log.Output()
		log.Output()
		log.Output(coreutils.PrintBold(coreutils.PrintTitle(runId.Name + " Step Status")))
		log.Output(stepTable)
		err = coreutils.PrintTable(stepTable, "", "No pipelines found", true)
		if err != nil {
			return "", err
		}
	}
	return "", nil
}

func PrettyString(str string) (string, error) {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(str), "", "    "); err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
}
