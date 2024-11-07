package buildtoollogin

import (
	"fmt"
	cmdutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/io"
	clientTestUtils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// #nosec G101 -- Dummy token for tests
var dummyToken = "eyJ2ZXIiOiIyIiwidHlwIjoiSldUIiwiYWxnIjoiUlMyNTYiLCJraWQiOiJIcnU2VHctZk1yOTV3dy12TDNjV3ZBVjJ3Qm9FSHpHdGlwUEFwOE1JdDljIn0.eyJzdWIiOiJqZnJ0QDAxYzNnZmZoZzJlOHc2MTQ5ZTNhMnEwdzk3XC91c2Vyc1wvYWRtaW4iLCJzY3AiOiJtZW1iZXItb2YtZ3JvdXBzOnJlYWRlcnMgYXBpOioiLCJhdWQiOiJqZnJ0QDAxYzNnZmZoZzJlOHc2MTQ5ZTNhMnEwdzk3IiwiaXNzIjoiamZydEAwMWMzZ2ZmaGcyZTh3NjE0OWUzYTJxMHc5NyIsImV4cCI6MTU1NjAzNzc2NSwiaWF0IjoxNTU2MDM0MTY1LCJqdGkiOiI1M2FlMzgyMy05NGM3LTQ0OGItOGExOC1iZGVhNDBiZjFlMjAifQ.Bp3sdvppvRxysMlLgqT48nRIHXISj9sJUCXrm7pp8evJGZW1S9hFuK1olPmcSybk2HNzdzoMcwhUmdUzAssiQkQvqd_HanRcfFbrHeg5l1fUQ397ECES-r5xK18SYtG1VR7LNTVzhJqkmRd3jzqfmIK2hKWpEgPfm8DRz3j4GGtDRxhb3oaVsT2tSSi_VfT3Ry74tzmO0GcCvmBE2oh58kUZ4QfEsalgZ8IpYHTxovsgDx_M7ujOSZx_hzpz-iy268-OkrU22PQPCfBmlbEKeEUStUO9n0pj4l1ODL31AGARyJRy46w4yzhw7Fk5P336WmDMXYs5LAX2XxPFNLvNzA"

var testCases = []struct {
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

func createTestBuildToolLoginCommand(buildTool project.ProjectType) *BuildToolLoginCommand {
	cmd := NewBuildToolLoginCommand(buildTool)
	cmd.repoName = "test-repo"
	dummyUrl := "https://acme.jfrog.io"
	cmd.serverDetails = &config.ServerDetails{Url: dummyUrl, ArtifactoryUrl: dummyUrl + "/artifactory"}

	return cmd
}

func TestBuildToolLoginCommand_Npm(t *testing.T) {
	// Create a temporary directory to act as the environment's npmrc file location.
	tempDir := t.TempDir()
	npmrcFilePath := filepath.Join(tempDir, ".npmrc")

	// Set NPM_CONFIG_USERCONFIG to point to the temporary npmrc file path.
	t.Setenv("NPM_CONFIG_USERCONFIG", npmrcFilePath)

	npmLoginCmd := createTestBuildToolLoginCommand(project.Npm)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Set up server details for the current test case's authentication type.
			npmLoginCmd.serverDetails.SetUser(testCase.user)
			npmLoginCmd.serverDetails.SetPassword(testCase.password)
			npmLoginCmd.serverDetails.SetAccessToken(testCase.accessToken)

			// Run the login command and ensure no errors occur.
			if !assert.NoError(t, npmLoginCmd.Run()) {
				t.FailNow()
			}

			// Read the contents of the temporary npmrc file.
			npmrcContentBytes, err := os.ReadFile(npmrcFilePath)
			assert.NoError(t, err)
			npmrcContent := string(npmrcContentBytes)

			// Validate that the registry URL was set correctly in .npmrc.
			assert.Contains(t, npmrcContent, fmt.Sprintf("%s=%s", cmdutils.NpmConfigRegistryKey, "https://acme.jfrog.io/artifactory/api/npm/test-repo"))

			// Validate token-based authentication.
			if testCase.accessToken != "" {
				assert.Contains(t, npmrcContent, fmt.Sprintf("//acme.jfrog.io/artifactory/api/npm/test-repo:%s=%s", cmdutils.NpmConfigAuthTokenKey, dummyToken))
			} else if testCase.user != "" && testCase.password != "" {
				// Validate basic authentication with encoded credentials.
				// Base64 encoding of "myUser:myPassword"
				expectedBasicAuth := fmt.Sprintf("//acme.jfrog.io/artifactory/api/npm/test-repo:%s=\"bXlVc2VyOm15UGFzc3dvcmQ=\"", cmdutils.NpmConfigAuthKey)
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

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Set up server details for the current test case's authentication type.
			yarnLoginCmd.serverDetails.SetUser(testCase.user)
			yarnLoginCmd.serverDetails.SetPassword(testCase.password)
			yarnLoginCmd.serverDetails.SetAccessToken(testCase.accessToken)

			// Run the login command and ensure no errors occur.
			if !assert.NoError(t, yarnLoginCmd.Run()) {
				t.FailNow()
			}

			// Read the contents of the temporary npmrc file.
			yarnrcContentBytes, err := os.ReadFile(yarnrcFilePath)
			assert.NoError(t, err)
			yarnrcContent := string(yarnrcContentBytes)

			// Check that the registry URL is correctly set in .yarnrc.
			assert.Contains(t, yarnrcContent, fmt.Sprintf("%s \"%s\"", cmdutils.NpmConfigRegistryKey, "https://acme.jfrog.io/artifactory/api/npm/test-repo"))

			// Validate token-based authentication.
			if testCase.accessToken != "" {
				assert.Contains(t, yarnrcContent, fmt.Sprintf("\"//acme.jfrog.io/artifactory/api/npm/test-repo:%s\" %s", cmdutils.NpmConfigAuthTokenKey, dummyToken))

			} else if testCase.user != "" && testCase.password != "" {
				// Validate basic authentication with encoded credentials.
				// Base64 encoding of "myUser:myPassword"
				assert.Contains(t, yarnrcContent, fmt.Sprintf("\"//acme.jfrog.io/artifactory/api/npm/test-repo:%s\" bXlVc2VyOm15UGFzc3dvcmQ=", cmdutils.NpmConfigAuthKey))
			}

			// Clean up the temporary npmrc file.
			assert.NoError(t, os.Remove(yarnrcFilePath))
		})
	}
}

func TestBuildToolLoginCommand_Pip(t *testing.T) {
	// pip and pipenv share the same configuration file.
	testBuildToolLoginCommandPip(t, project.Pip)
}

func TestBuildToolLoginCommand_Pipenv(t *testing.T) {
	// pip and pipenv share the same configuration file.
	testBuildToolLoginCommandPip(t, project.Pipenv)
}

func testBuildToolLoginCommandPip(t *testing.T, buildTool project.ProjectType) {
	var pipConfFilePath string
	if coreutils.IsWindows() {
		pipConfFilePath = filepath.Join(os.Getenv("APPDATA"), "pip", "pip.ini")
	} else {
		// Retrieve the home directory and construct the pip.conf file path.
		homeDir, err := os.UserHomeDir()
		assert.NoError(t, err)
		pipConfFilePath = filepath.Join(homeDir, ".config", "pip", "pip.conf")
	}
	// Back up the existing .pip.conf file and ensure restoration after the test.
	restorePipConfFunc, err := ioutils.BackupFile(pipConfFilePath, ".pipconf.backup")
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, restorePipConfFunc())
	}()

	pipLoginCmd := createTestBuildToolLoginCommand(buildTool)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Set up server details for the current test case's authentication type.
			pipLoginCmd.serverDetails.SetUser(testCase.user)
			pipLoginCmd.serverDetails.SetPassword(testCase.password)
			pipLoginCmd.serverDetails.SetAccessToken(testCase.accessToken)

			// Run the login command and ensure no errors occur.
			if !assert.NoError(t, pipLoginCmd.Run()) {
				t.FailNow()
			}

			// Read the contents of the temporary pip config file.
			pipConfigContentBytes, err := os.ReadFile(pipConfFilePath)
			assert.NoError(t, err)
			pipConfigContent := string(pipConfigContentBytes)

			switch {
			case testCase.accessToken != "":
				// Validate token-based authentication.
				assert.Contains(t, pipConfigContent, fmt.Sprintf("index-url = https://%s:%s@acme.jfrog.io/artifactory/api/pypi/test-repo/simple", auth.ExtractUsernameFromAccessToken(testCase.accessToken), testCase.accessToken))
			case testCase.user != "" && testCase.password != "":
				// Validate basic authentication with user and password.
				assert.Contains(t, pipConfigContent, fmt.Sprintf("index-url = https://%s:%s@acme.jfrog.io/artifactory/api/pypi/test-repo/simple", "myUser", "myPassword"))
			default:
				// Validate anonymous access.
				assert.Contains(t, pipConfigContent, "index-url = https://acme.jfrog.io/artifactory/api/pypi/test-repo/simple")
			}

			// Clean up the temporary pip config file.
			assert.NoError(t, os.Remove(pipConfFilePath))
		})
	}
}

func TestBuildToolLoginCommand_configurePoetry(t *testing.T) {
	// Retrieve the home directory and construct the .yarnrc file path.
	homeDir, err := os.UserHomeDir()
	assert.NoError(t, err)
	var poetryConfigDir string
	switch {
	case io.IsWindows():
		poetryConfigDir = filepath.Join(homeDir, "AppData", "Roaming")
	case io.IsMacOS():
		poetryConfigDir = filepath.Join(homeDir, "Library", "Application Support")
	default:
		poetryConfigDir = filepath.Join(homeDir, ".config")
	}

	poetryConfigFilePath := filepath.Join(poetryConfigDir, "pypoetry", "config.toml")
	// Poetry stores the auth in a separate file
	poetryAuthFilePath := filepath.Join(poetryConfigDir, "pypoetry", "auth.toml")

	// Back up the existing config.toml and auth.toml files and ensure restoration after the test.
	restorePoetryConfigFunc, err := ioutils.BackupFile(poetryConfigFilePath, ".poetry.config.backup")
	assert.NoError(t, err)
	restorePoetryAuthFunc, err := ioutils.BackupFile(poetryAuthFilePath, ".poetry-auth.backup")
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, restorePoetryConfigFunc())
		assert.NoError(t, restorePoetryAuthFunc())
	}()

	poetryLoginCmd := createTestBuildToolLoginCommand(project.Poetry)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Set up server details for the current test case's authentication type.
			poetryLoginCmd.serverDetails.SetUser(testCase.user)
			poetryLoginCmd.serverDetails.SetPassword(testCase.password)
			poetryLoginCmd.serverDetails.SetAccessToken(testCase.accessToken)

			// Run the login command and ensure no errors occur.
			if !assert.NoError(t, poetryLoginCmd.Run()) {
				t.FailNow()
			}

			// Validate that the repository URL was set correctly in config.toml.
			// Read the contents of the temporary Poetry config file.
			poetryConfigContentBytes, err := os.ReadFile(poetryConfigFilePath)
			assert.NoError(t, err)
			poetryConfigContent := string(poetryConfigContentBytes)
			// Normalize line endings for comparison.(For Windows)
			poetryConfigContent = strings.ReplaceAll(poetryConfigContent, "\r\n", "\n")

			assert.Contains(t, poetryConfigContent, "[repositories.test-repo]\nurl = \"https://acme.jfrog.io/artifactory/api/pypi/test-repo/simple\"")

			// Validate that the auth details were set correctly in auth.toml.
			// Read the contents of the temporary Poetry config file.
			poetryAuthContentBytes, err := os.ReadFile(poetryAuthFilePath)
			assert.NoError(t, err)
			poetryAuthContent := string(poetryAuthContentBytes)
			// Normalize line endings for comparison.(For Windows)
			poetryAuthContent = strings.ReplaceAll(poetryAuthContent, "\r\n", "\n")

			if testCase.accessToken != "" {
				// Validate token-based authentication (The token is stored in the keyring so we can't test it)
				assert.Contains(t, poetryAuthContent, fmt.Sprintf("[http-basic.test-repo]\nusername = \"%s\"", auth.ExtractUsernameFromAccessToken(testCase.accessToken)))
			} else if testCase.user != "" && testCase.password != "" {
				// Validate basic authentication with user and password. (The password is stored in the keyring so we can't test it)
				assert.Contains(t, poetryAuthContent, fmt.Sprintf("[http-basic.test-repo]\nusername = \"%s\"", "myUser"))
			}

			// Clean up the temporary Poetry config files.
			assert.NoError(t, os.Remove(poetryConfigFilePath))
			assert.NoError(t, os.Remove(poetryAuthFilePath))
		})
	}
}

func TestBuildToolLoginCommand_Go(t *testing.T) {
	goProxyEnv := "GOPROXY"
	// Restore the original value of the GOPROXY environment variable after the test.
	restoreGoProxy := clientTestUtils.SetEnvWithCallbackAndAssert(t, goProxyEnv, "")
	defer restoreGoProxy()

	// Assuming createTestBuildToolLoginCommand initializes your Go login command
	goLoginCmd := createTestBuildToolLoginCommand(project.Go)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Set up server details for the current test case's authentication type.
			goLoginCmd.serverDetails.SetUser(testCase.user)
			goLoginCmd.serverDetails.SetPassword(testCase.password)
			goLoginCmd.serverDetails.SetAccessToken(testCase.accessToken)

			// Run the login command and ensure no errors occur.
			if !assert.NoError(t, goLoginCmd.Run()) {
				t.FailNow()
			}

			// Get the value of the GOPROXY environment variable.
			outputBytes, err := exec.Command("go", "env", "GOPROXY").Output()
			assert.NoError(t, err)
			goProxy := string(outputBytes)

			switch {
			case testCase.accessToken != "":
				// Validate token-based authentication.
				assert.Contains(t, goProxy, fmt.Sprintf("https://%s:%s@acme.jfrog.io/artifactory/api/go/test-repo", auth.ExtractUsernameFromAccessToken(testCase.accessToken), testCase.accessToken))
			case testCase.user != "" && testCase.password != "":
				// Validate basic authentication with user and password.
				assert.Contains(t, goProxy, fmt.Sprintf("https://%s:%s@acme.jfrog.io/artifactory/api/go/test-repo", "myUser", "myPassword"))
			default:
				// Validate anonymous access.
				assert.Contains(t, goProxy, "https://acme.jfrog.io/artifactory/api/go/test-repo")
			}
		})
	}
}
