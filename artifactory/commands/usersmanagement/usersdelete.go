package usersmanagement

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
)

type UsersDeleteCommand struct {
	rtDetails  *config.ArtifactoryDetails
	usersNames []string
}

func NewUsersDeleteCommand() *UsersDeleteCommand {
	return &UsersDeleteCommand{}
}

func (udc *UsersDeleteCommand) SetUsers(usersNames []string) *UsersDeleteCommand {
	udc.usersNames = usersNames
	return udc
}

func (udc *UsersDeleteCommand) UsersNames() []string {
	return udc.usersNames
}

func (udc *UsersDeleteCommand) RtDetails() (*config.ArtifactoryDetails, error) {
	return udc.rtDetails, nil
}

func (udc *UsersDeleteCommand) SetRtDetails(rtDetails *config.ArtifactoryDetails) *UsersDeleteCommand {
	udc.rtDetails = rtDetails
	return udc
}

func (udc *UsersDeleteCommand) CommandName() string {
	return "rt_users_delete"
}

func (udc *UsersDeleteCommand) Run() error {
	servicesManager, err := utils.CreateServiceManager(udc.rtDetails, false)
	if err != nil {
		return err
	}

	for _, userName := range udc.usersNames {
		err = servicesManager.DeleteUser(userName)
		if err != nil {
			break
		}
	}
	return err
}
