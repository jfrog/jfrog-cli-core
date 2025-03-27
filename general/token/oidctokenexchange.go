package token

import (
	"encoding/json"
	"fmt"
	rtUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/access/services"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"strings"
)

const (
	//#nosec G101 // False positive: This is not a hardcoded credential.
	grantType = "urn:ietf:params:oauth:grant-type:token-exchange"
	//#nosec G101 jfrog-ignore
	subjectTokenType = "urn:ietf:params:oauth:token-type:id_token"
)

type OidcProviderType int

const (
	GitHub OidcProviderType = iota
	Azure
	GenericOidc
)

func (p OidcProviderType) String() string {
	return [...]string{"GitHub", "Azure", "GenericOidc"}[p]
}

func OidcProviderTypeFromString(providerType string) (OidcProviderType, error) {
	if providerType == "" {
		return 0, nil
	}
	switch strings.ToLower(providerType) {
	case strings.ToLower(GitHub.String()):
		return GitHub, nil
	case strings.ToLower(Azure.String()):
		return Azure, nil
	case strings.ToLower(GenericOidc.String()):
		return GenericOidc, nil
	default:
		return 0, fmt.Errorf("unsupported oidc provider type: %s", providerType)
	}
}

type OidcTokenExchangeCommand struct {
	*OidcTokenParams
	serverDetails        *config.ServerDetails
	response             *auth.OidcTokenResponseData
	outputTokenToConsole bool
}

type OidcTokenParams struct {
	ProviderType OidcProviderType
	ProviderName string
	TokenId      string
	Audience     string
	// Those values are used to link the token to a specific use, they are optional
	ProjectKey     string
	ApplicationKey string
	JobId          string
	RunId          string
	Repository     string
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
	otc.ProviderName = providerName
	return otc
}

func (otc *OidcTokenExchangeCommand) SetOidcTokenID(oidcTokenID string) *OidcTokenExchangeCommand {
	otc.TokenId = oidcTokenID
	return otc
}

func (otc *OidcTokenExchangeCommand) SetProjectKey(projectKey string) *OidcTokenExchangeCommand {
	otc.ProjectKey = projectKey
	return otc
}

func (otc *OidcTokenExchangeCommand) SetApplicationName(applicationName string) *OidcTokenExchangeCommand {
	otc.ApplicationKey = applicationName
	return otc
}

func (otc *OidcTokenExchangeCommand) SetAudience(audience string) *OidcTokenExchangeCommand {
	otc.Audience = audience
	return otc
}

func (otc *OidcTokenExchangeCommand) SetRunId(runId string) *OidcTokenExchangeCommand {
	otc.RunId = runId
	return otc
}

func (otc *OidcTokenExchangeCommand) SetJobId(jobId string) *OidcTokenExchangeCommand {
	otc.JobId = jobId
	return otc
}

func (otc *OidcTokenExchangeCommand) SetRepository(repo string) *OidcTokenExchangeCommand {
	otc.Repository = repo
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

func (otc *OidcTokenExchangeCommand) ShouldPrintResponse() bool {
	return otc.outputTokenToConsole
}

func (otc *OidcTokenExchangeCommand) SetProviderType(providerType string) (err error) {
	otc.ProviderType, err = OidcProviderTypeFromString(providerType)
	return
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
	return
}

func (otc *OidcTokenExchangeCommand) getOidcTokenParams() services.CreateOidcTokenParams {
	oidcTokenParams := services.CreateOidcTokenParams{}
	oidcTokenParams.GrantType = grantType
	oidcTokenParams.SubjectTokenType = subjectTokenType
	oidcTokenParams.OidcTokenID = otc.TokenId
	oidcTokenParams.ProjectKey = otc.ProjectKey
	oidcTokenParams.ApplicationKey = otc.ApplicationKey
	oidcTokenParams.RunId = otc.RunId
	oidcTokenParams.JobId = otc.JobId
	oidcTokenParams.Repo = otc.Repository
	oidcTokenParams.Audience = otc.Audience
	oidcTokenParams.ProviderName = otc.ProviderName
	return oidcTokenParams
}
