package generic

import (
	"encoding/json"
	rtUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/access/services"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"strings"
)

const (
	AdminScope        = "applied-permissions/admin"
	GroupsScopePrefix = "applied-permissions/groups:"
)

type AccessTokenCreateCommand struct {
	serverDetails *config.ServerDetails
	username      string
	projectKey    string

	groups     string
	scope      string
	grantAdmin bool

	expiry      *uint
	refreshable bool
	description string

	audience              string
	includeReferenceToken bool

	response *auth.CreateTokenResponseData
}

func NewAccessTokenCreateCommand() *AccessTokenCreateCommand {
	return &AccessTokenCreateCommand{response: new(auth.CreateTokenResponseData)}
}

func (atc *AccessTokenCreateCommand) SetServerDetails(serverDetails *config.ServerDetails) *AccessTokenCreateCommand {
	atc.serverDetails = serverDetails
	return atc
}

func (atc *AccessTokenCreateCommand) SetUsername(username string) *AccessTokenCreateCommand {
	atc.username = username
	return atc
}

func (atc *AccessTokenCreateCommand) SetProjectKey(projectKey string) *AccessTokenCreateCommand {
	atc.projectKey = projectKey
	return atc
}

func (atc *AccessTokenCreateCommand) SetGroups(groups string) *AccessTokenCreateCommand {
	atc.groups = groups
	return atc
}

func (atc *AccessTokenCreateCommand) SetScope(scope string) *AccessTokenCreateCommand {
	atc.scope = scope
	return atc
}

func (atc *AccessTokenCreateCommand) SetGrantAdmin(grantAdmin bool) *AccessTokenCreateCommand {
	atc.grantAdmin = grantAdmin
	return atc
}

func (atc *AccessTokenCreateCommand) SetExpiry(expiry *uint) *AccessTokenCreateCommand {
	atc.expiry = expiry
	return atc
}

func (atc *AccessTokenCreateCommand) SetRefreshable(refreshable bool) *AccessTokenCreateCommand {
	atc.refreshable = refreshable
	return atc
}

func (atc *AccessTokenCreateCommand) SetDescription(description string) *AccessTokenCreateCommand {
	atc.description = description
	return atc
}

func (atc *AccessTokenCreateCommand) SetAudience(audience string) *AccessTokenCreateCommand {
	atc.audience = audience
	return atc
}

func (atc *AccessTokenCreateCommand) SetIncludeReferenceToken(includeReferenceToken bool) *AccessTokenCreateCommand {
	atc.includeReferenceToken = includeReferenceToken
	return atc
}

func (atc *AccessTokenCreateCommand) Response() ([]byte, error) {
	content, err := json.Marshal(*atc.response)
	return content, errorutils.CheckError(err)
}

func (atc *AccessTokenCreateCommand) ServerDetails() (*config.ServerDetails, error) {
	return atc.serverDetails, nil
}

func (atc *AccessTokenCreateCommand) CommandName() string {
	return "access_token_create"
}

func (atc *AccessTokenCreateCommand) Run() error {
	servicesManager, err := rtUtils.CreateAccessServiceManager(atc.serverDetails, false)
	if err != nil {
		return err
	}

	*atc.response, err = servicesManager.CreateAccessToken(atc.getTokenParams())
	return err
}

func (atc *AccessTokenCreateCommand) getTokenParams() services.CreateTokenParams {
	tokenParams := services.NewCreateTokenParams()

	tokenParams.Username = strings.ToLower(atc.username)
	tokenParams.ProjectKey = atc.projectKey

	// If a specific scope was requested, apply it. Otherwise, the scope remains empty and the default user scope will be applied.
	switch {
	case atc.grantAdmin:
		tokenParams.Scope = AdminScope
	case atc.groups != "":
		tokenParams.Scope = GroupsScopePrefix + atc.groups
	case atc.scope != "":
		tokenParams.Scope = atc.scope
	}

	tokenParams.ExpiresIn = atc.expiry
	tokenParams.Refreshable = &atc.refreshable
	tokenParams.Description = atc.description
	tokenParams.Audience = atc.audience
	tokenParams.IncludeReferenceToken = &atc.includeReferenceToken
	return tokenParams
}
