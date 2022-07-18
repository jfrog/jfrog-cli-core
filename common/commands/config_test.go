package commands

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"os"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/stretchr/testify/assert"
)

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
		ServerId: "test", ClientCertPath: "test/cert/path", ClientCertKeyPath: "test/cert/key/path",
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
	assert.NoError(t, NewConfigCommand(Delete, "test").Run())

	// Verify setting the option enables refreshable tokens.
	outputConfig, err = configAndGetTestServer(t, inputDetails, false, false)
	assert.NoError(t, err)
	assert.Equal(t, coreutils.TokenRefreshDefaultInterval, outputConfig.ArtifactoryTokenRefreshInterval, "expected refreshable token to be enabled")
	assert.NoError(t, NewConfigCommand(Delete, "test").Run())
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
	defer assert.NoError(t, fileutils.RemoveTempDir(tempDirPath), "Couldn't remove temp dir")
	assert.NoError(t, os.Setenv(coreutils.HomeDir, tempDirPath))
	assert.Error(t, Export(""))
}

func testExportImport(t *testing.T, inputDetails *config.ServerDetails) {
	serverToken, err := config.Export(inputDetails)
	assert.NoError(t, err)
	outputDetails, err := config.Import(serverToken)
	assert.NoError(t, err)
	assert.Equal(t, configStructToString(inputDetails), configStructToString(outputDetails), "unexpected configuration was saved to file")
}

func configAndTest(t *testing.T, inputDetails *config.ServerDetails, interactive bool) {
	outputConfig, err := configAndGetTestServer(t, inputDetails, true, interactive)
	assert.NoError(t, err)
	assert.Equal(t, configStructToString(inputDetails), configStructToString(outputConfig), "unexpected configuration was saved to file")
	assert.NoError(t, NewConfigCommand(Delete, "test").Run())
	testExportImport(t, inputDetails)
}

func configAndGetTestServer(t *testing.T, inputDetails *config.ServerDetails, basicAuthOnly, interactive bool) (*config.ServerDetails, error) {
	configCmd := NewConfigCommand(AddOrEdit, "test").SetDetails(inputDetails).SetUseBasicAuthOnly(basicAuthOnly).SetInteractive(interactive)
	configCmd.disablePromptUrls = true
	assert.NoError(t, configCmd.Run())
	return GetConfig("test", false)
}

func configStructToString(artConfig *config.ServerDetails) string {
	artConfig.IsDefault = false
	marshaledStruct, _ := json.Marshal(*artConfig)
	return string(marshaledStruct)
}
