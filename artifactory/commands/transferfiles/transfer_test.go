package transferfiles

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestHandleStopInitAndClose(t *testing.T) {
	transferFilesCommand := NewTransferFilesCommand(nil, nil)

	shouldStop := false
	var newPhase transferPhase
	finishStopping := transferFilesCommand.handleStop(&shouldStop, &newPhase, nil)
	finishStopping()
}

const uuidTokenForTest = "af14706e-e0c1-4b7d-8791-6a18bd1fd339"

func TestValidateDataTransferPluginMinimumVersion(t *testing.T) {
	t.Run("valid version", func(t *testing.T) { testValidateDataTransferPluginMinimumVersion(t, "9.9.9", false) })
	t.Run("exact version", func(t *testing.T) {
		testValidateDataTransferPluginMinimumVersion(t, dataTransferPluginMinVersion, false)
	})
	t.Run("invalid version", func(t *testing.T) { testValidateDataTransferPluginMinimumVersion(t, "1.0.0", true) })
}

func testValidateDataTransferPluginMinimumVersion(t *testing.T, curVersion string, errorExpected bool) {
	var pluginVersion string
	testServer, serverDetails, _ := commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+pluginsExecuteRestApi+"dataTransferVersion" {
			content, err := json.Marshal(utils.VersionResponse{Version: pluginVersion})
			assert.NoError(t, err)
			_, err = w.Write(content)
			assert.NoError(t, err)
		}
	})
	defer testServer.Close()
	srcPluginManager := initSrcUserPluginServiceManager(t, serverDetails)

	pluginVersion = curVersion
	err := getAndValidateDataTransferPlugin(srcPluginManager)
	if errorExpected {
		assert.EqualError(t, err, getMinimalVersionErrorMsg(curVersion))
		return
	}
	assert.NoError(t, err)
}

func initSrcUserPluginServiceManager(t *testing.T, serverDetails *coreConfig.ServerDetails) *srcUserPluginService {
	srcPluginManager, err := createSrcRtUserPluginServiceManager(serverDetails)
	assert.NoError(t, err)
	return srcPluginManager
}

func TestVerifyConfigImportPluginNotInstalled(t *testing.T) {
	testServer, serverDetails, _ := commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+pluginsExecuteRestApi+"dataTransferVersion" {
			w.WriteHeader(http.StatusNotFound)
			_, err := w.Write([]byte("Not found"))
			assert.NoError(t, err)
		}
	})
	defer testServer.Close()
	srcPluginManager := initSrcUserPluginServiceManager(t, serverDetails)

	_, err := srcPluginManager.version()
	assert.ErrorContains(t, err, "Response from Artifactory: 404 Not Found.")
}

func TestUploadChunkAndPollUploads(t *testing.T) {
	totalChunkStatusVisits := 0
	fileSample := FileRepresentation{
		Repo: "my-repo-local",
		Path: "rel-path",
		Name: "name-demo",
	}

	testServer, serverDetails, _ := initPollUploadsTestMockServer(t, &totalChunkStatusVisits, fileSample)
	defer testServer.Close()
	srcPluginManager := initSrcUserPluginServiceManager(t, serverDetails)

	uploadChunkAndPollTwice(t, srcPluginManager, fileSample)

	// Assert that exactly 2 requests to chunk status were made - once when still in progress, and once when done.
	assert.Equal(t, 2, totalChunkStatusVisits)
}

// Sends chunk to upload, polls on chunk twice - once when it is still in progress, and once after it is done.
func uploadChunkAndPollTwice(t *testing.T, srcPluginManager *srcUserPluginService, fileSample FileRepresentation) {
	curThreads = 8
	uploadTokensChan := make(chan string, 3)
	doneChan := make(chan bool, 1)
	var runWaitGroup sync.WaitGroup

	err := uploadChunkAndAddToken(srcPluginManager, UploadChunk{
		UploadCandidates: []FileRepresentation{fileSample},
	}, uploadTokensChan)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	runWaitGroup.Add(1)
	go func() {
		defer runWaitGroup.Done()
		pollUploads(nil, srcPluginManager, uploadTokensChan, doneChan, nil, nil)
	}()
	// Let the whole process run for a few chunk status checks, then mark it as done.
	time.Sleep(5 * waitTimeBetweenChunkStatusSeconds * time.Second)
	doneChan <- true
	// Wait for the go routine to return.
	runWaitGroup.Wait()
}

func getUploadChunkMockResponse(t *testing.T, w http.ResponseWriter) {
	w.WriteHeader(http.StatusAccepted)
	resp := UploadChunkResponse{UuidTokenResponse: UuidTokenResponse{UuidToken: uuidTokenForTest}}
	writeMockResponse(t, w, resp)
}

func validateChunkStatusBody(t *testing.T, r *http.Request) {
	// Read body
	content, err := io.ReadAll(r.Body)
	assert.NoError(t, err)
	var actual UploadChunksStatusBody
	assert.NoError(t, json.Unmarshal(content, &actual))

	// Make sure all parameters as expected
	assert.Len(t, actual.UuidTokens, 1)
	assert.Equal(t, uuidTokenForTest, actual.UuidTokens[0])
}

func getChunkStatusMockInProgressResponse(t *testing.T, w http.ResponseWriter) {
	resp := UploadChunksStatusResponse{ChunksStatus: []ChunkStatus{
		{
			UuidTokenResponse: UuidTokenResponse{UuidToken: uuidTokenForTest},
			Status:            InProgress,
		},
	}}
	writeMockResponse(t, w, resp)
}

func getChunkStatusMockDoneResponse(t *testing.T, w http.ResponseWriter, file FileRepresentation) {
	resp := UploadChunksStatusResponse{ChunksStatus: []ChunkStatus{
		{
			UuidTokenResponse: UuidTokenResponse{UuidToken: uuidTokenForTest},
			Status:            Done,
			Files:             []FileUploadStatusResponse{{FileRepresentation: file, Status: Success, StatusCode: http.StatusOK}},
		},
	}}
	writeMockResponse(t, w, resp)
}

func writeMockResponse(t *testing.T, w http.ResponseWriter, resp interface{}) {
	content, err := json.Marshal(resp)
	assert.NoError(t, err)
	_, err = w.Write(content)
	assert.NoError(t, err)
}

func initPollUploadsTestMockServer(t *testing.T, totalChunkStatusVisits *int, file FileRepresentation) (*httptest.Server, *coreConfig.ServerDetails, artifactory.ArtifactoryServicesManager) {
	return commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+pluginsExecuteRestApi+"uploadChunk" {
			getUploadChunkMockResponse(t, w)
		} else if r.RequestURI == "/"+pluginsExecuteRestApi+"getUploadChunksStatus" {
			*totalChunkStatusVisits++
			validateChunkStatusBody(t, r)

			// If already visited chunk status, return status done this time.
			if *totalChunkStatusVisits > 1 {
				getChunkStatusMockDoneResponse(t, w, file)
			} else {
				getChunkStatusMockInProgressResponse(t, w)
			}
		}
	})
}
