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
	return tokenRefreshPreRequestInterceptor(fields, httpClientDetails, AccessToken, auth.InviteRefreshBeforeExpiryMinutes)
}

func ArtifactoryTokenRefreshPreRequestInterceptor(fields *auth.CommonConfigFields, httpClientDetails *httputils.HttpClientDetails) (err error) {
	return tokenRefreshPreRequestInterceptor(fields, httpClientDetails, ArtifactoryToken, auth.RefreshBeforeExpiryMinutes)
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
		e := unlockFunc()
		if err == nil {
			err = e
		}
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
	err = errorutils.CheckError(errors.New("unsupported refreshable token type: " + string(tokenType)))
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

	err = writeNewArtifactoryTokens(serverConfiguration, tokenRefreshServerId, newToken.AccessToken, newToken.RefreshToken)
	return newToken.AccessToken, err
}

func refreshAccessTokenAndWriteToConfig(serverConfiguration *ServerDetails, currentAccessToken string) (string, error) {
	// Try refreshing tokens
	newToken, err := refreshExpiredAccessToken(serverConfiguration, currentAccessToken, serverConfiguration.RefreshToken)
	if err != nil {
		return "", errorutils.CheckError(errors.New("Refresh access token failed: " + err.Error()))
	}
	err = writeNewArtifactoryTokens(serverConfiguration, tokenRefreshServerId, newToken.AccessToken, newToken.RefreshToken)
	return newToken.AccessToken, err
}

func writeNewArtifactoryTokens(serverConfiguration *ServerDetails, serverId, accessToken, refreshToken string) error {
	serverConfiguration.SetAccessToken(accessToken)
	serverConfiguration.SetArtifactoryRefreshToken(refreshToken)

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
	if !(serverDetails.ArtifactoryTokenRefreshInterval > 0 && serverDetails.ArtifactoryRefreshToken == "" && serverDetails.AccessToken == "") ||
		(serverDetails.RefreshToken != "" && serverDetails.AccessToken != "") {
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
		e := unlockFunc()
		if err == nil {
			err = e
		}
	}()
	if err != nil {
		return
	}

	newToken, err := createTokensForConfig(serverDetails, serverDetails.ArtifactoryTokenRefreshInterval*60)
	if err != nil {
		return
	}
	// Remove initializing value.
	serverDetails.ArtifactoryTokenRefreshInterval = 0
	err = writeNewArtifactoryTokens(serverDetails, serverDetails.ServerId, newToken.AccessToken, newToken.RefreshToken)
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
