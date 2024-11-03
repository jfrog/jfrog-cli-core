package buildtoollogin

import (
	"fmt"
	cmdutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

// #nosec G101 -- Dummy token for tests
var dummyToken = "eyJ2ZXIiOiIyIiwidHlwIjoiSldUIiwiYWxnIjoiUlMyNTYiLCJraWQiOiJIcnU2VHctZk1yOTV3dy12TDNjV3ZBVjJ3Qm9FSHpHdGlwUEFwOE1JdDljIn0.eyJzdWIiOiJqZnJ0QDAxYzNnZmZoZzJlOHc2MTQ5ZTNhMnEwdzk3XC91c2Vyc1wvYWRtaW4iLCJzY3AiOiJtZW1iZXItb2YtZ3JvdXBzOnJlYWRlcnMgYXBpOioiLCJhdWQiOiJqZnJ0QDAxYzNnZmZoZzJlOHc2MTQ5ZTNhMnEwdzk3IiwiaXNzIjoiamZydEAwMWMzZ2ZmaGcyZTh3NjE0OWUzYTJxMHc5NyIsImV4cCI6MTU1NjAzNzc2NSwiaWF0IjoxNTU2MDM0MTY1LCJqdGkiOiI1M2FlMzgyMy05NGM3LTQ0OGItOGExOC1iZGVhNDBiZjFlMjAifQ.Bp3sdvppvRxysMlLgqT48nRIHXISj9sJUCXrm7pp8evJGZW1S9hFuK1olPmcSybk2HNzdzoMcwhUmdUzAssiQkQvqd_HanRcfFbrHeg5l1fUQ397ECES-r5xK18SYtG1VR7LNTVzhJqkmRd3jzqfmIK2hKWpEgPfm8DRz3j4GGtDRxhb3oaVsT2tSSi_VfT3Ry74tzmO0GcCvmBE2oh58kUZ4QfEsalgZ8IpYHTxovsgDx_M7ujOSZx_hzpz-iy268-OkrU22PQPCfBmlbEKeEUStUO9n0pj4l1ODL31AGARyJRy46w4yzhw7Fk5P336WmDMXYs5LAX2XxPFNLvNzA"

func createTestBuildToolLoginCommand(buildTool project.ProjectType) *BuildToolLoginCommand {
	cmd := NewBuildToolLoginCommand(buildTool)
	cmd.repoName = "test-repo"
	cmd.serverDetails = &config.ServerDetails{ArtifactoryUrl: "https://acme.jfrog.io/artifactory"}

	return cmd
}

func TestBuildToolLoginCommand_Npm(t *testing.T) {
	// Create a temporary directory to act as the environment's npmrc file location.
	tempDir := t.TempDir()
	npmrcFilePath := filepath.Join(tempDir, ".npmrc")

	// Set NPM_CONFIG_USERCONFIG to point to the temporary npmrc file path.
	t.Setenv("NPM_CONFIG_USERCONFIG", npmrcFilePath)

	npmLoginCmd := createTestBuildToolLoginCommand(project.Npm)

	// Define test cases for different authentication types.
	testCases := []struct {
		name        string
		user        string
		password    string
		accessToken string
	}{
		{
			name:        "Token Authentication",
			accessToken: dummyToken,
		},
		{
			name:     "Basic Authentication",
			user:     "myUser",
			password: "myPassword",
		},
		{
			name: "Anonymous Access",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Set up server details for the current test case's authentication type.
			npmLoginCmd.serverDetails.SetUser(testCase.user)
			npmLoginCmd.serverDetails.SetPassword(testCase.password)
			npmLoginCmd.serverDetails.SetAccessToken(testCase.accessToken)

			// Run the login command and ensure no errors occur.
			if npmLoginCmd.Run() != nil {
				t.FailNow()
			}

			// Read the contents of the temporary npmrc file.
			npmrcContentBytes, err := os.ReadFile(npmrcFilePath)
			assert.NoError(t, err)
			npmrcContent := string(npmrcContentBytes)

			// Validate that the registry URL was set correctly in .npmrc.
			assert.Contains(t, npmrcContent, fmt.Sprintf("%s=%s", cmdutils.NpmConfigRegistryKey, "https://acme.jfrog.io/artifactory/api/npm/test-repo"))

			// Define expected keys for basic and token auth in the .npmrc.
			basicAuthKey := fmt.Sprintf("//acme.jfrog.io/artifactory/api/npm/test-repo:%s=", cmdutils.NpmConfigAuthKey)
			tokenAuthKey := fmt.Sprintf("//acme.jfrog.io/artifactory/api/npm/test-repo:%s=", cmdutils.NpmConfigAuthTokenKey)

			// Validate token-based authentication.
			if testCase.accessToken != "" {
				assert.Contains(t, npmrcContent, tokenAuthKey+"test-token")
			} else if testCase.user != "" && testCase.password != "" {
				// Validate basic authentication with encoded credentials.
				// Base64 encoding of "myUser:myPassword"
				expectedBasicAuth := basicAuthKey + "\"bXlVc2VyOm15UGFzc3dvcmQ=\""
				assert.Contains(t, npmrcContent, expectedBasicAuth)
			}

			// Clean up the temporary npmrc file.
			assert.NoError(t, os.Remove(npmrcFilePath))
		})
	}
}

func TestBuildToolLoginCommand_Yarn(t *testing.T) {
	// Retrieve the home directory and construct the .yarnrc file path.
	homeDir, err := os.UserHomeDir()
	assert.NoError(t, err)
	yarnrcFilePath := filepath.Join(homeDir, ".yarnrc")

	// Back up the existing .yarnrc file and ensure restoration after the test.
	restoreYarnrcFunc, err := ioutils.BackupFile(yarnrcFilePath, ".yarnrc.backup")
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, restoreYarnrcFunc())
	}()

	yarnLoginCmd := createTestBuildToolLoginCommand(project.Yarn)

	// Define test cases for different authentication types.
	testCases := []struct {
		name        string
		user        string
		password    string
		accessToken string
	}{
		{
			name:        "Token Authentication",
			accessToken: "test-token",
		},
		{
			name:     "Basic Authentication",
			user:     "myUser",
			password: "myPassword",
		},
		{
			name: "Anonymous Access",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Set up server details for the current test case's authentication type.
			yarnLoginCmd.serverDetails.SetUser(testCase.user)
			yarnLoginCmd.serverDetails.SetPassword(testCase.password)
			yarnLoginCmd.serverDetails.SetAccessToken(testCase.accessToken)

			// Run the login command and ensure no errors occur.
			if yarnLoginCmd.Run() != nil {
				t.FailNow()
			}

			// Read the contents of the temporary npmrc file.
			yarnrcContentBytes, err := os.ReadFile(yarnrcFilePath)
			assert.NoError(t, err)
			yarnrcContent := string(yarnrcContentBytes)

			// Check that the registry URL is correctly set in .yarnrc.
			assert.Contains(t, yarnrcContent, fmt.Sprintf("%s \"%s\"", cmdutils.NpmConfigRegistryKey, "https://acme.jfrog.io/artifactory/api/npm/test-repo"))

			// Define expected keys for basic auth and token-based auth in .yarnrc.
			basicAuthKey := fmt.Sprintf("\"//acme.jfrog.io/artifactory/api/npm/test-repo:%s\" ", cmdutils.NpmConfigAuthKey)
			tokenAuthKey := fmt.Sprintf("\"//acme.jfrog.io/artifactory/api/npm/test-repo:%s\" ", cmdutils.NpmConfigAuthTokenKey)

			// Validate token-based authentication.
			if testCase.accessToken != "" {
				assert.Contains(t, yarnrcContent, tokenAuthKey+"test-token")

			} else if testCase.user != "" && testCase.password != "" {
				// Validate basic authentication with encoded credentials.
				// Base64 encoding of "myUser:myPassword"
				expectedBasicAuth := basicAuthKey + "bXlVc2VyOm15UGFzc3dvcmQ="
				assert.Contains(t, yarnrcContent, expectedBasicAuth)
			}

			// Clean up the temporary npmrc file.
			assert.NoError(t, os.Remove(yarnrcFilePath))
		})
	}
}

func TestBuildToolLoginCommand_Pip(t *testing.T) {
	// Retrieve the home directory and construct the pip.conf file path.
	homeDir, err := os.UserHomeDir()
	assert.NoError(t, err)
	var pipConfFilePath string
	if coreutils.IsWindows() {
		pipConfFilePath = filepath.Join(homeDir, "pip", "pip.ini")
	} else {
		pipConfFilePath = filepath.Join(homeDir, ".config", "pip", "pip.conf")
	}

	// Back up the existing .pip.conf file and ensure restoration after the test.
	restorePipConfFunc, err := ioutils.BackupFile(pipConfFilePath, ".pipconf.backup")
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, restorePipConfFunc())
	}()

	pipLoginCmd := createTestBuildToolLoginCommand(project.Pip)

	// Define test cases for different authentication types.
	testCases := []struct {
		name        string
		user        string
		password    string
		accessToken string
	}{
		{
			name:        "Token Authentication",
			accessToken: "test-token",
		},
		{
			name:     "Basic Authentication",
			user:     "myUser",
			password: "myPassword",
		},
		{
			name: "Anonymous Access",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Clean up the temporary pip config file.
			assert.NoError(t, os.Remove(pipConfFilePath))

			// Set up server details for the current test case's authentication type.
			pipLoginCmd.serverDetails.SetUser(testCase.user)
			pipLoginCmd.serverDetails.SetPassword(testCase.password)
			pipLoginCmd.serverDetails.SetAccessToken(testCase.accessToken)

			// Run the login command and ensure no errors occur.
			if pipLoginCmd.Run() != nil {
				t.FailNow()
			}

			// Read the contents of the temporary pip config file.
			pipConfigContentBytes, err := os.ReadFile(pipConfFilePath)
			assert.NoError(t, err)
			pipConfigContent := string(pipConfigContentBytes)

			// Validate that the index URL was set correctly in pip.conf.
			assert.Contains(t, pipConfigContent, fmt.Sprintf("index-url = https://%s:%s@acme.jfrog.io/artifactory/api/pypi/test-repo/simple", testCase.user, testCase.password))

			// Validate token-based authentication.
			if testCase.accessToken != "" {
				assert.Contains(t, pipConfigContent, fmt.Sprintf("index-url = https://%s@acme.jfrog.io/artifactory/api/pypi/test-repo/simple", "test-token"))
			} else if testCase.user != "" && testCase.password != "" {
				// Validate basic authentication with user and password.
				assert.Contains(t, pipConfigContent, fmt.Sprintf("index-url = https://%s:%s@acme.jfrog.io/artifactory/api/pypi/test-repo/simple", "myUser", "myPassword"))
			}
		})
	}
}

func TestBuildToolLoginCommand_configurePipenv(t *testing.T) {
	// todo
}

func TestBuildToolLoginCommand_configurePoetry(t *testing.T) {
	// todo
}
