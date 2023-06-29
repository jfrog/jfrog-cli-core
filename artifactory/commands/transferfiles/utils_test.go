package transferfiles

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
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

const (
	staleChunksNodeIdOne = "node-id-1"
	staleChunksNodeIdTwo = "node-id-2"
	staleChunksChunkId   = "chunk-id"
	staleChunksPath      = "path-in-repo"
	staleChunksName      = "file-name"
)

func TestGetRunningNodes(t *testing.T) {
	testServer, serverDetails, _ := createMockServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(runningNodesResponse))
		assert.NoError(t, err)
	})
	defer testServer.Close()

	runningNodes, err := getRunningNodes(context.Background(), serverDetails)
	assert.NoError(t, err)
	assert.ElementsMatch(t, runningNodes, []string{"node-1", "node-2", "node-3"})
}

func TestStopTransferOnArtifactoryNodes(t *testing.T) {
	stoppedNodeOne, stoppedNodeTwo := false, false
	requestNumber := 0
	testServer, _, srcUpService := createMockServer(t, func(w http.ResponseWriter, _ *http.Request) {
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

	stopTransferInArtifactoryNodes(srcUpService, []string{"node-1", "node-2"})
	assert.True(t, stoppedNodeOne)
	assert.True(t, stoppedNodeTwo)
}

const repoConfigurationResponse = `
{
  "key" : "%[1]s-local",
  "packageType" : "%[1]s",
  "description" : "",
  "notes" : "",
  "includesPattern" : "**/*",
  "excludesPattern" : "",
  "repoLayoutRef" : "simple-default",
  "enableComposerSupport" : false,
  "enableNuGetSupport" : false,
  "enableGemsSupport" : false,
  "enableNpmSupport" : false,
  "enableBowerSupport" : false,
  "enableCocoaPodsSupport" : false,
  "enableConanSupport" : false,
  "enableDebianSupport" : false,
  "debianTrivialLayout" : false,
  "enablePypiSupport" : false,
  "enablePuppetSupport" : false,
  "enableDockerSupport" : false,
  "dockerApiVersion" : "V2",
  "blockPushingSchema1" : true,
  "forceNugetAuthentication" : false,
  "enableVagrantSupport" : false,
  "enableGitLfsSupport" : false,
  "enableDistRepoSupport" : false,
  "priorityResolution" : false,
  "checksumPolicyType" : "client-checksums",
  "handleReleases" : true,
  "handleSnapshots" : true,
  "maxUniqueSnapshots" : %[2]d,
  "maxUniqueTags" : %[3]d,
  "snapshotVersionBehavior" : "unique",
  "suppressPomConsistencyChecks" : true,
  "blackedOut" : false,
  "propertySets" : [ ],
  "archiveBrowsingEnabled" : false,
  "calculateYumMetadata" : false,
  "enableFileListsIndexing" : false,
  "yumRootDepth" : 0,
  "downloadRedirect" : false,
  "xrayIndex" : false,
  "enabledChefSupport" : false,
  "rclass" : "local"
}
`

func TestGetMaxUniqueSnapshots(t *testing.T) {
	testCases := []struct {
		packageType                string
		expectedMaxUniqueSnapshots int
	}{
		{conan, -1},
		{maven, 5},
		{gradle, 5},
		{nuget, 5},
		{ivy, 5},
		{sbt, 5},
		{docker, 3},
	}

	testServer, serverDetails, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		packageType := strings.TrimSuffix(strings.TrimPrefix(r.RequestURI, "/api/repositories/"), "-local")
		var response string
		switch packageType {
		case "docker":
			response = fmt.Sprintf(repoConfigurationResponse, packageType, 0, 3)
		case "maven", "gradle", "nuget", "ivy", "sbt":
			response = fmt.Sprintf(repoConfigurationResponse, packageType, 5, 0)
		default:
			assert.Fail(t, "tried to get the Max Unique Snapshots setting of a repository of an unsupported package type")
		}
		_, err := w.Write([]byte(response))
		assert.NoError(t, err)
	})
	defer testServer.Close()

	for _, testCase := range testCases {
		t.Run(testCase.packageType, func(t *testing.T) {
			lowerPackageType := strings.ToLower(testCase.packageType)
			repoSummary := &utils.RepositorySummary{RepoKey: lowerPackageType + "-local", PackageType: testCase.packageType}
			maxUniqueSnapshots, err := getMaxUniqueSnapshots(context.Background(), serverDetails, repoSummary)
			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedMaxUniqueSnapshots, maxUniqueSnapshots)
		})
	}
}

func TestUpdateMaxUniqueSnapshots(t *testing.T) {
	packageTypes := []string{conan, maven, gradle, nuget, ivy, sbt, docker}

	testServer, serverDetails, _ := createMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		repoDetails := &services.RepositoryDetails{}
		assert.NoError(t, json.Unmarshal(body, repoDetails))
		packageType := repoDetails.PackageType

		expectedPackageType := strings.TrimPrefix(r.RequestURI, "/api/repositories/")
		if strings.HasSuffix(expectedPackageType, "-local") {
			expectedPackageType = strings.TrimSuffix(expectedPackageType, "-local")
			assert.Equal(t, services.LocalRepositoryRepoType, repoDetails.Rclass)
		} else {
			expectedPackageType = strings.TrimSuffix(expectedPackageType, "-federated")
			assert.Equal(t, services.FederatedRepositoryRepoType, repoDetails.Rclass)
		}

		assert.Equal(t, expectedPackageType, packageType)
		switch repoDetails.PackageType {
		case "docker":
			assert.Contains(t, string(body), "\"maxUniqueTags\":5")
		case "maven", "gradle", "nuget", "ivy", "sbt":
			assert.Contains(t, string(body), "\"maxUniqueSnapshots\":5")
		default:
			assert.Fail(t, "tried to update the Max Unique Snapshots setting of a repository of an unsupported package type")
		}
		_, err = w.Write([]byte(fmt.Sprintf("Repository %s-local update successfully.", packageType)))
		assert.NoError(t, err)
	})
	defer testServer.Close()

	for _, packageType := range packageTypes {
		t.Run(packageType, func(t *testing.T) {
			lowerPackageType := strings.ToLower(packageType)
			repoSummary := &utils.RepositorySummary{RepoKey: lowerPackageType + "-local", PackageType: packageType, RepoType: "LOCAL"}
			err := updateMaxUniqueSnapshots(context.Background(), serverDetails, repoSummary, 5)
			assert.NoError(t, err)

			repoSummary = &utils.RepositorySummary{RepoKey: lowerPackageType + "-federated", PackageType: packageType, RepoType: "FEDERATED"}
			err = updateMaxUniqueSnapshots(context.Background(), serverDetails, repoSummary, 5)
			assert.NoError(t, err)
		})
	}
}

func TestInterruptIfRequested(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	// Create new transfer files command
	transferFilesCommand, err := NewTransferFilesCommand(nil, nil)
	assert.NoError(t, err)

	// Run interruptIfRequested and make sure that the interrupted signal wasn't sent to the channel
	assert.NoError(t, interruptIfRequested(transferFilesCommand.stopSignal))
	select {
	case <-transferFilesCommand.stopSignal:
		assert.Fail(t, "Signal was sent, but shouldn't be")
	default:
	}

	// Create the 'stop' file
	assert.NoError(t, transferFilesCommand.initTransferDir())
	assert.NoError(t, transferFilesCommand.stateManager.TryLockTransferStateManager())
	assert.NoError(t, transferFilesCommand.signalStop())

	// Run interruptIfRequested and make sure that the signal was sent to the channel
	assert.NoError(t, interruptIfRequested(transferFilesCommand.stopSignal))
	actualSignal, ok := <-transferFilesCommand.stopSignal
	assert.True(t, ok)
	assert.Equal(t, os.Interrupt, actualSignal)
}

func TestStoreStaleChunksEmpty(t *testing.T) {
	// Init state manager
	stateManager, cleanUp := state.InitStateTest(t)
	defer cleanUp()

	// Store empty stale chunks
	chunksLifeCycleManager := ChunksLifeCycleManager{
		nodeToChunksMap: make(map[nodeId]map[api.ChunkId]UploadedChunkData),
	}
	assert.NoError(t, chunksLifeCycleManager.StoreStaleChunks(stateManager))

	// Make sure no chunks
	staleChunks, err := stateManager.GetStaleChunks()
	assert.NoError(t, err)
	assert.Empty(t, staleChunks)
}

func TestStoreStaleChunksNoStale(t *testing.T) {
	// Init state manager
	stateManager, cleanUp := state.InitStateTest(t)
	defer cleanUp()

	// Store chunk that is not stale
	chunksLifeCycleManager := ChunksLifeCycleManager{
		nodeToChunksMap: map[nodeId]map[api.ChunkId]UploadedChunkData{
			staleChunksNodeIdOne: {
				staleChunksChunkId: {
					TimeSent:   time.Now().Add(-time.Minute),
					ChunkFiles: []api.FileRepresentation{{Repo: repo1Key, Path: staleChunksPath, Name: staleChunksName}},
				},
			},
		},
	}
	assert.NoError(t, chunksLifeCycleManager.StoreStaleChunks(stateManager))

	// Make sure no chunks
	staleChunks, err := stateManager.GetStaleChunks()
	assert.NoError(t, err)
	assert.Empty(t, staleChunks)
}

func TestStoreStaleChunksStale(t *testing.T) {
	// Init state manager
	stateManager, cleanUp := state.InitStateTest(t)
	defer cleanUp()

	// Store stale chunk
	sent := time.Now().Add(-time.Hour)
	chunksLifeCycleManager := ChunksLifeCycleManager{
		nodeToChunksMap: map[nodeId]map[api.ChunkId]UploadedChunkData{
			staleChunksNodeIdOne: {
				staleChunksChunkId: {
					TimeSent:   sent,
					ChunkFiles: []api.FileRepresentation{{Repo: repo1Key, Path: staleChunksPath, Name: staleChunksName, Size: 100}},
				},
			},
		},
	}
	assert.NoError(t, chunksLifeCycleManager.StoreStaleChunks(stateManager))

	// Make sure the stale chunk was stored in the state
	staleChunks, err := stateManager.GetStaleChunks()
	assert.NoError(t, err)
	assert.Len(t, staleChunks, 1)
	assert.Equal(t, staleChunksNodeIdOne, staleChunks[0].NodeID)
	assert.Len(t, staleChunks[0].Chunks, 1)
	assert.Equal(t, staleChunksChunkId, staleChunks[0].Chunks[0].ChunkID)
	assert.Equal(t, sent.Unix(), staleChunks[0].Chunks[0].Sent)
	assert.Len(t, staleChunks[0].Chunks[0].Files, 1)
	assert.Equal(t, fmt.Sprintf("%s/%s/%s (0.1KB)", repo1Key, staleChunksPath, staleChunksName), staleChunks[0].Chunks[0].Files[0])
}

func TestStoreStaleChunksTwoNodes(t *testing.T) {
	// Init state manager
	stateManager, cleanUp := state.InitStateTest(t)
	defer cleanUp()

	// Store 1 stale chunk and 1 non-stale chunk
	chunksLifeCycleManager := ChunksLifeCycleManager{
		nodeToChunksMap: map[nodeId]map[api.ChunkId]UploadedChunkData{
			staleChunksNodeIdOne: {
				staleChunksChunkId: {
					TimeSent:   time.Now().Add(-time.Hour), // Older than 0.5 hours
					ChunkFiles: []api.FileRepresentation{{Repo: repo1Key, Path: staleChunksPath, Name: staleChunksName, Size: 1024}},
				},
			},
			staleChunksNodeIdTwo: {
				staleChunksChunkId: {
					TimeSent:   time.Now(), // Less than 0.5 hours
					ChunkFiles: []api.FileRepresentation{{Repo: repo2Key, Path: staleChunksPath, Name: staleChunksName, Size: 0}},
				},
			},
		},
	}
	assert.NoError(t, chunksLifeCycleManager.StoreStaleChunks(stateManager))

	// Make sure only the stale chunk was stored in the state
	staleChunks, err := stateManager.GetStaleChunks()
	assert.NoError(t, err)
	assert.Len(t, staleChunks, 1)
	assert.Equal(t, staleChunksNodeIdOne, staleChunks[0].NodeID)
}

// Create mock server to test transfer config commands
// t           - The testing object
// testHandler - The HTTP handler of the test
func createMockServer(t *testing.T, testHandler transferFilesHandler) (*httptest.Server, *config.ServerDetails, *srcUserPluginService) {
	testServer := httptest.NewServer(http.HandlerFunc(testHandler))
	serverDetails := &config.ServerDetails{ArtifactoryUrl: testServer.URL + "/"}

	serviceManager, err := createSrcRtUserPluginServiceManager(context.Background(), serverDetails)
	assert.NoError(t, err)
	return testServer, serverDetails, serviceManager
}

func TestGetUniqueErrorOrDelayFilePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "unique_file_path_test")
	assert.NoError(t, err)

	createUniqueFileAndAssertCounter(t, tmpDir, "prefix", 0)
	// A file with 0 already exists, so new counter should be 1.
	createUniqueFileAndAssertCounter(t, tmpDir, "prefix", 1)
	// Unique prefix, so counter should be 0.
	createUniqueFileAndAssertCounter(t, tmpDir, "new", 0)

}

func createUniqueFileAndAssertCounter(t *testing.T, tmpDir, prefix string, expectedCounter int) {
	filePath, err := getUniqueErrorOrDelayFilePath(tmpDir, func() string {
		return prefix
	})
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(filePath, nil, 0644))
	assert.True(t, strings.HasSuffix(filePath, strconv.Itoa(expectedCounter)+".json"))
}
