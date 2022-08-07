package tests

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	testsutils "github.com/jfrog/jfrog-cli-core/v2/utils/config/tests"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
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

type restsTestHandler func(w http.ResponseWriter, r *http.Request)

// Create mock server to test REST APIs.
// t           - The testing object
// testHandler - The HTTP handler of the test
func CreateRestsMockServer(t *testing.T, testHandler restsTestHandler) (*httptest.Server, *config.ServerDetails, artifactory.ArtifactoryServicesManager) {
	testServer := httptest.NewServer(http.HandlerFunc(testHandler))
	serverDetails := &config.ServerDetails{ArtifactoryUrl: testServer.URL + "/"}

	serviceManager, err := utils.CreateServiceManager(serverDetails, -1, 0, false)
	assert.NoError(t, err)
	return testServer, serverDetails, serviceManager
}
