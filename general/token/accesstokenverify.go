package token

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	rtUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/auth/accesstoken"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	referenceTokenPrefix = "cmVmdGtu"
	rootCertificateCall  = "GET /access/api/v1/cert/root"
	readinessCall        = "GET /access/api/v1/system/readiness"
)

type AccessTokenVerifyResult struct {
	Valid   bool                   `json:"valid"`
	Error   string                 `json:"error,omitempty"`
	Header  *accesstoken.Header    `json:"header,omitempty"`
	Payload map[string]interface{} `json:"payload,omitempty"`
	Types   []TokenType            `json:"types,omitempty"`
}

type accessClient interface {
	GetRootCertificate() (string, error)
	CheckReadiness() (int, error)
}

type AccessTokenVerifyCommand struct {
	serverDetails   *config.ServerDetails
	usageDetails    *config.ServerDetails
	token           string
	result          *AccessTokenVerifyResult
	now             func() time.Time
	newAccessClient func(*config.ServerDetails) (accessClient, error)
}

func NewAccessTokenVerifyCommand() *AccessTokenVerifyCommand {
	return &AccessTokenVerifyCommand{
		now: time.Now,
		newAccessClient: func(details *config.ServerDetails) (accessClient, error) {
			return rtUtils.CreateAccessServiceManager(details, false)
		},
	}
}

func (c *AccessTokenVerifyCommand) SetServerDetails(serverDetails *config.ServerDetails) *AccessTokenVerifyCommand {
	c.serverDetails = serverDetails
	c.usageDetails = withoutTokenRefresh(serverDetails)
	return c
}

// Usage reporting authenticates with these details; without the refresh tokens, it can't refresh or rewrite the profile mid-verification.
func withoutTokenRefresh(profile *config.ServerDetails) *config.ServerDetails {
	if profile == nil {
		return nil
	}
	details := *profile
	details.RefreshToken = ""
	details.ArtifactoryRefreshToken = ""
	details.DisableTokenRefresh = true
	return &details
}

func (c *AccessTokenVerifyCommand) SetToken(token string) *AccessTokenVerifyCommand {
	c.token = token
	return c
}

func (c *AccessTokenVerifyCommand) ServerDetails() (*config.ServerDetails, error) {
	return c.usageDetails, nil
}

func (c *AccessTokenVerifyCommand) CommandName() string {
	return "jf_access_token_verify"
}

func (c *AccessTokenVerifyCommand) Result() *AccessTokenVerifyResult {
	return c.result
}

func (c *AccessTokenVerifyCommand) Run() error {
	c.result = &AccessTokenVerifyResult{}
	if err := c.verify(); err != nil {
		c.result.Error = err.Error()
		return err
	}
	c.result.Valid = true
	return nil
}

func (c *AccessTokenVerifyCommand) verify() error {
	if httpclient.IsApiKey(c.token) {
		return errorutils.CheckErrorf("the provided value is an API key, not an access token")
	}
	if strings.HasPrefix(c.token, referenceTokenPrefix) {
		return errorutils.CheckErrorf("the provided value is a reference token, which cannot be decoded locally; verify the JWT access token instead")
	}
	parsed, err := accesstoken.ParseToken(c.token)
	if err != nil {
		return errorutils.CheckErrorf("failed to parse token: %s", err.Error())
	}
	c.result.Header = &parsed.Header
	c.result.Payload = parsed.Claims
	if err = accesstoken.ValidateHeader(parsed.Header); err != nil {
		return err
	}
	if c.serverDetails == nil || c.serverDetails.Url == "" {
		return errorutils.CheckErrorf("no JFrog server with a platform URL is configured; run 'jf config add'")
	}
	if warning := insecureConnectionWarning(c.serverDetails); warning != "" {
		log.Warn(warning)
	}
	client, err := c.newAccessClient(tokenUnderTestDetails(c.serverDetails, c.token))
	if err != nil {
		return err
	}
	if err = verifySignature(client, c.token); err != nil {
		return err
	}
	if err = accesstoken.ValidatePayloadFields(parsed.Payload); err != nil {
		return err
	}
	if err = accesstoken.ValidateExpiration(parsed.Payload, c.now()); err != nil {
		return err
	}
	if err = checkReadiness(client); err != nil {
		return err
	}
	c.result.Types = IdentifyTokenTypes(parsed)
	return nil
}

func verifySignature(client accessClient, token string) error {
	rootCert, err := client.GetRootCertificate()
	if err != nil {
		return errorutils.CheckErrorf("failed to fetch root certificate (%s): %s", rootCertificateCall, err.Error())
	}
	key, err := accesstoken.ParseRootCertificate(rootCert)
	if err != nil {
		return err
	}
	return accesstoken.ValidateSignature(token, key)
}

func checkReadiness(client accessClient) error {
	status, err := client.CheckReadiness()
	switch {
	case err != nil:
		return errorutils.CheckErrorf("failed to reach Access server (%s): %s", readinessCall, err.Error())
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return errorutils.CheckErrorf("token rejected by Access server: status %d from %s", status, readinessCall)
	case status != http.StatusOK:
		return errorutils.CheckErrorf("unexpected response from Access server: status %d from %s", status, readinessCall)
	}
	return nil
}

// Starts from blank details: copying the profile would carry its refresh token, and the refresh hook
// could then send the profile's own token instead of the one under test.
func tokenUnderTestDetails(profile *config.ServerDetails, token string) *config.ServerDetails {
	details := new(config.ServerDetails)
	details.Url = profile.Url
	details.ClientCertPath = profile.ClientCertPath
	details.ClientCertKeyPath = profile.ClientCertKeyPath
	details.AccessToken = token
	details.DisableTokenRefresh = true
	return details
}

func insecureConnectionWarning(details *config.ServerDetails) string {
	if !strings.HasPrefix(strings.ToLower(details.Url), "http://") {
		return ""
	}
	return fmt.Sprintf("The server URL %s uses http://, so the signature check can't prove the root certificate came from your JFrog server.", details.Url)
}
