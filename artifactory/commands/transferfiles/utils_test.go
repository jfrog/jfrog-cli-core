package transferfiles

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	artifactoryutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	clientutilstests "github.com/jfrog/jfrog-client-go/utils/tests"
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

func TestInitTempDir(t *testing.T) {
	// Create JFrog home
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	// Set the temp dir to be <jfrog-home>/transfer/tmp/
	unsetTempDir, err := initTempDir()
	assert.NoError(t, err)

	// Assert temp dir base path contain transfer/tmp
	assert.Contains(t, fileutils.GetTempDirBase(), filepath.Join("transfer", "tmp"))

	// Unset temp dir and assert that it is not contain transfer/tmp
	unsetTempDir()
	assert.NotContains(t, fileutils.GetTempDirBase(), filepath.Join("transfer", "tmp"))
}

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
		_, err := fmt.Fprintf(w, `{"node_id": "%s"}`, nodeId)
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
		_, err := w.Write([]byte(response)) // #nosec G705 -- test server response, not user input
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
		_, err = fmt.Fprintf(w, "Repository %s-local update successfully.", packageType)
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

func TestGetNodeIdToChunkIdsMap(t *testing.T) {
	// Test empty ChunksLifeCycleManager
	chunksLifeCycleManager := ChunksLifeCycleManager{}
	assert.Empty(t, chunksLifeCycleManager.GetNodeIdToChunkIdsMap())

	// Create ChunksLifeCycleManager with 3 nodes
	chunksLifeCycleManager = ChunksLifeCycleManager{
		nodeToChunksMap: make(map[api.NodeId]map[api.ChunkId]UploadedChunkData),
	}
	chunksLifeCycleManager.nodeToChunksMap["nodeId-1"] = map[api.ChunkId]UploadedChunkData{"0": {}, "1": {}}
	chunksLifeCycleManager.nodeToChunksMap["nodeId-2"] = map[api.ChunkId]UploadedChunkData{"2": {}}
	chunksLifeCycleManager.nodeToChunksMap["nodeId-3"] = map[api.ChunkId]UploadedChunkData{}

	// Generate the map and check response
	nodeIdToChunkIdsMap := chunksLifeCycleManager.GetNodeIdToChunkIdsMap()
	assert.ElementsMatch(t, nodeIdToChunkIdsMap["nodeId-1"], []api.ChunkId{"0", "1"})
	assert.ElementsMatch(t, nodeIdToChunkIdsMap["nodeId-2"], []api.ChunkId{"2"})
	assert.ElementsMatch(t, nodeIdToChunkIdsMap["nodeId-3"], []api.ChunkId{})
}

func TestStoreStaleChunksEmpty(t *testing.T) {
	// Init state manager
	stateManager, cleanUp := state.InitStateTest(t)
	defer cleanUp()

	// Store empty stale chunks
	chunksLifeCycleManager := ChunksLifeCycleManager{
		nodeToChunksMap: make(map[api.NodeId]map[api.ChunkId]UploadedChunkData),
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
		nodeToChunksMap: map[api.NodeId]map[api.ChunkId]UploadedChunkData{
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
		nodeToChunksMap: map[api.NodeId]map[api.ChunkId]UploadedChunkData{
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
		nodeToChunksMap: map[api.NodeId]map[api.ChunkId]UploadedChunkData{
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

var updateThreadsProvider = []struct {
	threadsNumber                int
	expectedChunkBuilderThreads  int
	expectedChunkUploaderThreads int
	buildInfo                    bool
}{
	{artifactoryutils.DefaultThreads - 1, artifactoryutils.DefaultThreads - 1, artifactoryutils.DefaultThreads - 1, false},
	{artifactoryutils.DefaultThreads, artifactoryutils.DefaultThreads, artifactoryutils.DefaultThreads, false},
	{artifactoryutils.MaxBuildInfoThreads + 1, artifactoryutils.MaxBuildInfoThreads + 1, artifactoryutils.MaxBuildInfoThreads + 1, false},
	{artifactoryutils.MaxChunkBuilderThreads + 1, artifactoryutils.MaxChunkBuilderThreads, artifactoryutils.MaxChunkBuilderThreads + 1, false},

	{artifactoryutils.DefaultThreads - 1, artifactoryutils.DefaultThreads - 1, artifactoryutils.DefaultThreads - 1, true},
	{artifactoryutils.DefaultThreads, artifactoryutils.DefaultThreads, artifactoryutils.DefaultThreads, true},
	{artifactoryutils.MaxBuildInfoThreads + 1, artifactoryutils.MaxBuildInfoThreads, artifactoryutils.MaxBuildInfoThreads, true},
	{artifactoryutils.MaxChunkBuilderThreads + 1, artifactoryutils.MaxBuildInfoThreads, artifactoryutils.MaxBuildInfoThreads, true},
}

func TestUpdateThreads(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	previousLog := clientutilstests.RedirectLogOutputToNil()
	defer func() {
		log.SetLogger(previousLog)
	}()

	for _, testCase := range updateThreadsProvider {
		t.Run(strconv.Itoa(testCase.threadsNumber)+" Build Info: "+strconv.FormatBool(testCase.buildInfo), func(t *testing.T) {
			transferSettings := &artifactoryutils.TransferSettings{ThreadsNumber: testCase.threadsNumber}
			assert.NoError(t, artifactoryutils.SaveTransferSettings(transferSettings))

			assert.NoError(t, updateThreads(nil, testCase.buildInfo))
			assert.Equal(t, testCase.expectedChunkBuilderThreads, curChunkBuilderThreads)
			assert.Equal(t, testCase.expectedChunkUploaderThreads, curChunkUploaderThreads)
		})
	}
}

// Test cases for convertPatternToPathPrefix
var convertPatternToPathPrefixTestCases = []struct {
	input    string
	expected string
}{
	{"folder/subfolder/*", "folder/subfolder"},  // strips trailing /*
	{"folder/**", "folder"},                     // strips trailing /**
	{"folder/", "folder"},                       // strips trailing /
	{"folder", "folder"},                        // no change when no trailing pattern
	{"a/b/c/d/e/*", "a/b/c/d/e"},                // deep path with wildcard
	{"single", "single"},                        // single segment without slash
}

func TestConvertPatternToPathPrefix(t *testing.T) {
	for _, testCase := range convertPatternToPathPrefixTestCases {
		t.Run(testCase.input, func(t *testing.T) {
			result := convertPatternToPathPrefix(testCase.input)
			assert.Equal(t, testCase.expected, result)
		})
	}
}

// Test cases for matchIncludeFilesPattern
var matchIncludeFilesPatternTestCases = []struct {
	name     string
	path     string
	patterns []string
	expected bool
}{
	// Empty patterns should match everything
	{"empty patterns matches all", "org/company/projectA/file.jar", []string{}, true},

	// Single pattern matching
	{"single pattern match", "org/company/projectA/file.jar", []string{"org/company/*"}, true},
	{"single pattern no match", "org/external/lib/file.jar", []string{"org/company/*"}, false},

	// Multiple patterns (OR logic)
	{"multiple patterns first matches", "org/company/projectA/file.jar", []string{"org/company/*", "com/*"}, true},
	{"multiple patterns second matches", "com/example/app/file.jar", []string{"org/company/*", "com/*"}, true},
	{"multiple patterns none match", "other/path/file.jar", []string{"org/company/*", "com/*"}, false},

	// Edge cases
	{"root level file no match", "file.jar", []string{"org/*"}, false},
}

func TestMatchIncludeFilesPattern(t *testing.T) {
	for _, testCase := range matchIncludeFilesPatternTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := matchIncludeFilesPattern(testCase.path, testCase.patterns)
			assert.Equal(t, testCase.expected, result)
		})
	}
}

// Test cases for filterFilesByPattern
func TestFilterFilesByPattern(t *testing.T) {
	files := []api.FileRepresentation{
		{Repo: "repo1", Path: "org/company/projectA", Name: "file1.jar", Size: 100},
		{Repo: "repo1", Path: "org/company/projectA", Name: "file2.jar", Size: 200},
		{Repo: "repo1", Path: "org/company/projectB", Name: "file3.jar", Size: 300},
		{Repo: "repo1", Path: "org/external/lib", Name: "external.jar", Size: 400},
		{Repo: "repo1", Path: "com/example/app", Name: "app.jar", Size: 500},
	}

	testCases := []struct {
		name          string
		patterns      []string
		expectedCount int
		expectedPaths []string
	}{
		{
			name:          "empty patterns returns all",
			patterns:      []string{},
			expectedCount: 5,
			expectedPaths: []string{"org/company/projectA", "org/company/projectA", "org/company/projectB", "org/external/lib", "com/example/app"},
		},
		{
			name:          "filter org/company only",
			patterns:      []string{"org/company/*"},
			expectedCount: 3,
			expectedPaths: []string{"org/company/projectA", "org/company/projectA", "org/company/projectB"},
		},
		{
			name:          "filter specific project",
			patterns:      []string{"org/company/projectA/*"},
			expectedCount: 2,
			expectedPaths: []string{"org/company/projectA", "org/company/projectA"},
		},
		{
			name:          "filter multiple patterns",
			patterns:      []string{"org/company/projectA/*", "com/*"},
			expectedCount: 3,
			expectedPaths: []string{"org/company/projectA", "org/company/projectA", "com/example/app"},
		},
		{
			name:          "filter with no matches",
			patterns:      []string{"nonexistent/*"},
			expectedCount: 0,
			expectedPaths: []string{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := filterFilesByPattern(files, testCase.patterns)
			assert.Equal(t, testCase.expectedCount, len(result))
			for i, file := range result {
				assert.Equal(t, testCase.expectedPaths[i], file.Path)
			}
		})
	}
}

// Test cases for convertPatternToAqlMatch
var convertPatternToAqlMatchTestCases = []struct {
	input    string
	expected string
}{
	{"folder/subfolder/*", "*folder/subfolder*"},  // path with wildcard
	{"folder", "*folder*"},                        // simple folder name
	{"org/company/project/*", "*org/company/project*"}, // deep nested path
	{"*already/prefixed", "*already/prefixed*"},   // already has leading wildcard
	{"already/suffixed*", "*already/suffixed*"},   // already has trailing wildcard
}

func TestConvertPatternToAqlMatch(t *testing.T) {
	for _, testCase := range convertPatternToAqlMatchTestCases {
		t.Run(testCase.input, func(t *testing.T) {
			result := convertPatternToAqlMatch(testCase.input)
			assert.Equal(t, testCase.expected, result)
		})
	}
}

// Test cases for generatePatternConditionsAql
func TestGeneratePatternConditionsAql(t *testing.T) {
	testCases := []struct {
		name     string
		patterns []string
		expected string
	}{
		{
			name:     "empty patterns",
			patterns: []string{},
			expected: "",
		},
		{
			name:     "single pattern",
			patterns: []string{"org/company/*"},
			expected: `,"$or":[{"path":{"$match":"*org/company*"}}]`,
		},
		{
			name:     "two patterns",
			patterns: []string{"org/company/*", "com/jfrog/*"},
			expected: `,"$or":[{"path":{"$match":"*org/company*"}},{"path":{"$match":"*com/jfrog*"}}]`,
		},
		{
			name:     "three patterns",
			patterns: []string{"org/company/*", "com/jfrog/*", "io/netty/*"},
			expected: `,"$or":[{"path":{"$match":"*org/company*"}},{"path":{"$match":"*com/jfrog*"}},{"path":{"$match":"*io/netty*"}}]`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := generatePatternConditionsAql(testCase.patterns)
			assert.Equal(t, testCase.expected, result)
		})
	}
}

// Test cases for generatePatternOrConditionForAnd
func TestGeneratePatternOrConditionForAnd(t *testing.T) {
	testCases := []struct {
		name     string
		patterns []string
		expected string
	}{
		{
			name:     "empty patterns",
			patterns: []string{},
			expected: "",
		},
		{
			name:     "single pattern",
			patterns: []string{"org/company/*"},
			expected: `,{"$or":[{"path":{"$match":"*org/company*"}}]}`,
		},
		{
			name:     "two patterns",
			patterns: []string{"org/company/*", "com/jfrog/*"},
			expected: `,{"$or":[{"path":{"$match":"*org/company*"}},{"path":{"$match":"*com/jfrog*"}}]}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := generatePatternOrConditionForAnd(testCase.patterns)
			assert.Equal(t, testCase.expected, result)
		})
	}
}

// Test cases for generatePatternBasedAqlQuery
func TestGeneratePatternBasedAqlQuery(t *testing.T) {
	testCases := []struct {
		name             string
		repoKey          string
		patterns         []string
		paginationOffset int
		expectedContains []string
	}{
		{
			name:             "single pattern",
			repoKey:          "test-repo",
			patterns:         []string{"org/company/*"},
			paginationOffset: 0,
			expectedContains: []string{
				`"type":"file"`,
				`"repo":"test-repo"`,
				`"$or":[{"path":{"$match":"*org/company*"}}]`,
				`.include("repo","path","name","type","size")`,
				`.sort({"$asc":["path","name"]})`,
				`.offset(0)`,
			},
		},
		{
			name:             "multiple patterns with pagination",
			repoKey:          "my-repo",
			patterns:         []string{"com/jfrog/*", "org/apache/*"},
			paginationOffset: 2,
			expectedContains: []string{
				`"repo":"my-repo"`,
				`"$or":[{"path":{"$match":"*com/jfrog*"}},{"path":{"$match":"*org/apache*"}}]`,
				fmt.Sprintf(`.offset(%d)`, 2*AqlPaginationLimit),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := generatePatternBasedAqlQuery(testCase.repoKey, testCase.patterns, testCase.paginationOffset, false)
			for _, expected := range testCase.expectedContains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

// Test cases for generateDiffAqlQueryWithPatterns
func TestGenerateDiffAqlQueryWithPatterns(t *testing.T) {
	fromTimestamp := "2024-01-01T00:00:00Z"
	toTimestamp := "2024-01-02T00:00:00Z"

	testCases := []struct {
		name             string
		repoKey          string
		patterns         []string
		expectedContains []string
	}{
		{
			name:     "with patterns",
			repoKey:  "test-repo",
			patterns: []string{"org/company/*"},
			expectedContains: []string{
				`"modified":{"$gte":"2024-01-01T00:00:00Z"}`,
				`"modified":{"$lt":"2024-01-02T00:00:00Z"}`,
				`"repo":"test-repo"`,
				`{"$or":[{"path":{"$match":"*org/company*"}}]}`,
			},
		},
		{
			name:     "multiple patterns",
			repoKey:  "my-repo",
			patterns: []string{"com/jfrog/*", "org/apache/*"},
			expectedContains: []string{
				`"repo":"my-repo"`,
				`{"path":{"$match":"*com/jfrog*"}}`,
				`{"path":{"$match":"*org/apache*"}}`,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := generateDiffAqlQueryWithPatterns(testCase.repoKey, fromTimestamp, toTimestamp, testCase.patterns, 0, false)
			for _, expected := range testCase.expectedContains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

// Test cases for generateDockerManifestAqlQueryWithPatterns
func TestGenerateDockerManifestAqlQueryWithPatterns(t *testing.T) {
	fromTimestamp := "2024-01-01T00:00:00Z"
	toTimestamp := "2024-01-02T00:00:00Z"

	testCases := []struct {
		name             string
		repoKey          string
		patterns         []string
		expectedContains []string
	}{
		{
			name:     "docker manifest with patterns",
			repoKey:  "docker-repo",
			patterns: []string{"myapp/*"},
			expectedContains: []string{
				`"repo":"docker-repo"`,
				`"name":"manifest.json"`,
				`"name":"list.manifest.json"`,
				`{"$or":[{"path":{"$match":"*myapp*"}}]}`,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := generateDockerManifestAqlQueryWithPatterns(testCase.repoKey, fromTimestamp, toTimestamp, testCase.patterns, 0, false)
			for _, expected := range testCase.expectedContains {
				assert.Contains(t, result, expected)
			}
		})
	}
}
