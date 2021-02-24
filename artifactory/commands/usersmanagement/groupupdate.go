package usersmanagement

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
)

type GroupUpdateCommand struct {
	serverDetails *config.ServerDetails
	name          string
	users         []string
}

func NewGroupUpdateCommand() *GroupUpdateCommand {
	return &GroupUpdateCommand{}
}

func (guc *GroupUpdateCommand) ServerDetails() (*config.ServerDetails, error) {
	return guc.serverDetails, nil
}

func (guc *GroupUpdateCommand) SetServerDetails(serverDetails *config.ServerDetails) *GroupUpdateCommand {
	guc.serverDetails = serverDetails
	return guc
}

func (guc *GroupUpdateCommand) SetName(groupName string) *GroupUpdateCommand {
	guc.name = groupName
	return guc
}

func (guc *GroupUpdateCommand) Name() string {
	return guc.name
}

func (guc *GroupUpdateCommand) SetUsers(users []string) *GroupUpdateCommand {
	guc.users = users
	return guc
}

func (guc *GroupUpdateCommand) Users() []string {
	return guc.users
}

func (guc *GroupUpdateCommand) CommandName() string {
	return "rt_group_update"
}

func (guc *GroupUpdateCommand) Run() error {
	servicesManager, err := utils.CreateServiceManager(guc.serverDetails, false)
	if err != nil {
		return err
	}
	group := new(services.Group)
	group.Name = guc.Name()
	group.UsersNames = guc.Users()
	params := new(services.GroupParams)
	params.GroupDetails = *group
	return servicesManager.UpdateGroup(*params)
}
