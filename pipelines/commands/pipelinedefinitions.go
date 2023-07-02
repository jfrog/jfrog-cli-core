package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strings"
)

type PipelineDefinition struct {
	FileName string `json:"fileName,omitempty"`
	Content  string `json:"content,omitempty"`
	YmlType  string `json:"ymlType,omitempty"`
}

type ValidationFiles struct {
	Files []PipelineDefinition `json:"files,omitempty"`
}

func structureFileContentAsPipelineDefinition(allPipelineFiles []string, values string) ([]PipelineDefinition, error) {
	var pipelineDefinitions []PipelineDefinition
	for _, pathToFile := range allPipelineFiles {
		log.Info("Attaching pipelines definition file: ", pathToFile)
		fileContent, fileInfo, err := utils.GetFileContentAndBaseName(pathToFile)
		if err != nil {
			return nil, err
		}
		ymlType := "pipelines"
		if strings.EqualFold(fileInfo.Name(), "values.yml") {
			ymlType = "values"
		} else if strings.Contains(fileInfo.Name(), "resources") {
			ymlType = "resources"
		}
		if len(fileContent) == 0 {
			continue
		}
		pipeDefinition := PipelineDefinition{
			FileName: fileInfo.Name(),
			Content:  string(fileContent),
			YmlType:  ymlType,
		}
		pipelineDefinitions = append(pipelineDefinitions, pipeDefinition)
	}
	if values != "" {
		fileContent, fileInfo, err := utils.GetFileContentAndBaseName(values)
		if err != nil {
			return nil, err
		}
		pipeDefinition := PipelineDefinition{
			FileName: fileInfo.Name(),
			Content:  string(fileContent),
			YmlType:  "values",
		}
		pipelineDefinitions = append(pipelineDefinitions, pipeDefinition)
	}
	log.Info("Total number of pipeline definition files eligible for validation: ", len(pipelineDefinitions))
	log.Debug("Pipelines file content: ", pipelineDefinitions)
	return pipelineDefinitions, nil
}
