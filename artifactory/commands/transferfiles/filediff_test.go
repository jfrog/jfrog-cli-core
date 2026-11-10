package transferfiles

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
			results := generateDiffAqlQuery(repo1Key, "1", "2", testCase.paginationOffset, testCase.disabledDistinctiveAql, nil)
			assert.Equal(t, testCase.expectedAql, results)
		})
	}
}

func TestGenerateDiffAqlQueryWithTimestampFilter(t *testing.T) {
	createdFilter := &timestampFilter{field: createdFilterField, timestamp: "2025-01-01T00:00:00.000Z"}
	downloadedFilter := &timestampFilter{field: downloadedFilterField, timestamp: "2025-01-01T00:00:00.000Z"}

	t.Run("created after filters files keeps folders", func(t *testing.T) {
		query := generateDiffAqlQuery(repo1Key, "1", "2", 0, false, createdFilter)
		assert.Contains(t, query, `"modified":{"$gte":"1"}`)
		assert.Contains(t, query, `"modified":{"$lt":"2"}`)
		assert.Contains(t, query, `"repo":"repo1"`)
		assert.NotContains(t, query, `"type":"any"`)
		assert.Equal(t,
			`items.find({"$and":[{"modified":{"$gte":"1"}},{"modified":{"$lt":"2"}},{"repo":"repo1"},{"$or":[{"type":"folder"},{"$and":[{"type":"file"},{"created":{"$gte":"2025-01-01T00:00:00.000Z"}}]}]}]}).include("repo","path","name","type","modified","size").sort({"$asc":["name","path"]}).offset(0).limit(10000)`,
			query)
	})

	t.Run("downloaded after uses stat.downloaded", func(t *testing.T) {
		query := generateDiffAqlQuery(repo1Key, "1", "2", 0, false, downloadedFilter)
		assert.Contains(t, query, `,{"$or":[{"type":"folder"},{"$and":[{"type":"file"},{"stat.downloaded":{"$gte":"2025-01-01T00:00:00.000Z"}}]}]}`)
	})
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
			results := generateDockerManifestAqlQuery(repo1Key, "1", "2", testCase.paginationOffset, testCase.disabledDistinctiveAql, nil)
			assert.Equal(t, testCase.expectedAql, results)
		})
	}
}

func TestGenerateDockerManifestAqlQueryWithTimestampFilter(t *testing.T) {
	filter := &timestampFilter{field: createdFilterField, timestamp: "2025-01-01T00:00:00.000Z"}
	query := generateDockerManifestAqlQuery(repo1Key, "1", "2", 0, false, filter)
	assert.Contains(t, query, `"name":"manifest.json"`)
	assert.Contains(t, query, `"created":{"$gte":"2025-01-01T00:00:00.000Z"}`)
	assert.Contains(t, query, `"modified":{"$gte":"1"}`)
}

func TestGenerateGetDirContentAqlQueryUnfiltered(t *testing.T) {
	query := generateGetDirContentAqlQuery(repo1Key, []string{"library/nginx/1.0"})
	assert.NotContains(t, query, `"$gte"`)
	assert.NotContains(t, query, "stat.downloaded")
	assert.NotContains(t, query, `"created":{`)
	assert.Contains(t, query, `"path":{"$match":"library/nginx/1.0"}`)
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

// TestGetDockerTimeFrameFilesDiffListManifest verifies that list.manifest.json files are included
// in the result and transferred directly, without triggering a second AQL for directory contents.
// This is a regression test for the bug where the list.manifest.json case body was empty,
// causing these files to be silently dropped during delta sync (Phase 2).
func TestGetDockerTimeFrameFilesDiffListManifest(t *testing.T) {
	stateManager, cleanUp := state.InitStateTest(t)
	defer cleanUp()

	aqlCallCount := 0

	listManifestItem := servicesUtils.ResultItem{
		Repo: "docker-repo", Path: "myapp/sha256:abc123", Name: "list.manifest.json", Size: 500, Type: "file",
	}

	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/search/aql" {
			aqlCallCount++
			mockAqlResults := servicesUtils.AqlSearchResult{
				Results: []servicesUtils.ResultItem{listManifestItem},
			}
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(mockAqlResults)
			_, _ = w.Write(response)
		}
	})
	defer testServer.Close()

	assert.NoError(t, stateManager.SetRepoState("docker-repo", 0, 0, false, true))

	phase := &filesDiffPhase{
		phaseBase: phaseBase{
			context:      context.Background(),
			stateManager: stateManager,
			repoKey:      "docker-repo",
			srcRtDetails: serverDetails,
		},
	}

	result, err := phase.getDockerTimeFrameFilesDiff("2024-01-01T00:00:00Z", "2024-01-02T00:00:00Z", 0)
	assert.NoError(t, err)
	// Only one AQL call should be made — no second call for directory contents
	assert.Equal(t, 1, aqlCallCount, "Only one AQL call should be made for list.manifest.json files")
	// The list.manifest.json file must appear in the result
	assert.Len(t, result.Results, 1)
	assert.Equal(t, "list.manifest.json", result.Results[0].Name)
	assert.Equal(t, "myapp/sha256:abc123", result.Results[0].Path)
}

func generateManifestPaths(count, pathLen int) []string {
	paths := make([]string, count)
	for i := range paths {
		prefix := fmt.Sprintf("img%05d/", i)
		paths[i] = prefix + strings.Repeat("x", pathLen-len(prefix))
	}
	return paths
}

// JFMIG-145: a single AQL with a clause per manifest path exceeds the server limit (default 6000 chars).
func TestSplitPathsForDirContentAql(t *testing.T) {
	testCases := []struct {
		name  string
		count int
	}{
		{"empty", 0},
		{"single", 1},
		{"few", 10},
		{"over-limit-70-paths", 70},
		{"large-10000-paths", 10000},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			paths := generateManifestPaths(testCase.count, 40)
			batches := splitPathsForDirContentAql(repo1Key, paths, dirContentAqlMaxQueryLength)
			var merged []string
			for _, batch := range batches {
				assert.NotEmpty(t, batch)
				assert.LessOrEqual(t, len(generateGetDirContentAqlQuery(repo1Key, batch)), dirContentAqlMaxQueryLength)
				merged = append(merged, batch...)
			}
			// Complete, ordered, no duplicates.
			assert.Equal(t, len(paths), len(merged))
			if len(paths) > 0 {
				assert.Equal(t, paths, merged)
			}
		})
	}
}

// A single path longer than the budget must still be sent (alone), not dropped.
func TestSplitPathsForDirContentAqlOversizedPath(t *testing.T) {
	long := strings.Repeat("y", dirContentAqlMaxQueryLength+10)
	batches := splitPathsForDirContentAql(repo1Key, []string{"a", long, "b"}, dirContentAqlMaxQueryLength)
	assert.Equal(t, [][]string{{"a"}, {long}, {"b"}}, batches)
}

// JFMIG-145: with many changed tags in one window, every AQL stays under the server limit and no tag is lost.
func TestGetDockerTimeFrameFilesDiffManyManifests(t *testing.T) {
	stateManager, cleanUp := state.InitStateTest(t)
	defer cleanUp()

	const serverLimit = 7000
	const manifestCount = 500
	paths := generateManifestPaths(manifestCount, 40)
	var manifestItems []servicesUtils.ResultItem
	for _, p := range paths {
		manifestItems = append(manifestItems, servicesUtils.ResultItem{Repo: "docker-repo", Path: p, Name: "manifest.json", Type: "file"})
	}
	dirContentQueries := 0

	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI != "/api/search/aql" {
			return
		}
		body, _ := io.ReadAll(r.Body)
		query := string(body)
		if len(query) > serverLimit {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("AQL query is too long; please reduce the query length to less than 7000 chars"))
			return
		}
		var results []servicesUtils.ResultItem
		if strings.Contains(query, `"$match":"*"`) {
			dirContentQueries++
			for _, p := range paths {
				if strings.Contains(query, `"path":{"$match":"`+p+`"}`) {
					results = append(results, servicesUtils.ResultItem{Repo: "docker-repo", Path: p, Name: "manifest.json", Type: "file"},
						servicesUtils.ResultItem{Repo: "docker-repo", Path: p, Name: "layer", Type: "file"})
				}
			}
		} else {
			results = manifestItems
		}
		w.WriteHeader(http.StatusOK)
		response, _ := json.Marshal(servicesUtils.AqlSearchResult{Results: results})
		_, _ = w.Write(response)
	})
	defer testServer.Close()

	assert.NoError(t, stateManager.SetRepoState("docker-repo", 0, 0, false, true))
	phase := &filesDiffPhase{
		phaseBase: phaseBase{context: context.Background(), stateManager: stateManager, repoKey: "docker-repo", srcRtDetails: serverDetails},
	}

	result, err := phase.getDockerTimeFrameFilesDiff("2024-01-01T00:00:00Z", "2024-01-02T00:00:00Z", 0)
	assert.NoError(t, err)
	assert.Greater(t, dirContentQueries, 1, "expected the dir-content AQL to be split into batches")
	assert.Len(t, result.Results, manifestCount*2)
}
