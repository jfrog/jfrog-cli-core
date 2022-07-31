package transferfiles

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"
)

const uuidTokenForTest = "af14706e-e0c1-4b7d-8791-6a18bd1fd339"

type validateVersionTestSuite struct {
	testName      string
	curVersion    string
	errorExpected bool
}

func TestValidateDataTransferPluginMinimumVersion(t *testing.T) {
	var pluginVersion string
	testServer, serverDetails, _ := tests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+pluginsExecuteRestApi+"dataTransferVersion" {
			content, err := json.Marshal(utils.VersionResponse{Version: pluginVersion})
			assert.NoError(t, err)
			_, err = w.Write(content)
			assert.NoError(t, err)
		}
	})
	defer testServer.Close()

	srcPluginManager, err := createSrcRtUserPluginServiceManager(serverDetails)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	testsArray := []validateVersionTestSuite{
		{"valid version", "9.9.9", false},
		{"exact version", dataTransferPluginMinVersion, false},
		{"invalid version", "1.0.0", true},
	}

	for _, test := range testsArray {
		t.Run(test.testName, func(t *testing.T) {
			pluginVersion = test.curVersion
			err = getAndValidateDataTransferPlugin(srcPluginManager)
			if test.errorExpected {
				assert.EqualError(t, err, getMinimalVersionErrorMsg(test.curVersion))
				return
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVerifyConfigImportPluginNotInstalled(t *testing.T) {
	testServer, serverDetails, _ := tests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+pluginsExecuteRestApi+"dataTransferVersion" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("Not found"))
		}
	})
	defer testServer.Close()

	srcPluginManager, err := createSrcRtUserPluginServiceManager(serverDetails)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	_, err = srcPluginManager.version()
	assert.ErrorContains(t, err, "Response from Artifactory: 404 Not Found.")
}

func TestUploadChunkAndPollUploads(t *testing.T) {
	totalChunkStatusVisits := 0
	fileSample := FileRepresentation{
		Repo: "my-repo-local",
		Path: "rel-path",
		Name: "name-demo",
	}

	testServer, serverDetails, _ := tests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+pluginsExecuteRestApi+"uploadChunk" {
			w.WriteHeader(http.StatusAccepted)
			content, err := json.Marshal(UploadChunkResponse{UuidTokenResponse: UuidTokenResponse{UuidToken: uuidTokenForTest}})
			assert.NoError(t, err)
			_, err = w.Write(content)
			assert.NoError(t, err)
		} else if r.RequestURI == "/"+pluginsExecuteRestApi+"getUploadChunksStatus" {
			totalChunkStatusVisits++

			// Read body
			content, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			var actual UploadChunksStatusBody
			assert.NoError(t, json.Unmarshal(content, &actual))

			// Make sure all parameters as expected
			assert.Len(t, actual.UuidTokens, 1)
			assert.Equal(t, uuidTokenForTest, actual.UuidTokens[0])

			resp := UploadChunksStatusResponse{ChunksStatus: []ChunkStatus{
				{
					UuidTokenResponse: UuidTokenResponse{UuidToken: uuidTokenForTest},
					Status:            InProgress,
				},
			}}

			// If already visited chunk status, return status done this time.
			if totalChunkStatusVisits > 1 {
				resp.ChunksStatus[0].Status = Done
				resp.ChunksStatus[0].Files = []FileUploadStatusResponse{{FileRepresentation: fileSample, Status: Success, StatusCode: http.StatusOK}}
			}
			content, err = json.Marshal(resp)
			assert.NoError(t, err)
			_, err = w.Write(content)
			assert.NoError(t, err)
		}
	})
	defer testServer.Close()

	srcPluginManager, err := createSrcRtUserPluginServiceManager(serverDetails)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	curThreads = 8
	uploadTokensChan := make(chan string, 3)
	doneChan := make(chan bool, 1)

	// Sends chunk to upload, polls on chunk twice - once when it is still in progress, and once after it is done.
	err = uploadChunkAndAddToken(srcPluginManager, UploadChunk{
		UploadCandidates: []FileRepresentation{fileSample},
	}, uploadTokensChan)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	var runWaitGroup sync.WaitGroup
	runWaitGroup.Add(1)
	go func() {
		defer runWaitGroup.Done()
		err = pollUploads(srcPluginManager, uploadTokensChan, doneChan, nil, nil)
		assert.NoError(t, err)
	}()

	// Let the whole process run for a few chunk status checks, then mark it as done.
	time.Sleep(5 * waitTimeBetweenChunkStatusSeconds * time.Second)
	doneChan <- true
	// Wait for the go routine to return.
	runWaitGroup.Wait()
	// Assert that exactly 2 requests to chunk status were made - once when still in progress, and once when done.
	assert.Equal(t, 2, totalChunkStatusVisits)
}
