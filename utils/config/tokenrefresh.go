package config

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jfrog/jfrog-client-go/access"
	accessservices "github.com/jfrog/jfrog-client-go/access/services"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/lock"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Internal golang locking for the same process.
var mutex sync.Mutex

// The serverId used for authentication. Use for reading and writing tokens from/to the config file, and for reading the credentials if needed.
var tokenRefreshServerId string

const (
	ArtifactoryToken TokenType = "artifactory"
	AccessToken      TokenType = "access"
)

type TokenType string

func AccessTokenRefreshPreRequestInterceptor(fields *auth.CommonConfigFields, httpClientDetails *httputils.HttpClientDetails) (err error) {
	return tokenRefreshPreRequestInterceptor(fields, httpClientDetails, AccessToken, auth.RefreshPlatformTokenBeforeExpiryMinutes)
}

func ArtifactoryTokenRefreshPreRequestInterceptor(fields *auth.CommonConfigFields, httpClientDetails *httputils.HttpClientDetails) (err error) {
	return tokenRefreshPreRequestInterceptor(fields, httpClientDetails, ArtifactoryToken, auth.RefreshArtifactoryTokenBeforeExpiryMinutes)
}

func tokenRefreshPreRequestInterceptor(fields *auth.CommonConfigFields, httpClientDetails *httputils.HttpClientDetails, tokenType TokenType, refreshBeforeExpiryMinutes int64) (err error) {
	if fields.GetAccessToken() == "" || httpClientDetails.AccessToken == "" {
		return nil
	}

	timeLeft, err := auth.GetTokenMinutesLeft(httpClientDetails.AccessToken)
	if err != nil || timeLeft > refreshBeforeExpiryMinutes {
		return err
	}
	// Lock to make sure only one thread is trying to refresh
	mutex.Lock()
	defer mutex.Unlock()
	// Refresh only if a new token wasn't acquired (by another thread) while waiting at mutex.
	if fields.AccessToken == httpClientDetails.AccessToken {
		newAccessToken, err := tokenRefreshHandler(httpClientDetails.AccessToken, tokenType)
		if err != nil {
			return err
		}
		if newAccessToken != "" && newAccessToken != httpClientDetails.AccessToken {
			fields.AccessToken = newAccessToken
		}
	}
	// Copy new token from the mutual struct CommonConfigFields to the private struct in httpClientDetails
	httpClientDetails.AccessToken = fields.AccessToken
	return nil
}

func tokenRefreshHandler(currentAccessToken string, tokenType TokenType) (newAccessToken string, err error) {
	log.Debug("Refreshing token...")
	// Lock config to prevent access from different processes
	lockDirPath, err := coreutils.GetJfrogConfigLockDir()
	if err != nil {
		return
	}
	unlockFunc, err := lock.CreateLock(lockDirPath)
	// Defer the lockFile.Unlock() function before throwing a possible error to avoid deadlock situations.
	defer func() {
		err = errors.Join(err, unlockFunc())
	}()
	if err != nil {
		return
	}

	serverConfiguration, err := GetSpecificConfig(tokenRefreshServerId, true, false)
	if err != nil {
		return
	}
	if tokenRefreshServerId == "" && serverConfiguration != nil {
		tokenRefreshServerId = serverConfiguration.ServerId
	}
	// If token already refreshed, get new token from config
	if serverConfiguration.AccessToken != "" && serverConfiguration.AccessToken != currentAccessToken {
		log.Debug("Fetched new token from config.")
		newAccessToken = serverConfiguration.AccessToken
		return
	}

	// If token isn't already expired, Wait to make sure requests using the current token are sent before it is refreshed and becomes invalid
	timeLeft, err := auth.GetTokenMinutesLeft(currentAccessToken)
	if err != nil {
		return
	}
	if timeLeft > 0 {
		time.Sleep(auth.WaitBeforeRefreshSeconds * time.Second)
	}

	if tokenType == ArtifactoryToken {
		newAccessToken, err = refreshArtifactoryTokenAndWriteToConfig(serverConfiguration, currentAccessToken)
		return
	}
	if tokenType == AccessToken {
		newAccessToken, err = refreshAccessTokenAndWriteToConfig(serverConfiguration, currentAccessToken)
		return
	}
	err = errorutils.CheckErrorf("unsupported refreshable token type: %s", string(tokenType))
	return
}

func refreshArtifactoryTokenAndWriteToConfig(serverConfiguration *ServerDetails, currentAccessToken string) (string, error) {
	refreshToken := serverConfiguration.ArtifactoryRefreshToken
	// Remove previous tokens
	serverConfiguration.AccessToken = ""
	serverConfiguration.ArtifactoryRefreshToken = ""
	// Try refreshing tokens
	newToken, err := refreshArtifactoryExpiredToken(serverConfiguration, currentAccessToken, refreshToken)

	if err != nil {
		log.Debug("Refresh token failed: " + err.Error())
		log.Debug("Trying to create new tokens...")

		expirySeconds, err := auth.ExtractExpiryFromAccessToken(currentAccessToken)
		if err != nil {
			return "", err
		}

		newToken, err = createTokensForConfig(serverConfiguration, expirySeconds)
		if err != nil {
			return "", err
		}
		log.Debug("New token created successfully.")
	} else {
		log.Debug("Token refreshed successfully.")
	}

	err = writeNewTokens(serverConfiguration, tokenRefreshServerId, newToken.AccessToken, newToken.RefreshToken, ArtifactoryToken)
	return newToken.AccessToken, err
}

func refreshAccessTokenAndWriteToConfig(serverConfiguration *ServerDetails, currentAccessToken string) (string, error) {
	// Try refreshing tokens
	newToken, err := refreshExpiredAccessToken(serverConfiguration, currentAccessToken, serverConfiguration.RefreshToken)
	if err != nil {
		return "", errorutils.CheckErrorf("Refresh access token failed: %s", err.Error())
	}
	err = writeNewTokens(serverConfiguration, tokenRefreshServerId, newToken.AccessToken, newToken.RefreshToken, AccessToken)
	return newToken.AccessToken, err
}

func writeNewTokens(serverConfiguration *ServerDetails, serverId, accessToken, refreshToken string, tokenType TokenType) error {
	serverConfiguration.SetAccessToken(accessToken)

	switch tokenType {
	case ArtifactoryToken:
		serverConfiguration.SetArtifactoryRefreshToken(refreshToken)
	case AccessToken:
		serverConfiguration.SetRefreshToken(refreshToken)
	}

	// Get configurations list
	configurations, err := GetAllServersConfigs()
	if err != nil {
		return err
	}

	// Remove and get the server details from the configurations list
	_, configurations = GetAndRemoveConfiguration(serverId, configurations)

	// Append the configuration to the configurations list
	configurations = append(configurations, serverConfiguration)
	return SaveServersConf(configurations)
}

func createTokensForConfig(serverDetails *ServerDetails, expirySeconds int) (auth.CreateTokenResponseData, error) {
	expiresIn := uint(max(expirySeconds, 0)) // #nosec G115 -- expirySeconds is validated positive by callers
	createTokenParams := accessservices.CreateTokenParams{
		CommonTokenParams: auth.CommonTokenParams{
			Scope:       "applied-permissions/user",
			ExpiresIn:   &expiresIn,
			Refreshable: clientutils.Pointer(true),
		},
		Username: serverDetails.User,
	}

	// First, try with the original credentials (basic auth: user + password).
	servicesManager, err := createAccessTokensServiceManager(serverDetails)
	if err != nil {
		return auth.CreateTokenResponseData{}, err
	}
	newToken, err := servicesManager.CreateAccessToken(createTokenParams)
	if err == nil {
		return newToken, nil
	}

	// If basic auth failed and a password is available, retry using it as a Bearer token.
	// This handles reference tokens, which the Access service can resolve server-side.
	if serverDetails.Password != "" {
		bearerDetails := copyServerDetailsWithAccessToken(serverDetails, serverDetails.Password)
		servicesManager, err = createAccessTokensServiceManager(bearerDetails)
		if err != nil {
			return auth.CreateTokenResponseData{}, err
		}
		newToken, err = servicesManager.CreateAccessToken(createTokenParams)
		if err == nil {
			return newToken, nil
		}
		log.Debug("Access token creation with Bearer auth failed: " + err.Error())
	}

	return auth.CreateTokenResponseData{}, fmt.Errorf(
		"automatic token creation via the Access API failed: %s. "+
			"If your JFrog Platform version does not support the Access API, please upgrade. "+
			"Alternatively, use the --basic-auth-only flag to skip automatic token creation",
		err.Error())
}

// copyServerDetailsWithAccessToken creates a shallow copy of ServerDetails with the given
// token set as AccessToken and credentials cleared, so that the Access API uses Bearer
// authentication instead of basic auth.
func copyServerDetailsWithAccessToken(original *ServerDetails, token string) *ServerDetails {
	copy := *original
	copy.AccessToken = token
	copy.User = ""
	copy.Password = ""
	return &copy
}

func CreateInitialRefreshableTokensIfNeeded(serverDetails *ServerDetails) (err error) {
	if serverDetails.ArtifactoryTokenRefreshInterval <= 0 || serverDetails.ArtifactoryRefreshToken != "" || serverDetails.AccessToken != "" {
		return nil
	}
	mutex.Lock()
	defer mutex.Unlock()
	lockDirPath, err := coreutils.GetJfrogConfigLockDir()
	if err != nil {
		return
	}
	unlockFunc, err := lock.CreateLock(lockDirPath)
	// Defer the lockFile.Unlock() function before throwing a possible error to avoid deadlock situations.
	defer func() {
		err = errors.Join(err, unlockFunc())
	}()
	if err != nil {
		return
	}

	newToken, tokenErr := createTokensForConfig(serverDetails, serverDetails.ArtifactoryTokenRefreshInterval*60)
	if tokenErr != nil {
		serverDetails.ArtifactoryTokenRefreshInterval = 0
		return nil
	}
	// Remove initializing value.
	serverDetails.ArtifactoryTokenRefreshInterval = 0
	err = writeNewTokens(serverDetails, serverDetails.ServerId, newToken.AccessToken, newToken.RefreshToken, ArtifactoryToken)
	return
}

func refreshArtifactoryExpiredToken(serverDetails *ServerDetails, currentAccessToken string, refreshToken string) (auth.CreateTokenResponseData, error) {
	// The tokens passed as parameters are also used for authentication
	noCredsDetails := new(ServerDetails)
	noCredsDetails.ArtifactoryUrl = serverDetails.ArtifactoryUrl
	noCredsDetails.ClientCertPath = serverDetails.ClientCertPath
	noCredsDetails.ClientCertKeyPath = serverDetails.ClientCertKeyPath
	noCredsDetails.ServerId = serverDetails.ServerId
	noCredsDetails.IsDefault = serverDetails.IsDefault

	servicesManager, err := createArtifactoryTokensServiceManager(noCredsDetails)
	if err != nil {
		return auth.CreateTokenResponseData{}, err
	}

	refreshTokenParams := services.NewArtifactoryRefreshTokenParams()
	refreshTokenParams.AccessToken = currentAccessToken
	refreshTokenParams.RefreshToken = refreshToken
	return servicesManager.RefreshToken(refreshTokenParams)
}

func refreshExpiredAccessToken(serverDetails *ServerDetails, currentAccessToken string, refreshToken string) (auth.CreateTokenResponseData, error) {
	// Creating accessTokens service manager without credentials.
	// In case credentials were provided accessTokens refresh mechanism will be operated. That will cause recursive locking mechanism.
	noCredServerDetails := new(ServerDetails)
	noCredServerDetails.Url = serverDetails.Url
	noCredServerDetails.ClientCertPath = serverDetails.ClientCertPath
	noCredServerDetails.ClientCertKeyPath = serverDetails.ClientCertKeyPath
	noCredServerDetails.ServerId = serverDetails.ServerId
	noCredServerDetails.IsDefault = serverDetails.IsDefault

	servicesManager, err := createAccessTokensServiceManager(noCredServerDetails)
	if err != nil {
		return auth.CreateTokenResponseData{}, err
	}

	refreshTokenParams := accessservices.CreateTokenParams{}
	refreshTokenParams.AccessToken = currentAccessToken
	refreshTokenParams.RefreshToken = refreshToken
	return servicesManager.RefreshAccessToken(refreshTokenParams)
}

func createArtifactoryTokensServiceManager(artDetails *ServerDetails) (artifactory.ArtifactoryServicesManager, error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	artAuth, err := artDetails.CreateArtAuthConfig()
	if err != nil {
		return nil, err
	}
	serviceConfig, err := config.NewConfigBuilder().
		SetServiceDetails(artAuth).
		SetCertificatesPath(certsPath).
		SetInsecureTls(artDetails.InsecureTls).
		SetDryRun(false).
		Build()
	if err != nil {
		return nil, err
	}
	return artifactory.New(serviceConfig)
}

func createAccessTokensServiceManager(serviceDetails *ServerDetails) (*access.AccessServicesManager, error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	accessAuth, err := serviceDetails.CreateAccessAuthConfig()
	if err != nil {
		return nil, err
	}
	serviceConfig, err := config.NewConfigBuilder().
		SetServiceDetails(accessAuth).
		SetCertificatesPath(certsPath).
		SetInsecureTls(serviceDetails.InsecureTls).
		SetDryRun(false).
		Build()
	if err != nil {
		return nil, err
	}
	return access.New(serviceConfig)
}
