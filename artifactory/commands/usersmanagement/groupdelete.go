package usersmanagement

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type GroupDeleteCommand struct {
	rtDetails *config.ServerDetails
	name      string
}

func NewGroupDeleteCommand() *GroupDeleteCommand {
	return &GroupDeleteCommand{}
}

func (gdc *GroupDeleteCommand) ServerDetails() (*config.ServerDetails, error) {
	return gdc.rtDetails, nil
}

func (gdc *GroupDeleteCommand) SetServerDetails(serverDetails *config.ServerDetails) *GroupDeleteCommand {
	gdc.rtDetails = serverDetails
	return gdc
}

func (gdc *GroupDeleteCommand) SetName(groupName string) *GroupDeleteCommand {
	gdc.name = groupName
	return gdc
}

func (gdc *GroupDeleteCommand) Name() string {
	return gdc.name
}

func (gdc *GroupDeleteCommand) CommandName() string {
	return "rt_group_delete"
}

func (gdc *GroupDeleteCommand) Run() error {
	servicesManager, err := utils.CreateServiceManager(gdc.rtDetails, -1, false)
	if err != nil {
		return err
	}
	return servicesManager.DeleteGroup(gdc.Name())
}
