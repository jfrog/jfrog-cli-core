package tests

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	testsutils "github.com/jfrog/jfrog-cli-core/v2/utils/config/tests"
	"testing"
)

func ConfigTestServer(t *testing.T) (cleanUp func(), err error) {
	cleanUp = testsutils.CreateTempEnv(t, false)
	serverDetails := CreateTestServerDetails()
	err = config.SaveServersConf([]*config.ServerDetails{serverDetails})
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
		IsDefault:         true,
		ClientCertPath:    "ClientCertPath", ClientCertKeyPath: "ClientCertKeyPath",
	}
}
