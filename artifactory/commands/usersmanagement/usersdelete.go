package usersmanagement

import (
	"fmt"

	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type UsersDeleteCommand struct {
	serverDetails *config.ServerDetails
	usersNames    []string
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

func (udc *UsersDeleteCommand) ServerDetails() (*config.ServerDetails, error) {
	return udc.serverDetails, nil
}

func (udc *UsersDeleteCommand) SetServerDetails(serverDetails *config.ServerDetails) *UsersDeleteCommand {
	udc.serverDetails = serverDetails
	return udc
}

func (udc *UsersDeleteCommand) CommandName() string {
	return "rt_users_delete"
}

func (udc *UsersDeleteCommand) Run() error {
	servicesManager, err := utils.CreateServiceManager(udc.serverDetails, -1, false)
	if err != nil {
		return err
	}

	for _, userName := range udc.usersNames {
		log.Info(fmt.Sprintf("Deleting user %s...", userName))
		err = servicesManager.DeleteUser(userName)
		if err != nil {
			break
		}
	}
	return err
}
