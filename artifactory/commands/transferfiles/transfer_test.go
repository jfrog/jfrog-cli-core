package transferfiles

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	coreUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"

	"github.com/stretchr/testify/assert"
)

func TestHandleStopInitAndClose(t *testing.T) {
	transferFilesCommand := NewTransferFilesCommand(nil, nil)
	finishStopping, _ := transferFilesCommand.handleStop(nil)
	finishStopping()
}

func TestCancelFunc(t *testing.T) {
	transferFilesCommand := NewTransferFilesCommand(nil, nil)
	assert.False(t, transferFilesCommand.shouldStop())

	transferFilesCommand.cancelFunc()
	assert.True(t, transferFilesCommand.shouldStop())
}

const firstUuidTokenForTest = "347cd3e9-86b6-4bec-9be9-e053a485f327"
const secondUuidTokenForTest = "af14706e-e0c1-4b7d-8791-6a18bd1fd339"
const nodeIdForTest = "nodea0gwihu76sk5g-artifactory-primary-0"

func TestValidateDataTransferPluginMinimumVersion(t *testing.T) {
	t.Run("valid version", func(t *testing.T) { testValidateDataTransferPluginMinimumVersion(t, "9.9.9", false) })
	t.Run("exact version", func(t *testing.T) {
		testValidateDataTransferPluginMinimumVersion(t, dataTransferPluginMinVersion, false)
	})
	t.Run("invalid version", func(t *testing.T) { testValidateDataTransferPluginMinimumVersion(t, "1.0.0", true) })
	t.Run("snapshot version", func(t *testing.T) { testValidateDataTransferPluginMinimumVersion(t, "1.0.x-SNAPSHOT", false) })
}

func testValidateDataTransferPluginMinimumVersion(t *testing.T, curVersion string, errorExpected bool) {
	var pluginVersion string
	testServer, serverDetails, _ := commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+pluginsExecuteRestApi+"verifyCompatibility" {
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

func TestVerifySourceTargetConnectivity(t *testing.T) {
	testServer, serverDetails, _ := commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+pluginsExecuteRestApi+"verifySourceTargetConnectivity" {
			w.WriteHeader(http.StatusOK)
		}
	})
	defer testServer.Close()
	srcPluginManager := initSrcUserPluginServiceManager(t, serverDetails)
	transferFilesCommand := NewTransferFilesCommand(serverDetails, serverDetails)
	err := transferFilesCommand.verifySourceTargetConnectivity(srcPluginManager)
	assert.NoError(t, err)
}

func TestVerifySourceTargetConnectivityError(t *testing.T) {
	testServer, serverDetails, _ := commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+pluginsExecuteRestApi+"verifySourceTargetConnectivity" {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte("No connection to target"))
			assert.NoError(t, err)
		}
	})
	defer testServer.Close()
	srcPluginManager := initSrcUserPluginServiceManager(t, serverDetails)
	transferFilesCommand := NewTransferFilesCommand(serverDetails, serverDetails)
	err := transferFilesCommand.verifySourceTargetConnectivity(srcPluginManager)
	assert.ErrorContains(t, err, "No connection to target")
}

func initSrcUserPluginServiceManager(t *testing.T, serverDetails *coreConfig.ServerDetails) *srcUserPluginService {
	srcPluginManager, err := createSrcRtUserPluginServiceManager(context.Background(), serverDetails)
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
	totalUploadChunkVisits := 0
	fileSample := FileRepresentation{
		Repo: "my-repo-local",
		Path: "rel-path",
		Name: "name-demo",
	}

	testServer, serverDetails, _ := initPollUploadsTestMockServer(t, &totalChunkStatusVisits, &totalUploadChunkVisits, fileSample)
	defer testServer.Close()
	srcPluginManager := initSrcUserPluginServiceManager(t, serverDetails)

	uploadChunkAndPollTwice(t, srcPluginManager, fileSample)

	// Assert that exactly 2 requests to chunk status were made
	// First request - get one DONE chunk and one IN PROGRESS
	// Second Request - get DONE for the other chunk
	assert.Equal(t, 2, totalChunkStatusVisits)
}

// Sends chunk to upload, polls on chunk three times - once when it is still in progress, once after done received and once to notify back to the source.
func uploadChunkAndPollTwice(t *testing.T, srcPluginManager *srcUserPluginService, fileSample FileRepresentation) {
	curThreads = 8
	uploadChunksChan := make(chan UploadedChunkData, 3)
	doneChan := make(chan bool, 1)
	var runWaitGroup sync.WaitGroup

	chunk := UploadChunk{}
	chunk.appendUploadCandidateIfNeeded(fileSample, false)
	stopped := uploadChunkWhenPossible(&phaseBase{context: context.Background(), srcUpService: srcPluginManager}, chunk, uploadChunksChan, nil)
	assert.False(t, stopped)
	stopped = uploadChunkWhenPossible(&phaseBase{context: context.Background(), srcUpService: srcPluginManager}, chunk, uploadChunksChan, nil)
	assert.False(t, stopped)
	assert.Equal(t, 2, curProcessedUploadChunks)

	runWaitGroup.Add(1)
	go func() {
		defer runWaitGroup.Done()
		pollUploads(nil, srcPluginManager, uploadChunksChan, doneChan, nil)
	}()
	// Let the whole process run for a few chunk status checks, then mark it as done.
	time.Sleep(5 * waitTimeBetweenChunkStatusSeconds * time.Second)
	doneChan <- true
	// Wait for the go routine to return.
	runWaitGroup.Wait()
}

func getUploadChunkMockResponse(t *testing.T, w http.ResponseWriter, totalUploadChunkVisits *int) {
	w.WriteHeader(http.StatusAccepted)
	var resp UploadChunkResponse
	if *totalUploadChunkVisits == 1 {
		resp = UploadChunkResponse{UuidTokenResponse: UuidTokenResponse{UuidToken: firstUuidTokenForTest}, NodeIdResponse: NodeIdResponse{NodeId: nodeIdForTest}}
	} else {
		resp = UploadChunkResponse{UuidTokenResponse: UuidTokenResponse{UuidToken: secondUuidTokenForTest}, NodeIdResponse: NodeIdResponse{NodeId: nodeIdForTest}}
	}
	writeMockResponse(t, w, resp)
}

func validateChunkStatusBody(t *testing.T, r *http.Request) {
	// Read body
	content, err := io.ReadAll(r.Body)
	assert.NoError(t, err)
	var actual UploadChunksStatusBody
	assert.NoError(t, json.Unmarshal(content, &actual))

	// Make sure all parameters as expected
	if len(actual.ChunksToDelete) == 0 {
		assert.Len(t, actual.AwaitingStatusChunks, 2)
		assert.ElementsMatch(t, []chunkId{firstUuidTokenForTest, secondUuidTokenForTest}, actual.AwaitingStatusChunks)
	} else {
		assert.Len(t, actual.ChunksToDelete, 1)
		assert.Len(t, actual.AwaitingStatusChunks, 1)
		assert.Equal(t, chunkId(firstUuidTokenForTest), actual.ChunksToDelete[0])
		assert.Equal(t, chunkId(secondUuidTokenForTest), actual.AwaitingStatusChunks[0])
	}

}

func getChunkStatusMockFirstResponse(t *testing.T, w http.ResponseWriter) {
	resp := UploadChunksStatusResponse{
		ChunksStatus: []ChunkStatus{
			{
				UuidTokenResponse: UuidTokenResponse{UuidToken: firstUuidTokenForTest},
				Status:            Done,
			},
			{
				UuidTokenResponse: UuidTokenResponse{UuidToken: secondUuidTokenForTest},
				Status:            InProgress,
			},
		},
		NodeIdResponse: NodeIdResponse{
			NodeId: nodeIdForTest,
		},
	}
	writeMockResponse(t, w, resp)
}

func getChunkStatusMockSecondResponse(t *testing.T, w http.ResponseWriter, file FileRepresentation) {
	resp := UploadChunksStatusResponse{
		ChunksStatus: []ChunkStatus{
			{
				UuidTokenResponse: UuidTokenResponse{UuidToken: secondUuidTokenForTest},
				Status:            Done,
				Files:             []FileUploadStatusResponse{{FileRepresentation: file, Status: Success, StatusCode: http.StatusOK}},
			},
		},
		DeletedChunks: []string{
			firstUuidTokenForTest,
		},
		NodeIdResponse: NodeIdResponse{
			NodeId: nodeIdForTest,
		},
	}
	writeMockResponse(t, w, resp)
}

func writeMockResponse(t *testing.T, w http.ResponseWriter, resp interface{}) {
	content, err := json.Marshal(resp)
	assert.NoError(t, err)
	_, err = w.Write(content)
	assert.NoError(t, err)
}

func initPollUploadsTestMockServer(t *testing.T, totalChunkStatusVisits *int, totalUploadChunkVisits *int, file FileRepresentation) (*httptest.Server, *coreConfig.ServerDetails, artifactory.ArtifactoryServicesManager) {
	return commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+pluginsExecuteRestApi+"uploadChunk" {
			*totalUploadChunkVisits++
			getUploadChunkMockResponse(t, w, totalUploadChunkVisits)
		} else if r.RequestURI == "/"+pluginsExecuteRestApi+syncChunks {
			*totalChunkStatusVisits++
			validateChunkStatusBody(t, r)
			// If already visited chunk status, return status done this time.
			if *totalChunkStatusVisits == 1 {
				getChunkStatusMockFirstResponse(t, w)
			} else {
				getChunkStatusMockSecondResponse(t, w, file)
			}
		}
	})
}

func TestGetAllLocalRepositories(t *testing.T) {
	// Prepare mock server
	testServer, serverDetails, _ := commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/api/storageinfo/calculate":
			// Reponse for CalculateStorageInfo
			w.WriteHeader(http.StatusAccepted)
		case "/api/storageinfo":
			// Reponse for GetStorageInfo
			w.WriteHeader(http.StatusOK)
			response := &clientUtils.StorageInfo{RepositoriesSummaryList: []clientUtils.RepositorySummary{
				{RepoKey: "repo-1"}, {RepoKey: "repo-2"},
				{RepoKey: "federated-repo-1"}, {RepoKey: "federated-repo-2"},
				{RepoKey: "artifactory-build-info", PackageType: "BuildInfo"}, {RepoKey: "proj-build-info", PackageType: "BuildInfo"}},
			}
			bytes, err := json.Marshal(response)
			assert.NoError(t, err)
			_, err = w.Write(bytes)
			assert.NoError(t, err)
		case "/api/repositories?type=local&packageType=":
			// Reponse for GetWithFilter
			w.WriteHeader(http.StatusOK)
			response := &[]services.RepositoryDetails{{Key: "repo-1"}, {Key: "repo-2"}}
			bytes, err := json.Marshal(response)
			assert.NoError(t, err)
			_, err = w.Write(bytes)
			assert.NoError(t, err)
		case "/api/repositories?type=federated&packageType=":
			// Reponse for GetWithFilter
			w.WriteHeader(http.StatusOK)
			response := &[]services.RepositoryDetails{{Key: "federated-repo-1"}, {Key: "federated-repo-2"}}
			bytes, err := json.Marshal(response)
			assert.NoError(t, err)
			_, err = w.Write(bytes)
			assert.NoError(t, err)
		}
	})
	defer testServer.Close()

	// Get and assert regular local and build info repositories
	transferFilesCommand := NewTransferFilesCommand(nil, nil)
	storageInfoManager, err := coreUtils.NewStorageInfoManager(context.Background(), serverDetails)
	assert.NoError(t, err)
	localRepos, localBuildInfoRepo, err := transferFilesCommand.getAllLocalRepos(serverDetails, storageInfoManager)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"repo-1", "repo-2", "federated-repo-1", "federated-repo-2"}, localRepos)
	assert.ElementsMatch(t, []string{"artifactory-build-info", "proj-build-info"}, localBuildInfoRepo)
}

func TestInitStorageInfoManagers(t *testing.T) {
	sourceServerCalculated, targetServerCalculated := false, false
	// Prepare source mock server
	sourceTestServer, sourceServerDetails, _ := commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/storageinfo/calculate" {
			w.WriteHeader(http.StatusAccepted)
			sourceServerCalculated = true
		}
	})
	defer sourceTestServer.Close()

	// Prepare target mock server
	targetTestServer, targetserverDetails, _ := commonTests.CreateRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/storageinfo/calculate" {
			w.WriteHeader(http.StatusAccepted)
			targetServerCalculated = true
		}
	})
	defer targetTestServer.Close()

	// Init and assert storage info managers
	transferFilesCommand := NewTransferFilesCommand(sourceServerDetails, targetserverDetails)
	err := transferFilesCommand.initStorageInfoManagers()
	assert.NoError(t, err)
	assert.True(t, sourceServerCalculated)
	assert.True(t, targetServerCalculated)
}
