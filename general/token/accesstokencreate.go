package token

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

	scope      string
	groups     string
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
	return "jf_access_token_create"
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
	tokenParams := services.CreateTokenParams{}

	tokenParams.Username = strings.ToLower(atc.username)
	tokenParams.ProjectKey = atc.projectKey
	tokenParams.Scope = atc.getScope()
	tokenParams.ExpiresIn = atc.expiry
	tokenParams.Refreshable = &atc.refreshable
	tokenParams.Description = atc.description
	tokenParams.Audience = atc.audience
	tokenParams.IncludeReferenceToken = &atc.includeReferenceToken
	return tokenParams
}

// If an explicit scope was provided, apply it.
// Otherwise, if admin or groups scopes were requested, construct scope from them (space separated).
// If no scopes were requested, leave scope empty to provide the default user scope.
func (atc *AccessTokenCreateCommand) getScope() string {
	if atc.scope != "" {
		return atc.scope
	}

	var scopes []string
	if atc.groups != "" {
		scopes = append(scopes, GroupsScopePrefix+atc.groups)
	}

	if atc.grantAdmin {
		scopes = append(scopes, AdminScope)
	}
	return strings.Join(scopes, " ")
}
