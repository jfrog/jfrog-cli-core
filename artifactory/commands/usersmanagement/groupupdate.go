package usersmanagement

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
)

type GroupUpdateCommand struct {
	rtDetails *config.ArtifactoryDetails
	name      string
	users     []string
}

func NewGroupUpdateCommand() *GroupUpdateCommand {
	return &GroupUpdateCommand{}
}

func (guc *GroupUpdateCommand) RtDetails() (*config.ArtifactoryDetails, error) {
	return guc.rtDetails, nil
}

func (guc *GroupUpdateCommand) SetRtDetails(rtDetails *config.ArtifactoryDetails) *GroupUpdateCommand {
	guc.rtDetails = rtDetails
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
	servicesManager, err := utils.CreateServiceManager(guc.rtDetails, false)
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
