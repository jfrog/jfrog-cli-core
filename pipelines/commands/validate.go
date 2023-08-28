package commands

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strings"
)

type ValidateCommand struct {
	serverDetails *config.ServerDetails
	files         string
	directory     string
}

type RespBody struct {
	IsValid bool               `json:"isValid"`
	Errors  []ValidationErrors `json:"errors"`
}

type ValidationErrors struct {
	Text       string
	LineNumber int
}

func NewValidateCommand() *ValidateCommand {
	return &ValidateCommand{}
}

func (vc *ValidateCommand) ServerDetails() (*config.ServerDetails, error) {
	return vc.serverDetails, nil
}

func (vc *ValidateCommand) SetServerDetails(serverDetails *config.ServerDetails) *ValidateCommand {
	vc.serverDetails = serverDetails
	return vc
}

func (vc *ValidateCommand) SetPipeResourceFiles(f string) *ValidateCommand {
	vc.files = f
	return vc
}

func (vc *ValidateCommand) SetDirectoryPath(d string) *ValidateCommand {
	vc.directory = d
	return vc
}

func (vc *ValidateCommand) CommandName() string {
	return "pl_validate"
}

func (vc *ValidateCommand) Run() error {
	serviceManager, err := manager.CreateServiceManager(vc.serverDetails)
	if err != nil {
		return err
	}
	payload, err := vc.preparePayload()
	if err != nil {
		return err
	}
	return serviceManager.ValidatePipelineSources(payload)
}

func (vc *ValidateCommand) preparePayload() ([]byte, error) {
	log.Info("Pipeline resources found for processing ")
	files := strings.Split(vc.files, ",")
	if len(vc.directory) > 0 {
		filesFromDir, err := utils.GetAllFilesFromDirectory(vc.directory)
		if err != nil && len(vc.files) == 0 {
			return []byte{}, err
		}
		log.Info("Proceeding with validation on ", vc.files)
		files = filesFromDir
	}
	pipelineDefinitions, err := structureFileContentAsPipelineDefinition(files, "")
	if err != nil {
		return []byte{}, err
	}
	validationFiles := ValidationFiles{Files: pipelineDefinitions}
	return json.Marshal(validationFiles)
}
