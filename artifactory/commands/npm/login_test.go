package npm

import (
	"fmt"
	cmdutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestNpmLoginCommand(t *testing.T) {
	// Create a temporary directory to act as the environment's npmrc file location.
	tempDir := t.TempDir()
	npmrcFilePath := filepath.Join(tempDir, ".npmrc")

	// Set NPM_CONFIG_USERCONFIG to point to the temporary npmrc file path.
	t.Setenv("NPM_CONFIG_USERCONFIG", npmrcFilePath)

	// Initialize a new NpmLoginCommand instance.
	loginCmd := NewNpmLoginCommand()
	loginCmd.SetServerDetails(&config.ServerDetails{ArtifactoryUrl: "https://acme.jfrog.io/artifactory"})
	loginCmd.repo = "npm-virtual"

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
			loginCmd.serverDetails.SetUser(testCase.user)
			loginCmd.serverDetails.SetPassword(testCase.password)
			loginCmd.serverDetails.SetAccessToken(testCase.accessToken)

			// Run the login command and ensure no errors occur.
			assert.NoError(t, loginCmd.Run())

			// Read the contents of the temporary npmrc file.
			npmrcContentBytes, err := os.ReadFile(npmrcFilePath)
			assert.NoError(t, err)
			npmrcContent := string(npmrcContentBytes)

			// Validate that the registry URL was set correctly in .npmrc.
			assert.Contains(t, npmrcContent, fmt.Sprintf("%s=%s", cmdutils.NpmConfigRegistryKey, "https://acme.jfrog.io/artifactory/api/npm/npm-virtual"))

			// Define expected keys for basic and token auth in the .npmrc.
			basicAuthKey := fmt.Sprintf("//acme.jfrog.io/artifactory/api/npm/npm-virtual:%s=", cmdutils.NpmConfigAuthKey)
			tokenAuthKey := fmt.Sprintf("//acme.jfrog.io/artifactory/api/npm/npm-virtual:%s=", cmdutils.NpmConfigAuthTokenKey)

			switch {
			// Validate token-based authentication.
			case testCase.accessToken != "":
				assert.Contains(t, npmrcContent, tokenAuthKey+"test-token")
				assert.NotContains(t, npmrcContent, basicAuthKey)

			// Validate basic authentication with encoded credentials.
			case testCase.user != "" && testCase.password != "":
				// Base64 encoding of "myUser:myPassword"
				expectedBasicAuth := basicAuthKey + "\"bXlVc2VyOm15UGFzc3dvcmQ=\""
				assert.Contains(t, npmrcContent, expectedBasicAuth)
				assert.NotContains(t, npmrcContent, tokenAuthKey)

			// Validate anonymous access, where neither auth method is configured.
			default:
				assert.NotContains(t, npmrcContent, basicAuthKey)
				assert.NotContains(t, npmrcContent, tokenAuthKey)
			}
		})
	}
}
