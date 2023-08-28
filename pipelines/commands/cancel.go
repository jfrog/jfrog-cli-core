package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type CancelCommand struct {
	serverDetails      *config.ServerDetails
	branch             string
	repositoryFullName string
}

func NewCancelCommand() *CancelCommand {
	return &CancelCommand{}
}

func (cc *CancelCommand) ServerDetails() (*config.ServerDetails, error) {
	return cc.serverDetails, nil
}

func (cc *CancelCommand) SetServerDetails(serverDetails *config.ServerDetails) *CancelCommand {
	cc.serverDetails = serverDetails
	return cc
}

func (cc *CancelCommand) CommandName() string {
	return "cancel"
}

func (cc *CancelCommand) SetBranch(br string) *CancelCommand {
	cc.branch = br
	return cc
}

func (cc *CancelCommand) SetRepositoryFullName(rfn string) *CancelCommand {
	cc.repositoryFullName = rfn
	return cc
}

func (cc *CancelCommand) Run() error {
	_, err := manager.CreateServiceManager(cc.serverDetails)
	if err != nil {
		return err
	}

	return nil
}
