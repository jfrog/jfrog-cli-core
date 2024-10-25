package utils

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/stretchr/testify/assert"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testArtifactoryUrl = "https://acme.jfrog.io/artifactory"

func TestGetRegistry(t *testing.T) {
	var getRegistryTest = []struct {
		repo     string
		url      string
		expected string
	}{
		{"repo", "http://url/art", "http://url/art/api/npm/repo"},
		{"repo", "http://url/art/", "http://url/art/api/npm/repo"},
		{"repo", "", "/api/npm/repo"},
		{"", "http://url/art", "http://url/art/api/npm/"},
	}

	for _, testCase := range getRegistryTest {
		if GetNpmRepositoryUrl(testCase.repo, testCase.url) != testCase.expected {
			t.Errorf("The expected output of getRegistry(\"%s\", \"%s\") is %s. But the actual result is:%s", testCase.repo, testCase.url, testCase.expected, GetNpmRepositoryUrl(testCase.repo, testCase.url))
		}
	}
}

type dummyArtifactoryServiceDetails struct {
	auth.CommonConfigFields
}

func (det *dummyArtifactoryServiceDetails) GetVersion() (string, error) {
	return "7.0.0", nil
}

const npmAuthResponse = "_auth = someCred\nalways-auth = true\nemail = some@gmail.com"

var getNpmAuthCases = []struct {
	testName               string
	expectedNpmAuth        string
	isNpmAuthLegacyVersion bool
	user                   string
	password               string
	accessToken            string
}{
	{"basic auth", npmAuthResponse, false, "user", "password", ""},
	{"basic auth legacy", npmAuthResponse, true, "user", "password", ""},
	{"access token", constructNpmAuthToken("token"), false, "", "", "token"},
	{"access token legacy", npmAuthResponse, true, "", "", "token"},
}

func TestGetNpmAuth(t *testing.T) {
	// Prepare mock server
	testServer := commonTests.CreateRestsMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+npmAuthRestApi {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(npmAuthResponse))
			assert.NoError(t, err)
		}
	})
	defer testServer.Close()

	for _, testCase := range getNpmAuthCases {
		t.Run(testCase.testName, func(t *testing.T) {
			authDetails := dummyArtifactoryServiceDetails{CommonConfigFields: auth.CommonConfigFields{Url: testServer.URL + "/", User: testCase.user, Password: testCase.password, AccessToken: testCase.accessToken}}
			actualNpmAuth, err := getNpmAuth(&authDetails, testCase.isNpmAuthLegacyVersion)
			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedNpmAuth, actualNpmAuth)
		})
	}
}

// Helper function to set up the NpmrcYarnrcManager.
func setupNpmrcManager(buildTool project.ProjectType) *NpmrcYarnrcManager {
	serverDetails := &config.ServerDetails{
		ArtifactoryUrl: testArtifactoryUrl,
	}
	return NewNpmrcYarnrcManager(buildTool, "my-repo-virtual", serverDetails)
}

// Helper function to create a temporary .npmrc file for isolated tests.
func createTempNpmrc(t *testing.T) string {
	// Create a temporary directory for npmrc
	tempDir := t.TempDir()

	// Set the NPM_CONFIG_USERCONFIG environment variable
	tempNpmrcPath := filepath.Join(tempDir, ".npmrc")
	t.Setenv("NPM_CONFIG_USERCONFIG", tempNpmrcPath)

	return tempNpmrcPath
}

// Helper function to create a temporary .npmrc file for isolated tests.
func createTempYarnrc(t *testing.T) (string, func() error) {
	homeDir, err := os.UserHomeDir()
	assert.NoError(t, err)
	yarnrcPath := filepath.Join(homeDir, ".yarnrc")
	restoreYarnrcFunc, err := ioutils.BackupFile(yarnrcPath, ".yarnrc.backup")
	assert.NoError(t, err)
	return yarnrcPath, restoreYarnrcFunc
}

// Test for configuring registry in npm.
func TestConfigureRegistry_Npm(t *testing.T) {
	tempNpmrcPath := createTempNpmrc(t)

	nm := setupNpmrcManager(project.Npm)

	err := nm.ConfigureRegistry()
	assert.NoError(t, err)

	// Verify that the correct .npmrc entry was added
	fileContent, err := os.ReadFile(tempNpmrcPath)
	assert.NoError(t, err)
	assert.Contains(t, string(fileContent), "registry="+testArtifactoryUrl)
}

// Test for configuring registry in yarn.
func TestConfigureRegistry_Yarn(t *testing.T) {
	yarnrcPath, restoreYarnrcFunc := createTempYarnrc(t)
	defer func() {
		assert.NoError(t, restoreYarnrcFunc())
	}()
	nm := setupNpmrcManager(project.Yarn)

	err := nm.ConfigureRegistry()
	assert.NoError(t, err)

	// Verify that the correct .yarnrc entry was added
	fileContent, err := os.ReadFile(yarnrcPath)
	assert.NoError(t, err)
	expectedRegistryLine := fmt.Sprintf("registry \"%sapi/npm/my-repo-virtual\"", testArtifactoryUrl)
	assert.Contains(t, string(fileContent), expectedRegistryLine)
}

// Test for setting token auth in npm.
func TestConfigureAuth_Token_Npm(t *testing.T) {
	tempNpmrcPath := createTempNpmrc(t)

	nm := setupNpmrcManager(project.Npm)

	err := nm.handleNpmrcTokenAuth("my-access-token")
	assert.NoError(t, err)

	// Verify that the correct auth token entry was added to .npmrc
	fileContent, err := os.ReadFile(tempNpmrcPath)
	assert.NoError(t, err)
	expectedAuthLine := fmt.Sprintf("//%s/api/npm/my-repo-virtual:_authToken=my-access-token", strings.TrimPrefix(testArtifactoryUrl, "https://"))
	assert.Contains(t, string(fileContent), expectedAuthLine)
}

// Test for setting token auth in yarn.
func TestConfigureAuth_Token_Yarn(t *testing.T) {
	yarnrcPath, restoreYarnrcFunc := createTempYarnrc(t)
	defer func() {
		assert.NoError(t, restoreYarnrcFunc())
	}()

	nm := setupNpmrcManager(project.Yarn)

	err := nm.handleNpmrcTokenAuth("my-access-token")
	assert.NoError(t, err)

	// Verify that the correct auth token entry was added to .yarnrc
	fileContent, err := os.ReadFile(yarnrcPath)
	assert.NoError(t, err)
	expectedAuthLine := fmt.Sprintf("\"//%s/api/npm/my-repo-virtual:_authToken\" my-access-token", strings.TrimPrefix(testArtifactoryUrl, "https://"))
	assert.Contains(t, string(fileContent), expectedAuthLine)
}

func TestHandleNpmrcBasicAuth(t *testing.T) {
	// Set up a temporary .npmrc configuration.
	tempDir := t.TempDir()
	t.Setenv("NPM_CONFIG_USERCONFIG", filepath.Join(tempDir, ".npmrc"))

	// Set up the NpmrcYarnrcManager for npm build tool.
	nm := setupNpmrcManager(project.Npm)

	// Actual username and password for testing.
	username := "myUser"
	password := "myPassword"

	// Expected base64 encoded value. (Base64 encoded "myUser:myPassword")
	expectedAuthValue := "bXlVc2VyOm15UGFzc3dvcmQ="

	// Run the method to handle Basic Auth.
	err := nm.handleNpmrcBasicAuth(username, password)
	assert.NoError(t, err)

	// Read the resulting .npmrc file and verify the auth value.
	npmrcContent, err := os.ReadFile(filepath.Join(tempDir, ".npmrc"))
	assert.NoError(t, err)

	// Verify that the auth key and value are correctly set.
	expectedAuthLine := fmt.Sprintf("//%s/api/npm/my-repo-virtual:_auth=\"%s\"", strings.TrimPrefix(testArtifactoryUrl, "https://"), expectedAuthValue)
	assert.Contains(t, string(npmrcContent), expectedAuthLine)
}

// Test for handling anonymous access in npm.
func TestHandleAnonymousAccess_Npm(t *testing.T) {
	tempNpmrcPath := createTempNpmrc(t)

	nm := setupNpmrcManager(project.Npm)
	// Set basic auth credentials to be removed on the by the anonymous access method.
	assert.NoError(t, nm.handleNpmrcBasicAuth("user", "password"))

	err := nm.handleNpmAnonymousAccess()
	assert.NoError(t, err)

	// Verify that the auth entries were removed from .npmrc
	fileContent, err := os.ReadFile(tempNpmrcPath)
	assert.NoError(t, err)
	assert.NotContains(t, string(fileContent), "_auth")
	assert.NotContains(t, string(fileContent), "_authToken")
}

// Test for handling anonymous access in yarn.
func TestHandleAnonymousAccess_Yarn(t *testing.T) {
	yarnrcPath, restoreYarnrcFunc := createTempYarnrc(t)
	defer func() {
		assert.NoError(t, restoreYarnrcFunc())
	}()

	nm := setupNpmrcManager(project.Yarn)

	err := nm.handleNpmAnonymousAccess()
	assert.NoError(t, err)

	// Verify that the auth entries were removed from .yarnrc
	fileContent, err := os.ReadFile(yarnrcPath)
	assert.NoError(t, err)
	assert.NotContains(t, string(fileContent), "_auth")
	assert.NotContains(t, string(fileContent), "_authToken")
}
