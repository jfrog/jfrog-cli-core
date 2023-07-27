package transferfiles

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gocarina/gocsv"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	coreUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/stretchr/testify/assert"
)

func TestHandleStopInitAndClose(t *testing.T) {
	transferFilesCommand, err := NewTransferFilesCommand(nil, nil)
	assert.NoError(t, err)
	finishStopping, _ := transferFilesCommand.handleStop(nil)
	finishStopping()
}

func TestCancelFunc(t *testing.T) {
	transferFilesCommand, err := NewTransferFilesCommand(nil, nil)
	assert.NoError(t, err)
	assert.False(t, transferFilesCommand.shouldStop())

	transferFilesCommand.cancelFunc()
	assert.True(t, transferFilesCommand.shouldStop())
}

func TestSignalStop(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	// Create transfer files command and mark the transfer as started
	transferFilesCommand, err := NewTransferFilesCommand(nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, transferFilesCommand.initTransferDir())
	assert.NoError(t, transferFilesCommand.stateManager.TryLockTransferStateManager())

	// Make sure that the '.jfrog/transfer/stop' doesn't exist
	transferDir, err := coreutils.GetJfrogTransferDir()
	assert.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(transferDir, StopFileName))

	// Run signalStop and make sure that the '.jfrog/transfer/stop' exists
	assert.NoError(t, transferFilesCommand.signalStop())
	assert.FileExists(t, filepath.Join(transferDir, StopFileName))
}

func TestSignalStopError(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	// Create transfer files command and mark the transfer as started
	transferFilesCommand, err := NewTransferFilesCommand(nil, nil)
	assert.NoError(t, err)

	// Check "not active file transfer" error
	assert.EqualError(t, transferFilesCommand.signalStop(), "There is no active file transfer process.")

	// Mock start transfer
	assert.NoError(t, transferFilesCommand.initTransferDir())
	assert.NoError(t, transferFilesCommand.stateManager.TryLockTransferStateManager())

	// Check "already in progress" error
	assert.NoError(t, transferFilesCommand.signalStop())
	assert.EqualError(t, transferFilesCommand.signalStop(), "Graceful stop is already in progress. Please wait...")
}

/* #nosec G101 -- Not credentials. */
const (
	firstUuidTokenForTest  = "347cd3e9-86b6-4bec-9be9-e053a485f327"
	secondUuidTokenForTest = "af14706e-e0c1-4b7d-8791-6a18bd1fd339"
	nodeIdForTest          = "nodea0gwihu76sk5g-artifactory-primary-0"
)

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
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+utils.PluginsExecuteRestApi+"verifyCompatibility" {
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
		assert.EqualError(t, err, coreutils.ValidateMinimumVersion(coreutils.DataTransfer, curVersion, dataTransferPluginMinVersion).Error())
		return
	}
	assert.NoError(t, err)
}

func TestVerifySourceTargetConnectivity(t *testing.T) {
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+utils.PluginsExecuteRestApi+"verifySourceTargetConnectivity" {
			w.WriteHeader(http.StatusOK)
		}
	})
	defer testServer.Close()
	srcPluginManager := initSrcUserPluginServiceManager(t, serverDetails)
	transferFilesCommand, err := NewTransferFilesCommand(serverDetails, serverDetails)
	assert.NoError(t, err)
	err = transferFilesCommand.verifySourceTargetConnectivity(srcPluginManager)
	assert.NoError(t, err)
}

func TestVerifySourceTargetConnectivityError(t *testing.T) {
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+utils.PluginsExecuteRestApi+"verifySourceTargetConnectivity" {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte("No connection to target"))
			assert.NoError(t, err)
		}
	})
	defer testServer.Close()
	srcPluginManager := initSrcUserPluginServiceManager(t, serverDetails)
	transferFilesCommand, err := NewTransferFilesCommand(serverDetails, serverDetails)
	assert.NoError(t, err)
	err = transferFilesCommand.verifySourceTargetConnectivity(srcPluginManager)
	assert.ErrorContains(t, err, "No connection to target")
}

func initSrcUserPluginServiceManager(t *testing.T, serverDetails *coreConfig.ServerDetails) *srcUserPluginService {
	srcPluginManager, err := createSrcRtUserPluginServiceManager(context.Background(), serverDetails)
	assert.NoError(t, err)
	return srcPluginManager
}

func TestVerifyConfigImportPluginNotInstalled(t *testing.T) {
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+utils.PluginsExecuteRestApi+"dataTransferVersion" {
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
	stateManager, cleanUp := state.InitStateTest(t)
	defer cleanUp()

	totalChunkStatusVisits := 0
	totalUploadChunkVisits := 0
	fileSample := api.FileRepresentation{
		Repo: repo1Key,
		Path: "rel-path",
		Name: "name-demo",
	}

	testServer, serverDetails, _ := initPollUploadsTestMockServer(t, &totalChunkStatusVisits, &totalUploadChunkVisits, fileSample)
	defer testServer.Close()
	srcPluginManager := initSrcUserPluginServiceManager(t, serverDetails)

	assert.NoError(t, stateManager.SetRepoState(repo1Key, 0, 0, false, true))
	phaseBase := &phaseBase{context: context.Background(), stateManager: stateManager, srcUpService: srcPluginManager, repoKey: repo1Key}
	uploadChunkAndPollTwice(t, phaseBase, fileSample)

	// Assert that exactly 2 requests to chunk status were made
	// First request - get one DONE chunk and one IN PROGRESS
	// Second Request - get DONE for the other chunk
	assert.Equal(t, 2, totalChunkStatusVisits)
}

// Sends chunk to upload, polls on chunk three times - once when it is still in progress, once after done received and once to notify back to the source.
func uploadChunkAndPollTwice(t *testing.T, phaseBase *phaseBase, fileSample api.FileRepresentation) {
	curThreads = 8
	uploadChunksChan := make(chan UploadedChunk, 3)
	doneChan := make(chan bool, 1)
	var runWaitGroup sync.WaitGroup

	chunk := api.UploadChunk{}
	chunk.AppendUploadCandidateIfNeeded(fileSample, false)
	stopped := uploadChunkWhenPossible(phaseBase, chunk, uploadChunksChan, nil)
	assert.False(t, stopped)
	stopped = uploadChunkWhenPossible(phaseBase, chunk, uploadChunksChan, nil)
	assert.False(t, stopped)
	assert.Equal(t, 2, curProcessedUploadChunks)

	runWaitGroup.Add(1)
	go func() {
		defer runWaitGroup.Done()
		pollUploads(phaseBase, phaseBase.srcUpService, uploadChunksChan, doneChan, nil)
	}()
	// Let the whole process run for a few chunk status checks, then mark it as done.
	time.Sleep(5 * waitTimeBetweenChunkStatusSeconds * time.Second)
	doneChan <- true
	// Wait for the go routine to return.
	runWaitGroup.Wait()
}

func getUploadChunkMockResponse(t *testing.T, w http.ResponseWriter, totalUploadChunkVisits *int) {
	w.WriteHeader(http.StatusAccepted)
	var resp api.UploadChunkResponse
	if *totalUploadChunkVisits == 1 {
		resp = api.UploadChunkResponse{UuidTokenResponse: api.UuidTokenResponse{UuidToken: firstUuidTokenForTest}, NodeIdResponse: api.NodeIdResponse{NodeId: nodeIdForTest}}
	} else {
		resp = api.UploadChunkResponse{UuidTokenResponse: api.UuidTokenResponse{UuidToken: secondUuidTokenForTest}, NodeIdResponse: api.NodeIdResponse{NodeId: nodeIdForTest}}
	}
	writeMockResponse(t, w, resp)
}

func validateChunkStatusBody(t *testing.T, r *http.Request) {
	// Read body
	content, err := io.ReadAll(r.Body)
	assert.NoError(t, err)
	var actual api.UploadChunksStatusBody
	assert.NoError(t, json.Unmarshal(content, &actual))

	// Make sure all parameters as expected
	if len(actual.ChunksToDelete) == 0 {
		assert.Len(t, actual.AwaitingStatusChunks, 2)
		assert.ElementsMatch(t, []api.ChunkId{firstUuidTokenForTest, secondUuidTokenForTest}, actual.AwaitingStatusChunks)
	} else {
		assert.Len(t, actual.ChunksToDelete, 1)
		assert.Len(t, actual.AwaitingStatusChunks, 1)
		assert.Equal(t, api.ChunkId(firstUuidTokenForTest), actual.ChunksToDelete[0])
		assert.Equal(t, api.ChunkId(secondUuidTokenForTest), actual.AwaitingStatusChunks[0])
	}

}

func getChunkStatusMockFirstResponse(t *testing.T, w http.ResponseWriter) {
	resp := getSampleChunkStatus()
	resp.ChunksStatus[0].Status = api.Done
	writeMockResponse(t, w, resp)
}

func getChunkStatusMockSecondResponse(t *testing.T, w http.ResponseWriter, file api.FileRepresentation) {
	resp := api.UploadChunksStatusResponse{
		ChunksStatus: []api.ChunkStatus{
			{
				UuidTokenResponse: api.UuidTokenResponse{UuidToken: secondUuidTokenForTest},
				Status:            api.Done,
				Files:             []api.FileUploadStatusResponse{{FileRepresentation: file, Status: api.Success, StatusCode: http.StatusOK}},
			},
		},
		DeletedChunks: []string{
			firstUuidTokenForTest,
		},
		NodeIdResponse: api.NodeIdResponse{
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

func initPollUploadsTestMockServer(t *testing.T, totalChunkStatusVisits *int, totalUploadChunkVisits *int, file api.FileRepresentation) (*httptest.Server, *coreConfig.ServerDetails, artifactory.ArtifactoryServicesManager) {
	return commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+utils.PluginsExecuteRestApi+"uploadChunk" {
			*totalUploadChunkVisits++
			getUploadChunkMockResponse(t, w, totalUploadChunkVisits)
		} else if r.RequestURI == "/"+utils.PluginsExecuteRestApi+syncChunks {
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
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.RequestURI {
		case "/api/storageinfo/calculate":
			// Response for CalculateStorageInfo
			w.WriteHeader(http.StatusAccepted)
		case "/api/storageinfo":
			// Response for GetStorageInfo
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
			// Response for GetWithFilter
			w.WriteHeader(http.StatusOK)
			response := &[]services.RepositoryDetails{{Key: "repo-1"}, {Key: "repo-2"}}
			bytes, err := json.Marshal(response)
			assert.NoError(t, err)
			_, err = w.Write(bytes)
			assert.NoError(t, err)
		case "/api/repositories?type=federated&packageType=":
			// Response for GetWithFilter
			w.WriteHeader(http.StatusOK)
			// We add a build info repository to the response to cover cases whereby a federated build-info repository is returned
			response := &[]services.RepositoryDetails{{Key: "federated-repo-1"}, {Key: "federated-repo-2"}, {Key: "proj-build-info"}}
			bytes, err := json.Marshal(response)
			assert.NoError(t, err)
			_, err = w.Write(bytes)
			assert.NoError(t, err)
		}
	})
	defer testServer.Close()

	// Get and assert regular local and build info repositories
	transferFilesCommand, err := NewTransferFilesCommand(nil, nil)
	assert.NoError(t, err)
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
	sourceTestServer, sourceServerDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/storageinfo/calculate" {
			w.WriteHeader(http.StatusAccepted)
			sourceServerCalculated = true
		}
	})
	defer sourceTestServer.Close()

	// Prepare target mock server
	targetTestServer, targetServerDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/storageinfo/calculate" {
			w.WriteHeader(http.StatusAccepted)
			targetServerCalculated = true
		}
	})
	defer targetTestServer.Close()

	// Init and assert storage info managers
	transferFilesCommand, err := NewTransferFilesCommand(sourceServerDetails, targetServerDetails)
	assert.NoError(t, err)
	err = transferFilesCommand.initStorageInfoManagers()
	assert.NoError(t, err)
	assert.True(t, sourceServerCalculated)
	assert.True(t, targetServerCalculated)
}

func getSampleChunkStatus() api.UploadChunksStatusResponse {
	return api.UploadChunksStatusResponse{
		NodeIdResponse: api.NodeIdResponse{NodeId: nodeIdForTest},
		ChunksStatus: []api.ChunkStatus{
			{
				UuidTokenResponse: api.UuidTokenResponse{UuidToken: firstUuidTokenForTest},
				Status:            api.InProgress,
				Files: []api.FileUploadStatusResponse{
					{
						FileRepresentation: api.FileRepresentation{
							Repo: "my-repo-local-2",
							Path: "rel-path-2",
							Name: "name-demo-2",
						},
					},
				},
			},
			{
				UuidTokenResponse: api.UuidTokenResponse{UuidToken: secondUuidTokenForTest},
				Status:            api.InProgress,
				Files: []api.FileUploadStatusResponse{
					{
						FileRepresentation: api.FileRepresentation{
							Repo: "my-repo-local-1",
							Path: "rel-path-1",
							Name: "name-demo-1",
						},
					},
				},
			},
		},
	}
}

func TestCheckChunkStatusSync(t *testing.T) {
	chunkStatus := getSampleChunkStatus()
	manager := ChunksLifeCycleManager{
		nodeToChunksMap: map[nodeId]map[api.ChunkId]UploadedChunkData{},
		totalChunks:     2,
	}
	manager.nodeToChunksMap[nodeIdForTest] = map[api.ChunkId]UploadedChunkData{}
	manager.nodeToChunksMap[nodeIdForTest][firstUuidTokenForTest] = UploadedChunkData{}
	manager.nodeToChunksMap[nodeIdForTest][secondUuidTokenForTest] = UploadedChunkData{}
	errChanMng := createErrorsChannelMng()
	checkChunkStatusSync(&chunkStatus, &manager, &errChanMng)
	assert.Len(t, manager.nodeToChunksMap[nodeIdForTest], 2)
	chunkStatus.ChunksStatus = chunkStatus.ChunksStatus[:len(chunkStatus.ChunksStatus)-1]
	checkChunkStatusSync(&chunkStatus, &manager, &errChanMng)
	assert.Len(t, manager.nodeToChunksMap[nodeIdForTest], 1)
	chunkStatus.ChunksStatus = chunkStatus.ChunksStatus[:len(chunkStatus.ChunksStatus)-1]
	checkChunkStatusSync(&chunkStatus, &manager, &errChanMng)
	assert.Len(t, manager.nodeToChunksMap[nodeIdForTest], 0)
}

func TestCreateErrorsSummaryFile(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	testDataDir := filepath.Join("..", "testdata", "transfer_summary")
	logFiles := []string{filepath.Join(testDataDir, "logs1.json"), filepath.Join(testDataDir, "logs2.json")}
	allErrors, err := parseErrorsFromLogFiles(logFiles)
	assert.NoError(t, err)
	// Create Errors Summary Csv File from given JSON log files
	createdCsvPath, err := utils.CreateCSVFile("transfer-files-logs", allErrors.Errors, time.Now())
	assert.NoError(t, err)
	assert.NotEmpty(t, createdCsvPath)
	createdFile, err := os.Open(createdCsvPath)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, createdFile.Close())
	}()
	actualFileErrors := new([]api.FileUploadStatusResponse)
	assert.NoError(t, gocsv.UnmarshalFile(createdFile, actualFileErrors))

	// Create expected csv file
	expectedFile, err := os.Open(filepath.Join(testDataDir, "logs.csv"))
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, expectedFile.Close())
	}()
	expectedFileErrors := new([]api.FileUploadStatusResponse)
	assert.NoError(t, gocsv.UnmarshalFile(expectedFile, expectedFileErrors))
	assert.ElementsMatch(t, *expectedFileErrors, *actualFileErrors)
}
