package config

import (
	"errors"
	"github.com/jfrog/jfrog-client-go/access"
	accessservices "github.com/jfrog/jfrog-client-go/access/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"sync"
	"time"

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
	return tokenRefreshPreRequestInterceptor(fields, httpClientDetails, AccessToken)
}

func ArtifactoryTokenRefreshPreRequestInterceptor(fields *auth.CommonConfigFields, httpClientDetails *httputils.HttpClientDetails) (err error) {
	return tokenRefreshPreRequestInterceptor(fields, httpClientDetails, ArtifactoryToken)
}

func tokenRefreshPreRequestInterceptor(fields *auth.CommonConfigFields, httpClientDetails *httputils.HttpClientDetails, tokenType TokenType) (err error) {
	if fields.GetAccessToken() == "" || httpClientDetails.AccessToken == "" {
		return nil
	}

	timePassed, err := auth.GetTokenMinutesPassed(httpClientDetails.AccessToken)
	if err != nil || timePassed < auth.RefreshAfterExpiryMinutes {
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
		return "", err
	}
	lockFile, err := lock.CreateLock(lockDirPath)
	defer func() {
		e := lockFile.Unlock()
		if err == nil {
			err = e
		}
	}()
	if err != nil {
		return "", err
	}

	serverConfiguration, err := GetSpecificConfig(tokenRefreshServerId, true, false)
	if err != nil {
		return "", err
	}
	if tokenRefreshServerId == "" && serverConfiguration != nil {
		tokenRefreshServerId = serverConfiguration.ServerId
	}
	// If token already refreshed, get new token from config
	if serverConfiguration.AccessToken != "" && serverConfiguration.AccessToken != currentAccessToken {
		log.Debug("Fetched new token from config.")
		return serverConfiguration.AccessToken, nil
	}

	// If token isn't already expired, Wait to make sure requests using the current token are sent before it is refreshed and becomes invalid
	timeLeft, err := auth.GetTokenMinutesLeft(currentAccessToken)
	if err != nil {
		return "", err
	}
	if timeLeft > 0 {
		time.Sleep(auth.WaitBeforeRefreshSeconds * time.Second)
	}

	if tokenType == ArtifactoryToken {
		return refreshArtifactoryTokenAndWriteToConfig(serverConfiguration, currentAccessToken)
	}
	if tokenType == AccessToken {
		return refreshAccessTokenAndWriteToConfig(serverConfiguration, currentAccessToken)
	}
	return "", errorutils.CheckError(errors.New("unsupported refreshable token type: " + string(tokenType)))

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
			return "", nil
		}
		log.Debug("New token created successfully.")
	} else {
		log.Debug("Token refreshed successfully.")
	}

	err = writeNewArtifactoryTokens(serverConfiguration, tokenRefreshServerId, newToken.AccessToken, newToken.RefreshToken)
	if err != nil {
		log.Error("Failed writing new tokens to config after handling access token expiry: " + err.Error())
	}
	return newToken.AccessToken, nil
}

func refreshAccessTokenAndWriteToConfig(serverConfiguration *ServerDetails, currentAccessToken string) (string, error) {
	refreshToken := serverConfiguration.RefreshToken
	// Remove previous tokens
	serverConfiguration.AccessToken = ""
	serverConfiguration.RefreshToken = ""
	// Try refreshing tokens
	newToken, err := refreshExpiredAccessToken(serverConfiguration, currentAccessToken, refreshToken)

	if err != nil {
		log.Debug("Refresh token failed: " + err.Error())
		return "", err
	}

	err = writeNewArtifactoryTokens(serverConfiguration, tokenRefreshServerId, newToken.AccessToken, newToken.RefreshToken)
	if err != nil {
		log.Error("Failed writing new tokens to config after handling access token expiry: " + err.Error())
	}
	return newToken.AccessToken, nil
}

func writeNewArtifactoryTokens(serverConfiguration *ServerDetails, serverId, accessToken, refreshToken string) error {
	serverConfiguration.SetAccessToken(accessToken)
	serverConfiguration.SetRefreshToken(refreshToken)

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
	servicesManager, err := createArtifactoryTokensServiceManager(serverDetails)
	if err != nil {
		return auth.CreateTokenResponseData{}, err
	}

	createTokenParams := services.NewCreateTokenParams()
	createTokenParams.Username = serverDetails.User
	createTokenParams.ExpiresIn = expirySeconds
	// User-scoped token
	createTokenParams.Scope = "member-of-groups:*"
	createTokenParams.Refreshable = true

	newToken, err := servicesManager.CreateToken(createTokenParams)
	if err != nil {
		return auth.CreateTokenResponseData{}, err
	}
	return newToken, nil
}

func CreateInitialRefreshableTokensIfNeeded(serverDetails *ServerDetails) (err error) {
	if !(serverDetails.TokenRefreshInterval > 0 && serverDetails.RefreshToken == "" && serverDetails.AccessToken == "") {
		return nil
	}
	mutex.Lock()
	defer mutex.Unlock()
	lockDirPath, err := coreutils.GetJfrogConfigLockDir()
	if err != nil {
		return err
	}
	lockFile, err := lock.CreateLock(lockDirPath)
	defer func() {
		e := lockFile.Unlock()
		if err == nil {
			err = e
		}
	}()
	if err != nil {
		return err
	}

	newToken, err := createTokensForConfig(serverDetails, serverDetails.TokenRefreshInterval*60)
	if err != nil {
		return err
	}
	// Remove initializing value.
	serverDetails.TokenRefreshInterval = 0
	return writeNewArtifactoryTokens(serverDetails, serverDetails.ServerId, newToken.AccessToken, newToken.RefreshToken)
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
	// The tokens passed as parameters are also used for authentication
	noCredsDetails := new(ServerDetails)
	noCredsDetails.Url = serverDetails.Url
	noCredsDetails.ClientCertPath = serverDetails.ClientCertPath
	noCredsDetails.ClientCertKeyPath = serverDetails.ClientCertKeyPath
	noCredsDetails.ServerId = serverDetails.ServerId
	noCredsDetails.IsDefault = serverDetails.IsDefault

	servicesManager, err := createAccessTokensServiceManager(noCredsDetails)
	if err != nil {
		return auth.CreateTokenResponseData{}, err
	}

	refreshTokenParams := accessservices.NewTokenParams()
	refreshTokenParams.AccessToken = currentAccessToken
	refreshTokenParams.RefreshToken = refreshToken
	return servicesManager.RefreshToken(refreshTokenParams)
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
