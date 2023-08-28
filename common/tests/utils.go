package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	testsutils "github.com/jfrog/jfrog-cli-core/v2/utils/config/tests"
	"github.com/jfrog/jfrog-client-go/access"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/distribution"
	"github.com/stretchr/testify/assert"
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
// testHandler - The HTTP handler of the test
func CreateRestsMockServer(testHandler restsTestHandler) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(testHandler))
}

func CreateRtRestsMockServer(t *testing.T, testHandler restsTestHandler) (*httptest.Server, *config.ServerDetails, artifactory.ArtifactoryServicesManager) {
	testServer := CreateRestsMockServer(testHandler)
	serverDetails := &config.ServerDetails{Url: testServer.URL + "/", ArtifactoryUrl: testServer.URL + "/"}

	serviceManager, err := utils.CreateServiceManager(serverDetails, -1, 0, false)
	assert.NoError(t, err)
	return testServer, serverDetails, serviceManager
}

func CreateAccessRestsMockServer(t *testing.T, testHandler restsTestHandler) (*httptest.Server, *config.ServerDetails, *access.AccessServicesManager) {
	testServer := CreateRestsMockServer(testHandler)
	serverDetails := &config.ServerDetails{Url: testServer.URL + "/", ServerId: "test-server"}

	serviceManager, err := utils.CreateAccessServiceManager(serverDetails, false)
	assert.NoError(t, err)
	return testServer, serverDetails, serviceManager
}

func CreateDsRestsMockServer(t *testing.T, testHandler restsTestHandler) (*httptest.Server, *config.ServerDetails, *distribution.DistributionServicesManager) {
	testServer := CreateRestsMockServer(testHandler)
	serverDetails := &config.ServerDetails{DistributionUrl: testServer.URL + "/"}

	serviceManager, err := utils.CreateDistributionServiceManager(serverDetails, false)
	assert.NoError(t, err)
	return testServer, serverDetails, serviceManager
}
