package token

import (
	"fmt"
	rtUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/access/services"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"strings"
	"time"
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
	var oidcProviderType OidcProviderType
	if providerType == "" {
		// If no provider type is provided, return 0 (GitHub) as default
		oidcProviderType = 0
	} else {
		switch strings.ToLower(providerType) {
		case strings.ToLower(GitHub.String()):
			oidcProviderType = GitHub
		case strings.ToLower(Azure.String()):
			oidcProviderType = Azure
		case strings.ToLower(GenericOidc.String()):
			oidcProviderType = GenericOidc
		default:
			return 0, fmt.Errorf("unsupported oidc provider type: %s", providerType)
		}
	}
	// This is used for usage reporting
	if err := os.Setenv(coreutils.OidcProviderType, oidcProviderType.String()); err != nil {
		log.Warn("Failed to set JFROG_CLI_OIDC_PROVIDER_TYPE environment variable")
	}
	return oidcProviderType, nil
}

type OidcTokenExchangeCommand struct {
	*OidcParams
	serverDetails *config.ServerDetails
	response      *auth.OidcTokenResponseData
}

type OidcParams struct {
	ProviderType OidcProviderType
	ProviderName string
	TokenId      string
	Audience     string
	// These values are used to link the token to a specific use, they are optional
	ProjectKey     string
	ApplicationKey string
	JobId          string
	RunId          string
	Repository     string
}

func NewOidcTokenExchangeCommand() *OidcTokenExchangeCommand {
	return &OidcTokenExchangeCommand{response: new(auth.OidcTokenResponseData), OidcParams: &OidcParams{}}
}

func (otc *OidcTokenExchangeCommand) SetServerDetails(serverDetails *config.ServerDetails) *OidcTokenExchangeCommand {
	otc.serverDetails = serverDetails
	return otc
}

func (otc *OidcTokenExchangeCommand) GetExchangedToken() string {
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

func (otc *OidcTokenExchangeCommand) SetApplicationKey(applicationName string) *OidcTokenExchangeCommand {
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

func (otc *OidcTokenExchangeCommand) Response() (response *auth.OidcTokenResponseData) {
	return otc.response
}

func (otc *OidcTokenExchangeCommand) ServerDetails() (*config.ServerDetails, error) {
	return otc.serverDetails, nil
}

func (otc *OidcTokenExchangeCommand) CommandName() string {
	return "jf_oidc_token_exchange"
}

func (otc *OidcTokenExchangeCommand) SetProviderType(providerType OidcProviderType) *OidcTokenExchangeCommand {
	otc.ProviderType = providerType
	return otc
}

func (otc *OidcTokenExchangeCommand) SetProviderTypeAsString(providerType string) (err error) {
	otc.ProviderType, err = OidcProviderTypeFromString(providerType)
	return
}

func (otc *OidcTokenExchangeCommand) PrintResponseToConsole() {
	log.Output(fmt.Sprintf("{ AccessToken: %s Username: %s }", otc.response.AccessToken, otc.response.Username))
}

func (otc *OidcTokenExchangeCommand) Run() (err error) {
	servicesManager, err := rtUtils.CreateAccessServiceManager(otc.serverDetails, false)
	if err != nil {
		return err
	}
	*otc.response, err = servicesManager.ExchangeOidcToken(otc.getOidcTokenParams())
	// Update the config server details with the exchanged token
	otc.serverDetails.AccessToken = otc.response.AccessToken

	// Safe to log token details for easier debugging
	log.Debug("Token Scope: ", otc.response.Scope)
	if otc.response.ExpiresIn != nil {
		expirationTime := time.Now().Add(time.Duration(*otc.response.ExpiresIn) * time.Second)
		log.Debug("Token Expiration Date: ", expirationTime)
	}
	log.Debug("Token Audience: ", otc.response.Audience)
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
