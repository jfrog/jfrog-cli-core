package usersmanagement

import (
	"fmt"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type UsersCreateCommand struct {
	serverDetails   *config.ServerDetails
	users           []services.User
	usersGroups     *[]string
	replaceIfExists bool
}

func NewUsersCreateCommand() *UsersCreateCommand {
	return &UsersCreateCommand{}
}

func (ucc *UsersCreateCommand) SetUsers(users []services.User) *UsersCreateCommand {
	ucc.users = users
	return ucc
}

func (ucc *UsersCreateCommand) Users() []services.User {
	return ucc.users
}

func (ucc *UsersCreateCommand) ServerDetails() (*config.ServerDetails, error) {
	return ucc.serverDetails, nil
}

func (ucc *UsersCreateCommand) SetServerDetails(serverDetails *config.ServerDetails) *UsersCreateCommand {
	ucc.serverDetails = serverDetails
	return ucc
}

func (ucc *UsersCreateCommand) SetUsersGroups(usersGroups *[]string) *UsersCreateCommand {
	ucc.usersGroups = usersGroups
	return ucc
}

func (ucc *UsersCreateCommand) SetReplaceIfExists(replaceIfExists bool) *UsersCreateCommand {
	ucc.replaceIfExists = replaceIfExists
	return ucc
}

func (ucc *UsersCreateCommand) ReplaceIfExists() bool {
	return ucc.replaceIfExists
}

func (ucc *UsersCreateCommand) CommandName() string {
	return "rt_users_create"
}

func (ucc *UsersCreateCommand) Run() error {
	servicesManager, err := utils.CreateServiceManager(ucc.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}

	for _, user := range ucc.users {
		log.Info(fmt.Sprintf("Creating user %s...", user.Name))
		user.Groups = ucc.usersGroups
		params := new(services.UserParams)
		params.UserDetails = user
		params.ReplaceIfExists = ucc.ReplaceIfExists()
		err = servicesManager.CreateUser(*params)
		if err != nil {
			break
		}
	}
	return err
}
