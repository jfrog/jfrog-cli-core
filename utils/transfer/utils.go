package transfer

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
