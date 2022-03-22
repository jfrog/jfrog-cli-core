package invite

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

const (
	cliSourceName = "cli"
)

type InviteCommand struct {
	invitedEmail  string
	serverDetails *config.ServerDetails
}

func (ic *InviteCommand) SetInvitedEmail(invitedEmail string) *InviteCommand {
	ic.invitedEmail = invitedEmail
	return ic
}

func (ic *InviteCommand) SetServerDetails(serverDetails *config.ServerDetails) *InviteCommand {
	ic.serverDetails = serverDetails
	return ic
}

func (ic *InviteCommand) ServerDetails() (*config.ServerDetails, error) {
	return ic.serverDetails, nil
}

func (ic *InviteCommand) CommandName() string {
	return "invite"
}

func NewInviteCommand() *InviteCommand {
	return &InviteCommand{}
}

func (ic *InviteCommand) Run() (err error) {
	servicesManager, err := utils.CreateServiceManager(ic.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}
	userDetails := ic.createNewInvitedUser()

	log.Info(fmt.Sprintf("Sending invitation email to: %s...", userDetails.Name))
	params := new(services.UserParams)
	params.UserDetails = *userDetails
	params.ReplaceIfExists = false
	err = servicesManager.CreateUser(*params)
	if err != nil {
		if strings.HasSuffix(err.Error(), "already exists") {
			log.Info(fmt.Sprintf("Re-sending invitation email to: %s...", userDetails.Name))
			err = servicesManager.InviteUser(params.UserDetails.Email)
			if err != nil {
				return
			}
		}
	}
	return
}

func (ic *InviteCommand) createNewInvitedUser() *services.User {
	trueValue := true
	userDetails := services.User{}
	userDetails.Email = ic.invitedEmail
	userDetails.Name = ic.invitedEmail
	userDetails.Password = "Password1!"
	userDetails.Admin = &trueValue
	userDetails.ShouldInvite = &trueValue
	userDetails.Source = cliSourceName
	return &userDetails
}

type InvitedUser struct {
	Email string `json:"invitedEmail,omitempty" csv:"invitedEmail,omitempty"`
}
