package token

import (
	"encoding/json"
	"fmt"
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
	// [Optional] Unique identifier for the CI runtime
	runId string
	// [Optional] Unique identifier for the CI job
	jobId string
	// [Optional] CI repository name
	repository string
	response   *auth.OidcTokenResponseData
}

func NewOidcTokenExchangeCommand() *OidcTokenExchangeCommand {
	return &OidcTokenExchangeCommand{response: new(auth.OidcTokenResponseData)}
}

func (otc *OidcTokenExchangeCommand) SetServerDetails(serverDetails *config.ServerDetails) *OidcTokenExchangeCommand {
	otc.serverDetails = serverDetails
	return otc
}

func (otc *OidcTokenExchangeCommand) GetOidToken() string {
	return otc.response.AccessToken
}

func (otc *OidcTokenExchangeCommand) SetProviderName(providerName string) *OidcTokenExchangeCommand {
	otc.providerName = providerName
	return otc
}

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
	otc.repository = repo
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
	if err != nil {
		return err
	}
	fmt.Printf(otc.response.AccessToken)
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
	oidcTokenParams.Repo = otc.repository
	//oidcTokenParams.Audience = otc.audience
	//oidcTokenParams.ProviderName = otc.providerName

	// Manual values for testing
	oidcTokenParams.Audience = "jfrog-github"
	oidcTokenParams.ProviderName = "setup-jfrog-cli-test"
	return oidcTokenParams
}
