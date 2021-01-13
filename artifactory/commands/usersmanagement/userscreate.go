package usersmanagement

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
)

type UsersCreateCommand struct {
	rtDetails         *config.ArtifactoryDetails
	users             []services.User
	usersGroups       []string
	replaceExistUsers bool
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

func (ucc *UsersCreateCommand) RtDetails() (*config.ArtifactoryDetails, error) {
	return ucc.rtDetails, nil
}

func (ucc *UsersCreateCommand) SetRtDetails(rtDetails *config.ArtifactoryDetails) *UsersCreateCommand {
	ucc.rtDetails = rtDetails
	return ucc
}

func (ucc *UsersCreateCommand) SetUsersGroups(usersGroups []string) *UsersCreateCommand {
	ucc.usersGroups = usersGroups
	return ucc
}

func (ucc *UsersCreateCommand) UsersGroups() []string {
	return ucc.usersGroups
}

func (ucc *UsersCreateCommand) SetReplaceExistUsersFlag(replaceExistUsers bool) *UsersCreateCommand {
	ucc.replaceExistUsers = replaceExistUsers
	return ucc
}

func (ucc *UsersCreateCommand) ReplaceExistUsersFlag() bool {
	return ucc.replaceExistUsers
}

func (ucc *UsersCreateCommand) CommandName() string {
	return "rt_users-create"
}

func (ucc *UsersCreateCommand) Run() error {
	servicesManager, err := utils.CreateServiceManager(ucc.rtDetails, false)
	if err != nil {
		return err
	}

	for _, user := range ucc.users {
		user.Groups = ucc.usersGroups
		err = servicesManager.CreateUser(user, ucc.ReplaceExistUsersFlag())
		if err != nil {
			break
		}
	}
	return err
}
