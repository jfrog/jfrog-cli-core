package utils

import (
	"encoding/base64"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/npm"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/yarn"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"strings"
)

// NpmrcYarnrcManager is responsible for configuring npm and Yarn registries
// and authentication settings based on the specified project type.
type NpmrcYarnrcManager struct {
	// buildTool represents the project type, either NPM or Yarn.
	buildTool project.ProjectType
	// repoUrl holds the URL to the npm or Yarn repository.
	repoUrl string
	// serverDetails contains configuration details for the Artifactory server.
	serverDetails *config.ServerDetails
}

// NewNpmrcYarnrcManager initializes a new NpmrcYarnrcManager with the given project type,
// repository name, and Artifactory server details.
func NewNpmrcYarnrcManager(buildTool project.ProjectType, repoName string, serverDetails *config.ServerDetails) *NpmrcYarnrcManager {
	repoUrl := GetNpmRepositoryUrl(repoName, serverDetails.ArtifactoryUrl)
	return &NpmrcYarnrcManager{
		buildTool:     buildTool,
		repoUrl:       repoUrl,
		serverDetails: serverDetails,
	}
}

// ConfigureRegistry sets the registry URL in the npmrc or yarnrc file.
func (nm *NpmrcYarnrcManager) Run() error {
	switch nm.buildTool {
	case project.Npm, project.Yarn:
		if err := nm.ConfigureRegistry(); err != nil {
			return err
		}
		return nm.ConfigureAuth()
	default:
		return errorutils.CheckError(fmt.Errorf("unsupported build tool: %s", nm.buildTool))
	}
}

// ConfigureRegistry sets the registry URL in the npmrc or yarnrc file.
func (nm *NpmrcYarnrcManager) ConfigureRegistry() error {
	return nm.configSet(NpmConfigRegistryKey, nm.repoUrl)
}

// ConfigureAuth configures authentication in npmrc or yarnrc using token or basic auth,
// or clears authentication for anonymous access.
func (nm *NpmrcYarnrcManager) ConfigureAuth() error {
	authArtDetails, err := nm.serverDetails.CreateArtAuthConfig()
	if err != nil {
		return err
	}

	// Configure authentication based on available credentials.
	switch {
	case authArtDetails.GetAccessToken() != "":
		return nm.handleNpmrcTokenAuth(authArtDetails.GetAccessToken())
	case authArtDetails.GetUser() != "" && authArtDetails.GetPassword() != "":
		return nm.handleNpmrcBasicAuth(authArtDetails.GetUser(), authArtDetails.GetPassword())
	default:
		return nm.handleNpmAnonymousAccess()
	}
}

// handleNpmrcTokenAuth sets the token in the npmrc or yarnrc file and clears basic auth if it exists.
func (nm *NpmrcYarnrcManager) handleNpmrcTokenAuth(token string) error {
	authKey := nm.createAuthKey(NpmConfigAuthTokenKey)
	if err := nm.configSet(authKey, token); err != nil {
		return err
	}
	return nm.removeNpmrcBasicAuthIfExists()
}

// handleNpmrcBasicAuth sets basic auth credentials and clears any token-based auth.
func (nm *NpmrcYarnrcManager) handleNpmrcBasicAuth(user, password string) error {
	authKey := nm.createAuthKey(NpmConfigAuthKey)
	authValue := basicAuthBase64Encode(user, password)
	if err := nm.configSet(authKey, authValue); err != nil {
		return err
	}
	return nm.removeNpmrcTokenAuthIfExists()
}

// handleNpmAnonymousAccess removes any existing authentication settings for anonymous access.
func (nm *NpmrcYarnrcManager) handleNpmAnonymousAccess() error {
	if err := nm.removeNpmrcBasicAuthIfExists(); err != nil {
		return err
	}
	return nm.removeNpmrcTokenAuthIfExists()
}

// removeNpmrcBasicAuthIfExists deletes basic auth credentials if present.
func (nm *NpmrcYarnrcManager) removeNpmrcBasicAuthIfExists() error {
	return nm.configDelete(nm.createAuthKey(NpmConfigAuthKey))
}

// removeNpmrcTokenAuthIfExists deletes token auth credentials if present.
func (nm *NpmrcYarnrcManager) removeNpmrcTokenAuthIfExists() error {
	return nm.configDelete(nm.createAuthKey(NpmConfigAuthTokenKey))
}

// configSet applies a configuration setting in npmrc or yarnrc, based on the build tool type.
func (nm *NpmrcYarnrcManager) configSet(key, value string) error {
	switch nm.buildTool {
	case project.Npm:
		return npm.ConfigSet(key, value, nm.buildTool.String())
	case project.Yarn:
		return yarn.ConfigSet(key, value, nm.buildTool.String(), false)
	default:
		return errorutils.CheckError(fmt.Errorf("unsupported build tool: %s", nm.buildTool))
	}
}

// configDelete removes a configuration setting from npmrc or yarnrc, based on the build tool type.
func (nm *NpmrcYarnrcManager) configDelete(key string) error {
	switch nm.buildTool {
	case project.Npm:
		return npm.ConfigDelete(key, nm.buildTool.String())
	case project.Yarn:
		return yarn.ConfigDelete(key, nm.buildTool.String())
	default:
		return errorutils.CheckError(fmt.Errorf("unsupported build tool: %s", nm.buildTool))
	}
}

// createAuthKey generates the correct authentication key for npm or Yarn, based on the repo URL.
func (nm *NpmrcYarnrcManager) createAuthKey(keySuffix string) string {
	return fmt.Sprintf("//%s:%s", strings.TrimPrefix(nm.repoUrl, "https://"), keySuffix)
}

// basicAuthBase64Encode encodes user credentials in Base64 for basic authentication.
func basicAuthBase64Encode(user, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", user, password)))
}
