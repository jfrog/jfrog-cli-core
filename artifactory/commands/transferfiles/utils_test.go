package transferfiles

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
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

func TestGetErrorsFiles(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	retryableErrorsDirPath, err := coreutils.GetJfrogTransferRetryableDir()
	assert.NoError(t, err)
	assert.NoError(t, fileutils.CreateDirIfNotExist(retryableErrorsDirPath))

	skippedErrorsDirPath, err := coreutils.GetJfrogTransferSkippedDir()
	assert.NoError(t, err)
	assert.NoError(t, fileutils.CreateDirIfNotExist(skippedErrorsDirPath))

	repoKey := "my-repo-local"
	// Create 3 retryable errors files that belong to the repo.
	assert.NoError(t, writeEmptyErrorsFile(retryableErrorsDirPath, repoKey, 0, 0))
	assert.NoError(t, writeEmptyErrorsFile(retryableErrorsDirPath, repoKey, 0, 1))
	assert.NoError(t, writeEmptyErrorsFile(retryableErrorsDirPath, repoKey, 1, 0))
	// Create 3 retryable errors files that are distractions.
	assert.NoError(t, writeEmptyErrorsFile(retryableErrorsDirPath, "wrong"+repoKey, 0, 0))
	assert.NoError(t, writeEmptyErrorsFile(retryableErrorsDirPath, repoKey+"wrong", 0, 1))
	assert.NoError(t, writeEmptyErrorsFile(retryableErrorsDirPath, "wrong-"+repoKey+"-wrong", 1, 0))
	// Create 1 skipped errors files that belong to the repo.
	assert.NoError(t, writeEmptyErrorsFile(skippedErrorsDirPath, repoKey, 0, 1))

	paths, err := getErrorsFiles(repoKey, true)
	assert.NoError(t, err)
	assert.Len(t, paths, 3)
	paths, err = getErrorsFiles(repoKey, false)
	assert.NoError(t, err)
	assert.Len(t, paths, 1)
}

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

func writeEmptyErrorsFile(path, repoKey string, phase, counter int) error {
	fileName := fmt.Sprintf("%s-%d-%s-%d.json", repoKey, phase, strconv.FormatInt(time.Now().Unix(), 10), counter)
	return ioutil.WriteFile(filepath.Join(path, fileName), nil, 0644)
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
