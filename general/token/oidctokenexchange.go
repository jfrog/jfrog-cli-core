package token

import (
	"encoding/json"
	rtUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/access/services"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

const (
	grantType        = "urn:ietf:params:oauth:grant-type:token-exchange"
	subjectTokenType = "urn:ietf:params:oauth:token-type:id_token"
)

type OidcTokenExchangeCommand struct {
	serverDetails   *config.ServerDetails
	providerName    string
	audience        string
	oidcTokenID     string
	projectKey      string
	applicationName string
	expiry          *uint
	refreshable     bool
	runId           string
	jobId           string
	repo            string
	response        *auth.CreateTokenResponseData
}

func NewOidcTokenExchangeCommand() *OidcTokenExchangeCommand {
	return &OidcTokenExchangeCommand{response: new(auth.CreateTokenResponseData)}
}

func (otc *OidcTokenExchangeCommand) SetServerDetails(serverDetails *config.ServerDetails) *OidcTokenExchangeCommand {
	otc.serverDetails = serverDetails
	return otc
}

func (otc *OidcTokenExchangeCommand) SetProviderName(providerName string) *OidcTokenExchangeCommand {
	otc.providerName = providerName
	return otc
}

// TokenID is received from the OIDC provider
func (otc *OidcTokenExchangeCommand) SetOidcTokenID(oidcTokenID string) *OidcTokenExchangeCommand {
	otc.oidcTokenID = oidcTokenID
	return otc
}

func (otc *OidcTokenExchangeCommand) SetProjectKey(projectKey string) *OidcTokenExchangeCommand {
	otc.projectKey = projectKey
	return otc
}

func (otc *OidcTokenExchangeCommand) SetApplicationName(applicationName string) *OidcTokenExchangeCommand {
	otc.applicationName = applicationName
	return otc
}

func (otc *OidcTokenExchangeCommand) SetAudience(audience string) *OidcTokenExchangeCommand {
	otc.audience = audience
	return otc
}

func (otc *OidcTokenExchangeCommand) SetRunId(runId string) *OidcTokenExchangeCommand {
	otc.runId = runId
	return otc
}

func (otc *OidcTokenExchangeCommand) SetJobId(jobId string) *OidcTokenExchangeCommand {
	otc.jobId = jobId
	return otc
}

func (otc *OidcTokenExchangeCommand) SetRepo(repo string) *OidcTokenExchangeCommand {
	otc.repo = repo
	return otc
}

// TODO check if it should be here
func (otc *OidcTokenExchangeCommand) SetExpiry(expiry *uint) *OidcTokenExchangeCommand {
	otc.expiry = expiry
	return otc
}

// TODO check if it is possible to refresh
func (otc *OidcTokenExchangeCommand) SetRefreshable(refreshable bool) *OidcTokenExchangeCommand {
	otc.refreshable = refreshable
	return otc
}

func (otc *OidcTokenExchangeCommand) Response() ([]byte, error) {
	content, err := json.Marshal(*otc.response)
	return content, errorutils.CheckError(err)
}

func (otc *OidcTokenExchangeCommand) ServerDetails() (*config.ServerDetails, error) {
	return otc.serverDetails, nil
}

func (otc *OidcTokenExchangeCommand) CommandName() string {
	return "jf_oidc_token_exchange"
}

func (otc *OidcTokenExchangeCommand) Run() (err error) {
	servicesManager, err := rtUtils.CreateAccessServiceManager(otc.serverDetails, false)
	if err != nil {
		return err
	}
	*otc.response, err = servicesManager.ExchangeOidcToken(otc.getOidcTokenParams())
	return
}

func (otc *OidcTokenExchangeCommand) getOidcTokenParams() services.CreateOidcTokenParams {
	oidcTokenParams := services.CreateOidcTokenParams{}
	oidcTokenParams.GrantType = grantType
	oidcTokenParams.SubjectTokenType = subjectTokenType
	oidcTokenParams.OidcTokenID = otc.oidcTokenID
	oidcTokenParams.ProjectKey = otc.projectKey
	oidcTokenParams.ApplicationKey = otc.applicationName
	oidcTokenParams.RunId = otc.runId
	oidcTokenParams.JobId = otc.jobId
	oidcTokenParams.Repo = otc.repo

	// Manual values for testing
	//oidcTokenParams.Audience = otc.audience
	oidcTokenParams.Audience = "jfrog-github"
	//oidcTokenParams.ProviderName = otc.providerName
	oidcTokenParams.ProviderName = "setup-jfrog-cli-test"
	// TODO see if this is relevant
	//oidcTokenParams.ExpiresIn = otc.expiry
	//oidcTokenParams.Refreshable = otc.refreshable
	return oidcTokenParams
}
