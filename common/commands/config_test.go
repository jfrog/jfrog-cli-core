package commands

import (
	"encoding/json"
	"testing"

	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/utils/log"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetDefaultLogger()
}

func TestBasicAuth(t *testing.T) {
	inputDetails := createTestServerDetails()
	inputDetails.User = "admin"
	inputDetails.Password = "password"

	configAndTest(t, inputDetails, false)
	configAndTest(t, inputDetails, true)
}

func TestUsernameSavedLowercase(t *testing.T) {
	inputDetails := createTestServerDetails()
	inputDetails.User = "ADMIN"
	inputDetails.Password = "password"

	outputConfig, err := configAndGetTestServer(t, inputDetails, false, false)
	assert.NoError(t, err)
	assert.Equal(t, outputConfig.User, "admin", "The config command is supposed to save username as lowercase")
}

func TestArtifactorySshKey(t *testing.T) {
	inputDetails := createTestServerDetails()
	inputDetails.User = "admin"
	inputDetails.Password = "password"
	inputDetails.SshKeyPath = "/tmp/sshKey"
	inputDetails.ArtifactoryUrl = "ssh://localhost:1339/"

	configAndTest(t, inputDetails, false)
	configAndTest(t, inputDetails, true)
}

func TestAccessToken(t *testing.T) {
	inputDetails := createTestServerDetails()
	inputDetails.AccessToken = "accessToken"

	configAndTest(t, inputDetails, false)
	configAndTest(t, inputDetails, true)
}

func TestRefreshToken(t *testing.T) {
	// Import after tokens were generated.
	inputDetails := createTestServerDetails()
	inputDetails.User = "admin"
	inputDetails.Password = "password"
	inputDetails.AccessToken = "accessToken"
	inputDetails.RefreshToken = "refreshToken"

	configAndTest(t, inputDetails, false)
	configAndTest(t, inputDetails, true)

	// Import before tokens were generated.
	inputDetails.AccessToken = ""
	inputDetails.RefreshToken = ""
	configAndTest(t, inputDetails, false)
	configAndTest(t, inputDetails, true)
}

func TestEmptyCredentials(t *testing.T) {
	configAndTest(t, createTestServerDetails(), false)
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
	inputDetails := createTestServerDetails()
	inputDetails.User = "admin"
	inputDetails.Password = "password"

	// Verify setting the option disables refreshable tokens.
	outputConfig, err := configAndGetTestServer(t, inputDetails, true, false)
	assert.NoError(t, err)
	assert.Equal(t, coreutils.TokenRefreshDisabled, outputConfig.TokenRefreshInterval, "expected refreshable token to be disabled")
	assert.NoError(t, DeleteConfig("test"))

	// Verify setting the option enables refreshable tokens.
	outputConfig, err = configAndGetTestServer(t, inputDetails, false, false)
	assert.NoError(t, err)
	assert.Equal(t, coreutils.TokenRefreshDefaultInterval, outputConfig.TokenRefreshInterval, "expected refreshable token to be enabled")
	assert.NoError(t, DeleteConfig("test"))
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
	assert.NoError(t, DeleteConfig("test"))
	testExportImport(t, inputDetails)
}

func configAndGetTestServer(t *testing.T, inputDetails *config.ServerDetails, basicAuthOnly, interactive bool) (*config.ServerDetails, error) {
	configCmd := NewConfigCommand().SetDetails(inputDetails).SetServerId("test").SetUseBasicAuthOnly(basicAuthOnly).SetInteractive(interactive)
	configCmd.disablePromptUrls = true
	assert.NoError(t, configCmd.Config())
	return GetConfig("test", false)
}

func configStructToString(artConfig *config.ServerDetails) string {
	artConfig.IsDefault = false
	marshaledStruct, _ := json.Marshal(*artConfig)
	return string(marshaledStruct)
}

func createTestServerDetails() *config.ServerDetails {
	return &config.ServerDetails{
		Url:               "http://localhost:8080",
		ArtifactoryUrl:    "http://localhost:8080/artifactory",
		DistributionUrl:   "http://localhost:8080/distribution",
		XrayUrl:           "http://localhost:8080/xray",
		MissionControlUrl: "http://localhost:8080/mc",
		PipelinesUrl:      "http://localhost:8080/pipelines",
		ServerId:          "test",
		IsDefault:         false,
		ClientCertPath:    "ClientCertPath", ClientCertKeyPath: "ClientCertKeyPath",
	}
}
