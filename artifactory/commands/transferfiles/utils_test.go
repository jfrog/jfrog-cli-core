package transferfiles

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

type transferFilesHandler func(w http.ResponseWriter, r *http.Request)

const runningNodesResponse = `
{
	"isHa": true,
	"nodes": [
	  {
		"id": "node-1",
		"state": "RUNNING"
	  },
	  {
		"id": "node-2",
		"state": "RUNNING"
	  },
	  {
		"id": "node-3",
		"state": "RUNNING"
	  }
	]
  }
`

func TestGetRunningNodes(t *testing.T) {
	testServer, serverDetails, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(runningNodesResponse))
		assert.NoError(t, err)
	})
	defer testServer.Close()

	runningNodes, err := getRunningNodes(serverDetails)
	assert.NoError(t, err)
	assert.ElementsMatch(t, runningNodes, []string{"node-1", "node-2", "node-3"})
}

func TestStopTransferOnArtifactoryNodes(t *testing.T) {
	stoppedNodeOne, stoppedNodeTwo := false, false
	requestNumber := 0
	testServer, _, srcUpService := createMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		var nodeId string
		if requestNumber == 0 {
			nodeId = "node-1"
			stoppedNodeOne = true
		} else {
			nodeId = "node-2"
			stoppedNodeTwo = true
		}
		_, err := w.Write([]byte(fmt.Sprintf(`{"node_id": "%s"}`, nodeId)))
		assert.NoError(t, err)
		requestNumber++
	})
	defer testServer.Close()

	stopTransferOnArtifactoryNodes(srcUpService, []string{"node-1", "node-2"})
	assert.True(t, stoppedNodeOne)
	assert.True(t, stoppedNodeTwo)
}

// Create mock server to test transfer config commands
// t           - The testing object
// testHandler - The HTTP handler of the test
func createMockServer(t *testing.T, testHandler transferFilesHandler) (*httptest.Server, *config.ServerDetails, *srcUserPluginService) {
	testServer := httptest.NewServer(http.HandlerFunc(testHandler))
	serverDetails := &config.ServerDetails{ArtifactoryUrl: testServer.URL + "/"}

	serviceManager, err := createSrcRtUserPluginServiceManager(serverDetails)
	assert.NoError(t, err)
	return testServer, serverDetails, serviceManager
}
