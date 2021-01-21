package usersmanagement

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
)

type GroupCreateCommand struct {
	rtDetails       *config.ArtifactoryDetails
	name            string
	replaceIfExists bool
}

func NewGroupCreateCommand() *GroupCreateCommand {
	return &GroupCreateCommand{}
}

func (gcc *GroupCreateCommand) RtDetails() (*config.ArtifactoryDetails, error) {
	return gcc.rtDetails, nil
}

func (gcc *GroupCreateCommand) SetRtDetails(rtDetails *config.ArtifactoryDetails) *GroupCreateCommand {
	gcc.rtDetails = rtDetails
	return gcc
}

func (gcc *GroupCreateCommand) SetName(groupName string) *GroupCreateCommand {
	gcc.name = groupName
	return gcc
}

func (gcc *GroupCreateCommand) Name() string {
	return gcc.name
}

func (gcc *GroupCreateCommand) SetReplaceIfExists(replaceIfExists bool) *GroupCreateCommand {
	gcc.replaceIfExists = replaceIfExists
	return gcc
}

func (gcc *GroupCreateCommand) ReplaceIfExists() bool {
	return gcc.replaceIfExists
}

func (gcc *GroupCreateCommand) CommandName() string {
	return "rt_group_create"
}

func (gcc *GroupCreateCommand) Run() error {
	servicesManager, err := utils.CreateServiceManager(gcc.rtDetails, false)
	if err != nil {
		return err
	}
	group := new(services.Group)
	group.Name = gcc.Name()
	params := new(services.GroupParams)
	params.GroupDetails = *group
	params.ReplaceIfExists = gcc.ReplaceIfExists()
	return servicesManager.CreateGroup(*params)
}
