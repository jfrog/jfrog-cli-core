package yarn

import (
	"fmt"
	cmdutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestYarnLoginCommand(t *testing.T) {
	// Retrieve the home directory and construct the .yarnrc file path.
	homeDir, err := os.UserHomeDir()
	assert.NoError(t, err)
	yarnrcPath := filepath.Join(homeDir, ".yarnrc")

	// Back up the existing .yarnrc file and ensure restoration after the test.
	restoreYarnrcFunc, err := ioutils.BackupFile(yarnrcPath, ".yarnrc.backup")
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, restoreYarnrcFunc())
	}()

	// Create a new YarnLoginCommand instance.
	loginCmd := NewYarnLoginCommand()
	loginCmd.SetServerDetails(&config.ServerDetails{ArtifactoryUrl: "https://acme.jfrog.io/artifactory"})
	loginCmd.repo = "npm-virtual"

	// Define test cases with different authentication methods.
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
			// Configure the command's server details for each test case.
			loginCmd.serverDetails.SetUser(testCase.user)
			loginCmd.serverDetails.SetPassword(testCase.password)
			loginCmd.serverDetails.SetAccessToken(testCase.accessToken)

			// Run the login command and check for errors.
			assert.NoError(t, loginCmd.Run())

			// Read the content of .yarnrc to verify configuration.
			yarnrcContentBytes, err := os.ReadFile(yarnrcPath)
			assert.NoError(t, err)
			yarnrcContent := string(yarnrcContentBytes)

			// Check that the registry URL is correctly set in .yarnrc.
			assert.Contains(t, yarnrcContent, fmt.Sprintf("%s \"%s\"", cmdutils.NpmConfigRegistryKey, "https://acme.jfrog.io/artifactory/api/npm/npm-virtual"))

			// Define expected keys for basic auth and token-based auth in .yarnrc.
			basicAuthKey := fmt.Sprintf("\"//acme.jfrog.io/artifactory/api/npm/npm-virtual:%s\"", cmdutils.NpmConfigAuthKey)
			tokenAuthKey := fmt.Sprintf("\"//acme.jfrog.io/artifactory/api/npm/npm-virtual:%s\"", cmdutils.NpmConfigAuthTokenKey)

			// Verify the .yarnrc file contents according to the authentication method.
			switch {
			case testCase.accessToken != "":
				assert.Contains(t, yarnrcContent, tokenAuthKey+" test-token")
				assert.NotContains(t, yarnrcContent, basicAuthKey)

			case testCase.user != "" && testCase.password != "":
				// Base64 encoding of "myUser:myPassword"
				expectedBasicAuth := basicAuthKey + " bXlVc2VyOm15UGFzc3dvcmQ="
				assert.Contains(t, yarnrcContent, expectedBasicAuth)
				assert.NotContains(t, yarnrcContent, tokenAuthKey)

			default:
				assert.NotContains(t, yarnrcContent, basicAuthKey)
				assert.NotContains(t, yarnrcContent, tokenAuthKey)
			}
		})
	}
}
