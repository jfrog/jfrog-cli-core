package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type TriggerCommand struct {
	serverDetails *config.ServerDetails
	branch        string
	pipelineName  string
	isMultiBranch bool
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

func (tc *TriggerCommand) SetBranch(br string) *TriggerCommand {
	tc.branch = br
	return tc
}

func (tc *TriggerCommand) SetPipelineName(pl string) *TriggerCommand {
	tc.pipelineName = pl
	return tc
}

func (tc *TriggerCommand) SetMultiBranch(multiBranch bool) *TriggerCommand {
	tc.isMultiBranch = multiBranch
	return tc
}

func (tc *TriggerCommand) CommandName() string {
	return "pl_trigger"
}

func (tc *TriggerCommand) Run() error {
	serviceManager, err := manager.CreateServiceManager(tc.serverDetails)
	if err != nil {
		return err
	}
	return serviceManager.TriggerPipelineRun(tc.branch, tc.pipelineName, tc.isMultiBranch)
}
