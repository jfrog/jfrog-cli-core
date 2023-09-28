package invite

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-client-go/access"
	accessservices "github.com/jfrog/jfrog-client-go/access/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

const InviteCliSourceName = "cli"

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
	// Inviting the new user - send a 'CreateUser' request to artifactory with the "shouldInvite=true" parameter.
	err = servicesManager.CreateUser(*params)
	if err != nil {
		if strings.HasSuffix(err.Error(), "already exists") {
			log.Debug(fmt.Sprintf("Re-sending invitation email to: %s...", userDetails.Name))
			var accessManager *access.AccessServicesManager
			accessManager, err = utils.CreateAccessServiceManager(ic.serverDetails, false)
			if err != nil {
				return
			}
			// Re-inviting user - send an "Invite" request to access.
			err = accessManager.InviteUser(params.UserDetails.Email, InviteCliSourceName)
			if err != nil {
				return
			}
		}
	}
	return
}

func (ic *InviteCommand) createNewInvitedUser() *services.User {
	userDetails := services.User{}
	// Parameters "name" and "email" should both be with the email value for internal reasons in access.
	userDetails.Email = ic.invitedEmail
	userDetails.Name = ic.invitedEmail
	// Random valid password - information won't be used in access.
	userDetails.Password = "Password1!"
	userDetails.Admin = clientutils.Pointer(true)
	userDetails.ShouldInvite = clientutils.Pointer(true)
	userDetails.Source = accessservices.InviteCliSourceName

	userDetails.ProfileUpdatable = clientutils.Pointer(true)
	userDetails.DisableUIAccess = clientutils.Pointer(false)
	userDetails.InternalPasswordDisabled = clientutils.Pointer(false)
	return &userDetails
}
