package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type TriggerCommand struct {
	serverDetails *config.ServerDetails
	branch        string
	pipelineName  string
	output        string
}

func NewTriggerCommand() *TriggerCommand {
	return &TriggerCommand{}
}

func (tc *TriggerCommand) ServerDetails() (*config.ServerDetails, error) {
	return tc.serverDetails, nil
}

func (tc *TriggerCommand) SetServerDetails(serverDetails *config.ServerDetails) *TriggerCommand {
	tc.serverDetails = serverDetails
	return tc
}

func (tc *TriggerCommand) CommandName() string {
	return "trigger"
}

func (tc *TriggerCommand) Run() (string, error) {
	var err error
	serviceManager, err := manager.CreateServiceManager(tc.serverDetails)
	if err != nil {
		return "", err
	}
	return serviceManager.TriggerPipelineRun(tc.branch, tc.pipelineName)
}
