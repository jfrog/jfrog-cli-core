package utils

import (
	"encoding/base64"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/npm"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/yarn"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	outFormat "github.com/jfrog/jfrog-cli-core/v2/common/format"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"net/http"
	"strings"
)

const (
	minSupportedArtifactoryVersionForNpmCmds = "5.5.2"

	NpmConfigAuthKey = "_auth"
	// Supported only in npm version 9 and above.
	NpmConfigAuthTokenKey = "_authToken"
	NpmConfigRegistryKey  = "registry"
	npmAuthRestApi        = "api/npm/auth"
)

// Constructs npm auth config and registry, manually or by requesting the Artifactory /npm/auth endpoint.
// Since the Artifactory /npm/auth endpoint doesn't handle groups access tokens well,
// we avoid using it when an access token is configured and the npm version supports setting the token directly.
// For yarn, this is always supported.
func GetArtifactoryNpmRepoDetails(repo string, authArtDetails auth.ServiceDetails, isNpmAuthLegacyVersion bool) (npmAuth, registry string, err error) {
	npmAuth, err = getNpmAuth(authArtDetails, isNpmAuthLegacyVersion)
	if err != nil {
		return "", "", err
	}

	if err = utils.ValidateRepoExists(repo, authArtDetails); err != nil {
		return "", "", err
	}

	registry = GetNpmRepositoryUrl(repo, authArtDetails.GetUrl())
	return
}

func getNpmAuth(authArtDetails auth.ServiceDetails, isNpmAuthLegacyVersion bool) (npmAuth string, err error) {
	// For supported npm versions, construct the npm authToken without using Artifactory due to limitations with access tokens.
	if authArtDetails.GetAccessToken() != "" && !isNpmAuthLegacyVersion {
		return constructNpmAuthToken(authArtDetails.GetAccessToken()), nil
	}

	// Check Artifactory version
	err = validateArtifactoryVersionForNpmCmds(authArtDetails)
	if err != nil {
		return
	}

	// Get npm token from Artifactory
	return getNpmAuthFromArtifactory(authArtDetails)
}

// Manually constructs the npm authToken config data.
func constructNpmAuthToken(token string) string {
	return fmt.Sprintf("%s = %s", NpmConfigAuthTokenKey, token)
}

func validateArtifactoryVersionForNpmCmds(artDetails auth.ServiceDetails) error {
	// Get Artifactory version.
	versionStr, err := artDetails.GetVersion()
	if err != nil {
		return err
	}

	// Validate version.
	return clientutils.ValidateMinimumVersion(clientutils.Artifactory, versionStr, minSupportedArtifactoryVersionForNpmCmds)
}

func getNpmAuthFromArtifactory(artDetails auth.ServiceDetails) (npmAuth string, err error) {
	authApiUrl := artDetails.GetUrl() + npmAuthRestApi
	log.Debug("Sending npm auth request")

	// Get npm token from Artifactory.
	client, err := httpclient.ClientBuilder().SetRetries(3).Build()
	if err != nil {
		return "", err
	}
	resp, body, _, err := client.SendGet(authApiUrl, true, artDetails.CreateHttpClientDetails(), "")
	if err != nil {
		return "", err
	}
	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return "", err
	}

	return string(body), nil
}

func GetNpmRepositoryUrl(repositoryName, artifactoryUrl string) string {
	return strings.TrimSuffix(artifactoryUrl, "/") + "/api/npm/" + repositoryName
}

// Remove all the none npm CLI flags from args.
func ExtractNpmOptionsFromArgs(args []string) (detailedSummary, xrayScan bool, scanOutputFormat outFormat.OutputFormat, cleanArgs []string, buildConfig *build.BuildConfiguration, err error) {
	cleanArgs = append([]string(nil), args...)
	cleanArgs, detailedSummary, err = coreutils.ExtractDetailedSummaryFromArgs(cleanArgs)
	if err != nil {
		return
	}

	cleanArgs, xrayScan, err = coreutils.ExtractXrayScanFromArgs(cleanArgs)
	if err != nil {
		return
	}

	cleanArgs, format, err := coreutils.ExtractXrayOutputFormatFromArgs(cleanArgs)
	if err != nil {
		return
	}
	scanOutputFormat, err = outFormat.GetOutputFormat(format)
	if err != nil {
		return
	}
	cleanArgs, buildConfig, err = build.ExtractBuildDetailsFromArgs(cleanArgs)
	return
}

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
