package token

import (
	"encoding/json"
	"fmt"
	rtUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/access/services"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"os"
)

const (
	grantType        = "urn:ietf:params:oauth:grant-type:token-exchange"
	subjectTokenType = "urn:ietf:params:oauth:token-type:id_token"
)

type oidcProviderType int

const (
	GitHub oidcProviderType = iota
	Azure
	GenericOidc
)

func (p oidcProviderType) String() string {
	return [...]string{"GitHub", "Azure", "GenericOidc"}[p]
}

type OidcTokenExchangeCommand struct {
	serverDetails   *config.ServerDetails
	providerName    string
	audience        string
	tokenId         string
	projectKey      string
	applicationName string
	providerType    oidcProviderType
	// [Optional] Unique identifier for the CI runtime
	runId string
	// [Optional] Unique identifier for the CI job
	jobId string
	// [Optional] CI repository name
	repository           string
	response             *auth.OidcTokenResponseData
	outputTokenToConsole bool
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
	otc.tokenId = oidcTokenID
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

func (otc *OidcTokenExchangeCommand) SetRepository(repo string) *OidcTokenExchangeCommand {
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

func (otc *OidcTokenExchangeCommand) SetProviderType(providerType string) error {
	switch providerType {
	case GitHub.String():
		otc.providerType = GitHub
	case Azure.String():
		otc.providerType = Azure
	case GenericOidc.String():
		otc.providerType = GenericOidc
	default:
		return errorutils.CheckError(fmt.Errorf("unspported oidc provider type: %s", providerType))
	}
	return nil
}

func (otc *OidcTokenExchangeCommand) Run() (err error) {
	// OIDC exchange token command will always fail to report to the visibility system
	// we are setting this env to avoid confusing logs in the visibility system
	err = os.Setenv(coreutils.ReportUsage, "false")
	if err != nil {
		return
	}
	defer func() {
		if err = os.Unsetenv(coreutils.ReportUsage); err != nil {
			return
		}
	}()
	servicesManager, err := rtUtils.CreateAccessServiceManager(otc.serverDetails, false)
	if err != nil {
		return err
	}
	*otc.response, err = servicesManager.ExchangeOidcToken(otc.getOidcTokenParams())
	if err != nil {
		return err
	}
	// When we use this command internally, we do not want to token to be outputted to the console
	if otc.outputTokenToConsole {
		fmt.Printf(otc.response.AccessToken)
	}
	return
}

func (otc *OidcTokenExchangeCommand) getOidcTokenParams() services.CreateOidcTokenParams {
	oidcTokenParams := services.CreateOidcTokenParams{}
	oidcTokenParams.GrantType = grantType
	oidcTokenParams.SubjectTokenType = subjectTokenType
	oidcTokenParams.OidcTokenID = otc.tokenId
	oidcTokenParams.ProjectKey = otc.projectKey
	oidcTokenParams.ApplicationKey = otc.applicationName
	oidcTokenParams.RunId = otc.runId
	oidcTokenParams.JobId = otc.jobId
	oidcTokenParams.Repo = otc.repository
	oidcTokenParams.Audience = otc.audience
	oidcTokenParams.ProviderName = otc.providerName
	return oidcTokenParams
}
