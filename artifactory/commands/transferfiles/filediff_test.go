package transferfiles

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/stretchr/testify/assert"
)

var convertResultsToFileRepresentationTestCases = []struct {
	input          servicesUtils.ResultItem
	expectedOutput api.FileRepresentation
}{
	{
		servicesUtils.ResultItem{Repo: repo1Key, Path: "path-in-repo", Name: "file-name", Type: "file", Size: 100},
		api.FileRepresentation{Repo: repo1Key, Path: "path-in-repo", Name: "file-name", Size: 100},
	},
	{
		servicesUtils.ResultItem{Repo: repo1Key, Path: "path-in-repo", Name: "folder-name", Type: "folder"},
		api.FileRepresentation{Repo: repo1Key, Path: "path-in-repo/folder-name"},
	},
	{
		servicesUtils.ResultItem{Repo: repo1Key, Path: ".", Name: "folder-name", Type: "folder"},
		api.FileRepresentation{Repo: repo1Key, Path: "folder-name"},
	},
}

func TestConvertResultsToFileRepresentation(t *testing.T) {
	for _, testCase := range convertResultsToFileRepresentationTestCases {
		files := convertResultsToFileRepresentation([]servicesUtils.ResultItem{testCase.input})
		assert.Equal(t, []api.FileRepresentation{testCase.expectedOutput}, files)
	}
}

var generateDiffAqlQueryTestCases = []struct {
	paginationOffset       int
	disabledDistinctiveAql bool
	expectedAql            string
}{
	{0, false, "items.find({\"$and\":[{\"modified\":{\"$gte\":\"1\"}},{\"modified\":{\"$lt\":\"2\"}},{\"repo\":\"repo1\",\"type\":\"any\"}]}).include(\"repo\",\"path\",\"name\",\"type\",\"modified\",\"size\").sort({\"$asc\":[\"name\",\"path\"]}).offset(0).limit(10000)"},
	{0, true, "items.find({\"$and\":[{\"modified\":{\"$gte\":\"1\"}},{\"modified\":{\"$lt\":\"2\"}},{\"repo\":\"repo1\",\"type\":\"any\"}]}).include(\"repo\",\"path\",\"name\",\"type\",\"modified\",\"size\").sort({\"$asc\":[\"name\",\"path\"]}).offset(0).limit(10000).distinct(false)"},
	{2, false, "items.find({\"$and\":[{\"modified\":{\"$gte\":\"1\"}},{\"modified\":{\"$lt\":\"2\"}},{\"repo\":\"repo1\",\"type\":\"any\"}]}).include(\"repo\",\"path\",\"name\",\"type\",\"modified\",\"size\").sort({\"$asc\":[\"name\",\"path\"]}).offset(20000).limit(10000)"},
	{2, true, "items.find({\"$and\":[{\"modified\":{\"$gte\":\"1\"}},{\"modified\":{\"$lt\":\"2\"}},{\"repo\":\"repo1\",\"type\":\"any\"}]}).include(\"repo\",\"path\",\"name\",\"type\",\"modified\",\"size\").sort({\"$asc\":[\"name\",\"path\"]}).offset(20000).limit(10000).distinct(false)"},
}

func TestGenerateDiffAqlQuery(t *testing.T) {
	for _, testCase := range generateDiffAqlQueryTestCases {
		t.Run("", func(*testing.T) {
			results := generateDiffAqlQuery(repo1Key, "1", "2", testCase.paginationOffset, testCase.disabledDistinctiveAql)
			assert.Equal(t, testCase.expectedAql, results)
		})
	}
}

var generateDockerManifestAqlQueryTestCases = []struct {
	paginationOffset       int
	disabledDistinctiveAql bool
	expectedAql            string
}{
	{0, false, "items.find({\"$and\":[{\"repo\":\"repo1\"},{\"modified\":{\"$gte\":\"1\"}},{\"modified\":{\"$lt\":\"2\"}},{\"$or\":[{\"name\":\"manifest.json\"},{\"name\":\"list.manifest.json\"}]}]}).include(\"repo\",\"path\",\"name\",\"type\",\"modified\").sort({\"$asc\":[\"name\",\"path\"]}).offset(0).limit(10000)"},
	{0, true, "items.find({\"$and\":[{\"repo\":\"repo1\"},{\"modified\":{\"$gte\":\"1\"}},{\"modified\":{\"$lt\":\"2\"}},{\"$or\":[{\"name\":\"manifest.json\"},{\"name\":\"list.manifest.json\"}]}]}).include(\"repo\",\"path\",\"name\",\"type\",\"modified\").sort({\"$asc\":[\"name\",\"path\"]}).offset(0).limit(10000).distinct(false)"},
	{2, false, "items.find({\"$and\":[{\"repo\":\"repo1\"},{\"modified\":{\"$gte\":\"1\"}},{\"modified\":{\"$lt\":\"2\"}},{\"$or\":[{\"name\":\"manifest.json\"},{\"name\":\"list.manifest.json\"}]}]}).include(\"repo\",\"path\",\"name\",\"type\",\"modified\").sort({\"$asc\":[\"name\",\"path\"]}).offset(20000).limit(10000)"},
	{2, true, "items.find({\"$and\":[{\"repo\":\"repo1\"},{\"modified\":{\"$gte\":\"1\"}},{\"modified\":{\"$lt\":\"2\"}},{\"$or\":[{\"name\":\"manifest.json\"},{\"name\":\"list.manifest.json\"}]}]}).include(\"repo\",\"path\",\"name\",\"type\",\"modified\").sort({\"$asc\":[\"name\",\"path\"]}).offset(20000).limit(10000).distinct(false)"},
}

func TestGenerateDockerManifestAqlQuery(t *testing.T) {
	for _, testCase := range generateDockerManifestAqlQueryTestCases {
		t.Run("", func(*testing.T) {
			results := generateDockerManifestAqlQuery(repo1Key, "1", "2", testCase.paginationOffset, testCase.disabledDistinctiveAql)
			assert.Equal(t, testCase.expectedAql, results)
		})
	}
}

// TestGetNonDockerTimeFrameFilesDiffWithPatterns tests that getNonDockerTimeFrameFilesDiff uses pattern filtering when patterns are set
func TestGetNonDockerTimeFrameFilesDiffWithPatterns(t *testing.T) {
	stateManager, cleanUp := state.InitStateTest(t)
	defer cleanUp()

	aqlCalled := false
	receivedQuery := ""

	mockAqlResults := servicesUtils.AqlSearchResult{
		Results: []servicesUtils.ResultItem{
			{Repo: "test-repo", Path: "org/company/projectA", Name: "file1.jar", Size: 100, Type: "file"},
		},
	}

	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/search/aql" {
			aqlCalled = true
			// Read the query body
			buf := make([]byte, 1024)
			n, _ := r.Body.Read(buf)
			receivedQuery = string(buf[:n])

			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(mockAqlResults)
			_, _ = w.Write(response)
		}
	})
	defer testServer.Close()

	assert.NoError(t, stateManager.SetRepoState("test-repo", 0, 0, false, true))

	phase := &filesDiffPhase{
		phaseBase: phaseBase{
			context:              context.Background(),
			stateManager:         stateManager,
			repoKey:              "test-repo",
			srcRtDetails:         serverDetails,
			includeFilesPatterns: []string{"org/company/*"},
		},
	}

	// Call getNonDockerTimeFrameFilesDiff with patterns
	result, err := phase.getNonDockerTimeFrameFilesDiff("2024-01-01T00:00:00Z", "2024-01-02T00:00:00Z", 0)
	assert.NoError(t, err)
	assert.True(t, aqlCalled, "AQL should be called")
	assert.Len(t, result.Results, 1)

	// Verify the query contains pattern filtering
	assert.Contains(t, receivedQuery, "$or", "Query should contain pattern $or condition")
	assert.Contains(t, receivedQuery, "org/company", "Query should contain the pattern")
}

// TestGetNonDockerTimeFrameFilesDiffWithoutPatterns tests that getNonDockerTimeFrameFilesDiff uses default query when no patterns
func TestGetNonDockerTimeFrameFilesDiffWithoutPatterns(t *testing.T) {
	stateManager, cleanUp := state.InitStateTest(t)
	defer cleanUp()

	aqlCalled := false
	receivedQuery := ""

	mockAqlResults := servicesUtils.AqlSearchResult{
		Results: []servicesUtils.ResultItem{
			{Repo: "test-repo", Path: "any/path", Name: "file1.jar", Size: 100, Type: "file"},
		},
	}

	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/search/aql" {
			aqlCalled = true
			buf := make([]byte, 1024)
			n, _ := r.Body.Read(buf)
			receivedQuery = string(buf[:n])

			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(mockAqlResults)
			_, _ = w.Write(response)
		}
	})
	defer testServer.Close()

	assert.NoError(t, stateManager.SetRepoState("test-repo", 0, 0, false, true))

	phase := &filesDiffPhase{
		phaseBase: phaseBase{
			context:              context.Background(),
			stateManager:         stateManager,
			repoKey:              "test-repo",
			srcRtDetails:         serverDetails,
			includeFilesPatterns: []string{}, // No patterns
		},
	}

	// Call getNonDockerTimeFrameFilesDiff without patterns
	result, err := phase.getNonDockerTimeFrameFilesDiff("2024-01-01T00:00:00Z", "2024-01-02T00:00:00Z", 0)
	assert.NoError(t, err)
	assert.True(t, aqlCalled, "AQL should be called")
	assert.Len(t, result.Results, 1)

	// Verify the query does NOT contain pattern filtering $or
	assert.NotContains(t, receivedQuery, `"$or":[{"path"`, "Query should not contain pattern $or condition when no patterns set")
}

// TestGetDockerTimeFrameFilesDiffWithPatterns tests that getDockerTimeFrameFilesDiff uses pattern filtering when patterns are set
func TestGetDockerTimeFrameFilesDiffWithPatterns(t *testing.T) {
	stateManager, cleanUp := state.InitStateTest(t)
	defer cleanUp()

	aqlCallCount := 0
	firstQuery := ""

	// First call returns manifest files, second call returns dir content
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/search/aql" {
			aqlCallCount++
			buf := make([]byte, 2048)
			n, _ := r.Body.Read(buf)
			query := string(buf[:n])

			if aqlCallCount == 1 {
				// First query - capture for assertion
				firstQuery = query
				// Return manifest file
				mockAqlResults := servicesUtils.AqlSearchResult{
					Results: []servicesUtils.ResultItem{
						{Repo: "docker-repo", Path: "myapp/1.0", Name: "manifest.json", Size: 100, Type: "file"},
					},
				}
				w.WriteHeader(http.StatusOK)
				response, _ := json.Marshal(mockAqlResults)
				_, _ = w.Write(response)
			} else {
				// Second query - return dir content
				mockAqlResults := servicesUtils.AqlSearchResult{
					Results: []servicesUtils.ResultItem{
						{Repo: "docker-repo", Path: "myapp/1.0", Name: "layer1.tar", Size: 1000, Type: "file"},
					},
				}
				w.WriteHeader(http.StatusOK)
				response, _ := json.Marshal(mockAqlResults)
				_, _ = w.Write(response)
			}
		}
	})
	defer testServer.Close()

	assert.NoError(t, stateManager.SetRepoState("docker-repo", 0, 0, false, true))

	phase := &filesDiffPhase{
		phaseBase: phaseBase{
			context:              context.Background(),
			stateManager:         stateManager,
			repoKey:              "docker-repo",
			srcRtDetails:         serverDetails,
			includeFilesPatterns: []string{"myapp/*"},
		},
	}

	// Call getDockerTimeFrameFilesDiff with patterns
	result, err := phase.getDockerTimeFrameFilesDiff("2024-01-01T00:00:00Z", "2024-01-02T00:00:00Z", 0)
	assert.NoError(t, err)
	assert.Equal(t, 2, aqlCallCount, "Two AQL calls should be made (manifest + dir content)")
	assert.Len(t, result.Results, 1)

	// Verify the FIRST query contains pattern filtering and docker manifest names
	assert.Contains(t, firstQuery, "myapp", "First query should contain the pattern")
	assert.Contains(t, firstQuery, "manifest.json", "First query should contain manifest.json")
}
