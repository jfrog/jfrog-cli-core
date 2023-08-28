package utils

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/status"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/pipelines"
	"github.com/jfrog/jfrog-client-go/pipelines/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/slices"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type StepStatus struct {
	Name             string `col-name:"Name"`
	StatusCode       int    `col-name:"StatusCode"`
	TriggeredAt      string `col-name:"Triggered At"`
	ExternalBuildUrl string
	StatusString     string `col-name:"Status"`
	Id               int
}

// GetStepStatus for the given pipeline run fetch associated steps
// and print status in table format
func GetStepStatus(runId services.PipelinesRunID, serviceManager *pipelines.PipelinesServicesManager) error {
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
			switch stepTable[i].StatusCode {
			case 4002, 4003, 4004, 4008:
				log.Info("step " + stepTable[i].Name + " completed with status " + stepTable[i].StatusString)
			}

		}

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
			err := getPipelineStepLogs(strconv.Itoa(endState[i].Id), serviceManager)
			if err != nil {
				return err
			}
			err = getPipelineStepletLogs(strconv.Itoa(endState[i].Id), serviceManager)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

// getPipelineStepLogs invokes to fetch pipeline step logs for the given step ID
func getPipelineStepLogs(stepID string, serviceManager *pipelines.PipelinesServicesManager) error {
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

func getPipelineStepletLogs(stepID string, serviceManager *pipelines.PipelinesServicesManager) error {
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

func GetFileContentAndBaseName(pathToFile string) ([]byte, os.FileInfo, error) {
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

func GetAllFilesFromDirectory(relativePathToPipelineDefinitions string) ([]string, error) {
	var files []string
	if len(relativePathToPipelineDefinitions) > 0 {
		pwd, err := os.Getwd()
		if err != nil {
			return files, err
		}
		log.Info("Running in ", pwd, " reading files from ", pwd+"/"+relativePathToPipelineDefinitions)
		err = filepath.Walk(pwd+"/"+relativePathToPipelineDefinitions, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && (strings.EqualFold(filepath.Ext(path), ".yml") || strings.EqualFold(filepath.Ext(path), ".yaml")) {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return files, err
		}
	}
	log.Debug("files collected from directory ", files)
	return files, nil
}
