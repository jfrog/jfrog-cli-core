package commands

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/common/tests"
	utilsTests "github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/stretchr/testify/assert"
)

const testServerId = "test"

func init() {
	log.SetDefaultLogger()
}

func TestBasicAuth(t *testing.T) {
	inputDetails := tests.CreateTestServerDetails()
	inputDetails.User = "admin"
	inputDetails.Password = "password"

	configAndTest(t, inputDetails, false)
	configAndTest(t, inputDetails, true)
}

func TestUsernameSavedLowercase(t *testing.T) {
	inputDetails := tests.CreateTestServerDetails()
	inputDetails.User = "ADMIN"
	inputDetails.Password = "password"

	outputConfig, err := configAndGetTestServer(t, inputDetails, false, false)
	assert.NoError(t, err)
	assert.Equal(t, outputConfig.User, "admin", "The config command is supposed to save username as lowercase")
}

func TestArtifactorySshKey(t *testing.T) {
	inputDetails := tests.CreateTestServerDetails()
	inputDetails.SshKeyPath = "/tmp/sshKey"
	inputDetails.SshPassphrase = "123456"
	inputDetails.ArtifactoryUrl = "ssh://localhost:1339/"

	configAndTest(t, inputDetails, false)
	configAndTest(t, inputDetails, true)
}

func TestAccessToken(t *testing.T) {
	inputDetails := tests.CreateTestServerDetails()
	inputDetails.AccessToken = "accessToken"

	configAndTest(t, inputDetails, false)
	configAndTest(t, inputDetails, true)
}

func TestAccessTokenWithUsername(t *testing.T) {
	inputDetails := tests.CreateTestServerDetails()
	inputDetails.AccessToken = "accessToken"
	inputDetails.User = "ADMIN"

	configAndTest(t, inputDetails, false)
	configAndTest(t, inputDetails, true)
}

func TestApiKeyInAccessToken(t *testing.T) {
	inputDetails := tests.CreateTestServerDetails()
	apiKey := "AKCp8" + "fsafsadfkljaodjpioqwu4-32742398ujklwertjp89347583jtklsdfmgklsdjuftp397859jsdklfnsljgflkdsjlgjld"
	inputDetails.AccessToken = apiKey

	// Should throw error if access token is API key and no username
	configCmd := NewConfigCommand(AddOrEdit, testServerId).SetDetails(inputDetails).SetUseBasicAuthOnly(true).SetInteractive(false)
	configCmd.disablePrompts = true
	assert.ErrorContains(t, configCmd.Run(), "the provided Access Token is an API key")

	// Should work without error if access token is API key but username exists
	inputDetails.User = "ADMIN"
	configCmd.SetDetails(inputDetails)
	assert.NoError(t, configCmd.Run())
}

func TestMTLS(t *testing.T) {
	inputDetails := tests.CreateTestServerDetails()
	inputDetails.ClientCertPath = "test/cert/path"
	inputDetails.ClientCertKeyPath = "test/cert/key/path"

	configAndTest(t, inputDetails, false)
	configAndTest(t, inputDetails, true)
}

func TestArtifactoryRefreshToken(t *testing.T) {
	// Import after tokens were generated.
	inputDetails := tests.CreateTestServerDetails()
	inputDetails.User = "admin"
	inputDetails.Password = "password"
	inputDetails.AccessToken = "accessToken"
	inputDetails.ArtifactoryRefreshToken = "refreshToken"

	configAndTest(t, inputDetails, false)
	configAndTest(t, inputDetails, true)

	// Import before tokens were generated.
	inputDetails.AccessToken = ""
	inputDetails.ArtifactoryRefreshToken = ""
	configAndTest(t, inputDetails, false)
	configAndTest(t, inputDetails, true)
}

func TestEmptyCredentials(t *testing.T) {
	configAndTest(t, tests.CreateTestServerDetails(), false)
}

func TestUrls(t *testing.T) {
	t.Run("non-interactive", func(t *testing.T) { testUrls(t, false) })
	t.Run("interactive", func(t *testing.T) { testUrls(t, true) })
}

func testUrls(t *testing.T, interactive bool) {
	inputDetails := config.ServerDetails{
		Url: "http://localhost:8080", User: "admin", Password: "password",
		ServerId: testServerId, ClientCertPath: "test/cert/path", ClientCertKeyPath: "test/cert/key/path",
		IsDefault: false}

	outputConfig, err := configAndGetTestServer(t, &inputDetails, false, interactive)
	assert.NoError(t, err)

	assert.Equal(t, "http://localhost:8080/", outputConfig.GetUrl())
	assert.Equal(t, "http://localhost:8080/artifactory/", outputConfig.GetArtifactoryUrl())
	assert.Equal(t, "http://localhost:8080/distribution/", outputConfig.GetDistributionUrl())
	assert.Equal(t, "http://localhost:8080/xray/", outputConfig.GetXrayUrl())
	assert.Equal(t, "http://localhost:8080/mc/", outputConfig.GetMissionControlUrl())
	assert.Equal(t, "http://localhost:8080/pipelines/", outputConfig.GetPipelinesUrl())

	inputDetails.ArtifactoryUrl = "http://localhost:8081/artifactory"
	inputDetails.DistributionUrl = "http://localhost:8081/distribution"
	inputDetails.XrayUrl = "http://localhost:8081/xray"
	inputDetails.MissionControlUrl = "http://localhost:8081/mc"
	inputDetails.PipelinesUrl = "http://localhost:8081/pipelines"
	inputDetails.AccessUrl = "http://localhost:8081/access"

	outputConfig, err = configAndGetTestServer(t, &inputDetails, false, interactive)
	assert.NoError(t, err)

	assert.Equal(t, "http://localhost:8080/", outputConfig.GetUrl())
	assert.Equal(t, "http://localhost:8081/artifactory/", outputConfig.GetArtifactoryUrl())
	assert.Equal(t, "http://localhost:8081/distribution/", outputConfig.GetDistributionUrl())
	assert.Equal(t, "http://localhost:8081/xray/", outputConfig.GetXrayUrl())
	assert.Equal(t, "http://localhost:8081/mc/", outputConfig.GetMissionControlUrl())
	assert.Equal(t, "http://localhost:8081/pipelines/", outputConfig.GetPipelinesUrl())
}

func TestBasicAuthOnlyOption(t *testing.T) {
	inputDetails := tests.CreateTestServerDetails()
	inputDetails.User = "admin"
	inputDetails.Password = "password"

	// Verify setting the option disables refreshable tokens.
	outputConfig, err := configAndGetTestServer(t, inputDetails, true, false)
	assert.NoError(t, err)
	assert.Equal(t, coreutils.TokenRefreshDisabled, outputConfig.ArtifactoryTokenRefreshInterval, "expected refreshable token to be disabled")
	assert.NoError(t, NewConfigCommand(Delete, testServerId).Run())

	// Verify setting the option enables refreshable tokens.
	outputConfig, err = configAndGetTestServer(t, inputDetails, false, false)
	assert.NoError(t, err)
	assert.Equal(t, coreutils.TokenRefreshDefaultInterval, outputConfig.ArtifactoryTokenRefreshInterval, "expected refreshable token to be enabled")
	assert.NoError(t, NewConfigCommand(Delete, testServerId).Run())
}

func TestMakeDefaultOption(t *testing.T) {
	originalDefault := tests.CreateTestServerDetails()
	originalDefault.ServerId = "originalDefault"
	originalDefault.IsDefault = false
	newDefault := tests.CreateTestServerDetails()
	newDefault.ServerId = "newDefault"
	newDefault.IsDefault = false

	// Config the first server, and expect it to be default because it is the only server.
	configAndAssertDefault(t, originalDefault, false)
	defer deleteServer(t, originalDefault.ServerId)

	// Config a second server and pass the makeDefault option.
	configAndAssertDefault(t, newDefault, true)
	defer deleteServer(t, newDefault.ServerId)

}

func configAndAssertDefault(t *testing.T, inputDetails *config.ServerDetails, makeDefault bool) {
	outputConfig, err := configAndGetServer(t, inputDetails.ServerId, inputDetails, false, false, makeDefault)
	assert.NoError(t, err)
	assert.Equal(t, inputDetails.ServerId, outputConfig.ServerId)
	assert.True(t, outputConfig.IsDefault)
}

func deleteServer(t *testing.T, serverId string) {
	assert.NoError(t, NewConfigCommand(Delete, serverId).Run())
}

type unsafeUrlTest struct {
	url    string
	isSafe bool
}

var unsafeUrlTestCases = []unsafeUrlTest{
	// Safe URLs
	{"https://acme.jfrog.io", true},
	{"http://127.0.0.1", true},
	{"http://localhost", true},
	{"http://127.0.0.1:8081", true},
	{"http://localhost:8081", true},
	{"ssh://localhost:1339/", true},

	// Unsafe URLs:
	{"http://acme.jfrog.io", false},
	{"http://acme.jfrog.io:8081", false},
	{"http://localhost-123", false},
}

func TestAssertUrlsSafe(t *testing.T) {
	for _, testCase := range unsafeUrlTestCases {
		t.Run(testCase.url, func(t *testing.T) {
			// Test non-interactive - should pass with a warning message
			inputDetails := &config.ServerDetails{Url: testCase.url, ServerId: testServerId}
			configAndTest(t, inputDetails, false)

			// Test interactive - should fail with an error
			configCmd := NewConfigCommand(AddOrEdit, testServerId).SetDetails(inputDetails).SetInteractive(true)
			configCmd.disablePrompts = true
			err := configCmd.Run()
			if testCase.isSafe {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, "config was aborted due to an insecure HTTP connection")
			}
		})
	}
}

func TestExportEmptyConfig(t *testing.T) {
	cliHome, exist := os.LookupEnv(coreutils.HomeDir)
	defer func() {
		if exist {
			assert.NoError(t, os.Setenv(coreutils.HomeDir, cliHome))
		} else {
			assert.NoError(t, os.Unsetenv(coreutils.HomeDir))
		}
	}()
	tempDirPath, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, fileutils.RemoveTempDir(tempDirPath), "Couldn't remove temp dir")
	}()
	assert.NoError(t, os.Setenv(coreutils.HomeDir, tempDirPath))
	assert.Error(t, Export(""))
}

func TestKeyEncryption(t *testing.T) {
	cleanUpJfrogHome, err := utilsTests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	assert.NoError(t, os.Setenv(coreutils.EncryptionKey, "p3aNuTbUtt3rJ3lly&ChEEsEPlEasE!!"))
	defer func() {
		assert.NoError(t, os.Unsetenv(coreutils.EncryptionKey))
	}()
	inputDetails := tests.CreateTestServerDetails()
	inputDetails.User = "admin"
	inputDetails.Password = "password"

	configAndTest(t, inputDetails, true)
	configAndTest(t, inputDetails, false)
}

func TestKeyDecryptionError(t *testing.T) {
	cleanUpJfrogHome, err := utilsTests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	assert.NoError(t, os.Setenv(coreutils.EncryptionKey, "p3aNuTbUtt3rJ3lly&ChEEsEPlEasE!!"))
	defer func() {
		assert.NoError(t, os.Unsetenv(coreutils.EncryptionKey))
	}()

	inputDetails := tests.CreateTestServerDetails()
	inputDetails.User = "admin"
	inputDetails.Password = "password"

	// Configure server with JFROG_CLI_ENCRYPTION_KEY set
	configCmd := NewConfigCommand(AddOrEdit, testServerId).SetDetails(inputDetails).SetUseBasicAuthOnly(true).SetInteractive(false)
	configCmd.disablePrompts = true
	assert.NoError(t, configCmd.Run())

	// Get the server details when JFROG_CLI_ENCRYPTION_KEY is not set and expect an error
	assert.NoError(t, os.Unsetenv(coreutils.EncryptionKey))
	_, err = GetConfig(testServerId, false)
	assert.ErrorContains(t, err, "cannot decrypt config")
}

func testExportImport(t *testing.T, inputDetails *config.ServerDetails) {
	configToken, err := config.Export(inputDetails)
	assert.NoError(t, err)
	outputDetails, err := config.Import(configToken)
	assert.NoError(t, err)
	assert.Equal(t, configStructToString(t, inputDetails), configStructToString(t, outputDetails), "unexpected configuration was saved to file")
}

func configAndTest(t *testing.T, inputDetails *config.ServerDetails, interactive bool) {
	outputConfig, err := configAndGetTestServer(t, inputDetails, true, interactive)
	assert.NoError(t, err)
	assert.Equal(t, configStructToString(t, inputDetails), configStructToString(t, outputConfig), "unexpected configuration was saved to file")
	assert.NoError(t, NewConfigCommand(Delete, testServerId).Run())
	testExportImport(t, inputDetails)
}

func configAndGetTestServer(t *testing.T, inputDetails *config.ServerDetails, basicAuthOnly, interactive bool) (*config.ServerDetails, error) {
	return configAndGetServer(t, testServerId, inputDetails, basicAuthOnly, interactive, false)
}

func configAndGetServer(t *testing.T, serverId string, inputDetails *config.ServerDetails, basicAuthOnly, interactive, makeDefault bool) (*config.ServerDetails, error) {
	configCmd := NewConfigCommand(AddOrEdit, serverId).SetDetails(inputDetails).SetUseBasicAuthOnly(basicAuthOnly).
		SetInteractive(interactive).SetMakeDefault(makeDefault)
	configCmd.disablePrompts = true
	assert.NoError(t, configCmd.Run())
	return GetConfig(serverId, false)
}

func configStructToString(t *testing.T, artConfig *config.ServerDetails) string {
	artConfig.IsDefault = false
	marshaledStruct, err := json.Marshal(*artConfig)
	assert.NoError(t, err)
	return string(marshaledStruct)
}
