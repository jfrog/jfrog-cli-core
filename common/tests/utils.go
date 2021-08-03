package tests

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	testsutils "github.com/jfrog/jfrog-cli-core/v2/utils/config/tests"
	"testing"
)

func configServer(inputDetails *config.ServerDetails) (err error) {
	//configCmd := commands.NewConfigCommand().SetDetails(inputDetails).SetServerId(serverId).SetUseBasicAuthOnly(basicAuthOnly).SetInteractive(interactive)
	//configCmd.disablePromptUrls = true
	//err = configCmd.Config()

	return config.SaveServersConf([]*config.ServerDetails{inputDetails})
}

func ConfigTestServer(t *testing.T) (err error, cleanUp func()) {
	cleanUp = testsutils.CreateTempEnv(t, false)
	serverDetails := CreateTestServerDetails()
	err = configServer(serverDetails)
	if err != nil {
		return
	}
	return
}

func CreateTestServerDetails() *config.ServerDetails {
	return &config.ServerDetails{
		Url:               "http://localhost:8080/",
		ArtifactoryUrl:    "http://localhost:8080/artifactory/",
		DistributionUrl:   "http://localhost:8080/distribution/",
		XrayUrl:           "http://localhost:8080/xray/",
		MissionControlUrl: "http://localhost:8080/mc/",
		PipelinesUrl:      "http://localhost:8080/pipelines/",
		ServerId:          "test",
		IsDefault:         false,
		ClientCertPath:    "ClientCertPath", ClientCertKeyPath: "ClientCertKeyPath",
	}
}
