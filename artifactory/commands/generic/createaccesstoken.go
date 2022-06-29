package generic

import (
	"encoding/json"
	"github.com/jfrog/jfrog-client-go/auth"
	"strings"

	rtUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

const (
	GroupsPrefix          = "member-of-groups:"
	UserScopedNotation    = "*"
	AdminPrivilegesSuffix = ":admin"
)

type AccessTokenCreateCommand struct {
	serverDetails *config.ServerDetails
	refreshable   bool
	expiry        int
	userName      string
	audience      string
	groups        string
	grantAdmin    bool
	response      *auth.CreateTokenResponseData
}

func NewAccessTokenCreateCommand() *AccessTokenCreateCommand {
	return &AccessTokenCreateCommand{response: new(auth.CreateTokenResponseData)}
}

func (atcc *AccessTokenCreateCommand) SetServerDetails(serverDetails *config.ServerDetails) *AccessTokenCreateCommand {
	atcc.serverDetails = serverDetails
	return atcc
}

func (atcc *AccessTokenCreateCommand) SetRefreshable(refreshable bool) *AccessTokenCreateCommand {
	atcc.refreshable = refreshable
	return atcc
}

func (atcc *AccessTokenCreateCommand) SetExpiry(expiry int) *AccessTokenCreateCommand {
	atcc.expiry = expiry
	return atcc
}

func (atcc *AccessTokenCreateCommand) SetUserName(userName string) *AccessTokenCreateCommand {
	atcc.userName = userName
	return atcc
}

func (atcc *AccessTokenCreateCommand) SetAudience(audience string) *AccessTokenCreateCommand {
	atcc.audience = audience
	return atcc
}

func (atcc *AccessTokenCreateCommand) SetGrantAdmin(grantAdmin bool) *AccessTokenCreateCommand {
	atcc.grantAdmin = grantAdmin
	return atcc
}

func (atcc *AccessTokenCreateCommand) SetGroups(groups string) *AccessTokenCreateCommand {
	atcc.groups = groups
	return atcc
}

func (atcc *AccessTokenCreateCommand) Response() ([]byte, error) {
	content, err := json.Marshal(*atcc.response)
	return content, errorutils.CheckError(err)
}

func (atcc *AccessTokenCreateCommand) ServerDetails() (*config.ServerDetails, error) {
	return atcc.serverDetails, nil
}

func (atcc *AccessTokenCreateCommand) CommandName() string {
	return "rt_create_access_token"
}

func (atcc *AccessTokenCreateCommand) Run() error {
	servicesManager, err := rtUtils.CreateServiceManager(atcc.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}
	tokenParams, err := atcc.getTokenParams()
	if err != nil {
		return err
	}

	*atcc.response, err = servicesManager.CreateToken(tokenParams)
	return err
}

func (atcc *AccessTokenCreateCommand) getTokenParams() (tokenParams services.CreateTokenParams, err error) {
	tokenParams = services.NewCreateTokenParams()
	tokenParams.ExpiresIn = atcc.expiry
	tokenParams.Refreshable = atcc.refreshable
	tokenParams.Audience = atcc.audience
	// Artifactory expects the username to be lower-cased. In case it is not,
	// Artifactory will still accept a non-lower-cased user, except for token related actions.
	tokenParams.Username = strings.ToLower(atcc.userName)
	// By default, we will create "user-scoped token", unless specific groups or admin-privilege-instance were specified
	if len(atcc.groups) == 0 && !atcc.grantAdmin {
		atcc.groups = UserScopedNotation
	}
	if len(atcc.groups) > 0 {
		tokenParams.Scope = GroupsPrefix + atcc.groups
	}
	if atcc.grantAdmin {
		instanceId, err := getInstanceId(atcc.serverDetails)
		if err != nil {
			return tokenParams, err
		}
		if len(tokenParams.Scope) > 0 {
			tokenParams.Scope += " "
		}
		tokenParams.Scope += instanceId + AdminPrivilegesSuffix
	}

	return
}

func getInstanceId(serverDetails *config.ServerDetails) (string, error) {
	servicesManager, err := rtUtils.CreateServiceManager(serverDetails, -1, 0, false)
	if err != nil {
		return "", err
	}
	return servicesManager.GetServiceId()
}
