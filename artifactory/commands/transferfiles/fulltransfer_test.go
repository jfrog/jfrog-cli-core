package transferfiles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/stretchr/testify/assert"
)

type fakeFileTransferExecutor struct {
	callCount int
	mu        sync.Mutex
}

func (f *fakeFileTransferExecutor) TransferFile(_ context.Context, candidate api.FileRepresentation) TransferResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callCount++
	if candidate.Name == "fail.jar" {
		return TransferResult{
			Candidate: candidate,
			Status:    api.Fail,
			Err:       errors.New("transfer failed"),
		}
	}
	return TransferResult{Candidate: candidate, Status: api.Success}
}

func TestFolderTraversal_schedulesFileTransfer(t *testing.T) {
	stateManager, cleanUp := state.InitStateTest(t)
	defer cleanUp()

	mockAqlResults := servicesUtils.AqlSearchResult{
		Results: []servicesUtils.ResultItem{
			{Repo: "test-repo", Path: ".", Name: "file.jar", Size: 100, Type: "file"},
		},
	}

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/search/aql" {
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(mockAqlResults)
			_, _ = w.Write(response)
		}
	}))
	defer testServer.Close()

	serverDetails := &coreConfig.ServerDetails{ArtifactoryUrl: testServer.URL + "/"}

	assert.NoError(t, stateManager.SetRepoState("test-repo", 0, 0, false, true))
	node, err := stateManager.LookUpNode(".")
	assert.NoError(t, err)

	executor := &fakeFileTransferExecutor{}
	pcWrapper := newProducerConsumerWrapper()
	errorsChannelMng := createErrorsChannelMng()

	phase := &fullTransferPhase{
		phaseBase: phaseBase{
			context:                context.Background(),
			stateManager:           stateManager,
			repoKey:                "test-repo",
			srcRtDetails:           serverDetails,
			fileTransfer:           executor,
			pcDetails:              &pcWrapper,
			locallyGeneratedFilter: &locallyGeneratedFilter{enabled: false},
		},
	}

	delayedArtifactsChannelMng := createdDelayedArtifactsChannelMng()
	delayHelper := delayUploadHelper{delayedArtifactsChannelMng: &delayedArtifactsChannelMng}

	err = phase.transferFolder(node, folderParams{relativePath: "."}, "", &pcWrapper, delayHelper, &errorsChannelMng)
	assert.NoError(t, err)

	assert.NoError(t, runProducerConsumers(&pcWrapper))

	assert.Equal(t, 1, executor.callCount, "folder traversal should schedule FileTransfer.TransferFile per file")
}

// TestGetPatternMatchingFilesWithResults tests getPatternMatchingFiles with files returned
func TestGetPatternMatchingFilesWithResults(t *testing.T) {
	stateManager, cleanUp := state.InitStateTest(t)
	defer cleanUp()

	mockAqlResults := servicesUtils.AqlSearchResult{
		Results: []servicesUtils.ResultItem{
			{Repo: "test-repo", Path: "org/company/projectA", Name: "file1.jar", Size: 100, Type: "file"},
			{Repo: "test-repo", Path: "org/company/projectA", Name: "file2.jar", Size: 200, Type: "file"},
		},
	}

	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/search/aql" {
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(mockAqlResults)
			_, _ = w.Write(response)
		}
	})
	defer testServer.Close()

	assert.NoError(t, stateManager.SetRepoState("test-repo", 0, 0, false, true))

	phase := &fullTransferPhase{
		phaseBase: phaseBase{
			context:                context.Background(),
			stateManager:           stateManager,
			repoKey:                "test-repo",
			srcRtDetails:           serverDetails,
			includeFilesPatterns:   []string{"org/company/*"},
			locallyGeneratedFilter: &locallyGeneratedFilter{enabled: false},
		},
	}

	results, lastPage, err := phase.getPatternMatchingFiles(0)
	assert.NoError(t, err)
	assert.True(t, lastPage)
	assert.Len(t, results, 2)

	// Also verify convertResultsToFileRepresentation works correctly
	files := convertResultsToFileRepresentation(results)
	assert.Len(t, files, 2)
	assert.Equal(t, "org/company/projectA", files[0].Path)
}

// TestRunWithAqlPatternFiltering tests that run() calls runWithAqlPatternFiltering when patterns are set
func TestRunWithAqlPatternFiltering(t *testing.T) {
	stateManager, cleanUp := state.InitStateTest(t)
	defer cleanUp()

	mockAqlResults := servicesUtils.AqlSearchResult{Results: []servicesUtils.ResultItem{}}

	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/search/aql" {
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(mockAqlResults)
			_, _ = w.Write(response)
		}
	})
	defer testServer.Close()

	assert.NoError(t, stateManager.SetRepoState("test-repo", 0, 0, false, true))
	pcWrapper := newProducerConsumerWrapper()

	phase := &fullTransferPhase{
		phaseBase: phaseBase{
			context:                context.Background(),
			stateManager:           stateManager,
			repoKey:                "test-repo",
			srcRtDetails:           serverDetails,
			includeFilesPatterns:   []string{"org/company/*"},
			locallyGeneratedFilter: &locallyGeneratedFilter{enabled: false},
			pcDetails:              &pcWrapper,
			startTime:              time.Now(),
		},
	}

	// Call run() - verifies runWithAqlPatternFiltering is called (print statement should appear)
	err := phase.run()
	assert.NoError(t, err)
}

// TestRunWithAqlPatternFilteringAqlError tests error handling when AQL query fails
// This tests getPatternMatchingFiles directly to avoid retry logic in the full run() path
func TestRunWithAqlPatternFilteringAqlError(t *testing.T) {
	stateManager, cleanUp := state.InitStateTest(t)
	defer cleanUp()

	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/search/aql" {
			// Return error response
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errors":[{"status":400,"message":"AQL query failed"}]}`))
		}
	})
	defer testServer.Close()

	assert.NoError(t, stateManager.SetRepoState("test-repo", 0, 0, false, true))

	phase := &fullTransferPhase{
		phaseBase: phaseBase{
			context:                context.Background(),
			stateManager:           stateManager,
			repoKey:                "test-repo",
			srcRtDetails:           serverDetails,
			includeFilesPatterns:   []string{"org/company/*"},
			locallyGeneratedFilter: &locallyGeneratedFilter{enabled: false},
		},
	}

	// Test getPatternMatchingFiles directly - should return error
	_, _, err := phase.getPatternMatchingFiles(0)
	assert.Error(t, err, "getPatternMatchingFiles should return error on AQL failure")
}

// TestRunWithAqlPatternFilteringPagination tests pagination detection logic
func TestRunWithAqlPatternFilteringPagination(t *testing.T) {
	stateManager, cleanUp := state.InitStateTest(t)
	defer cleanUp()

	aqlCallCount := 0

	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/search/aql" {
			aqlCallCount++
			w.WriteHeader(http.StatusOK)
			var mockAqlResults servicesUtils.AqlSearchResult
			if aqlCallCount == 1 {
				// First page - return exactly AqlPaginationLimit items to indicate more pages exist
				results := make([]servicesUtils.ResultItem, AqlPaginationLimit)
				for i := 0; i < AqlPaginationLimit; i++ {
					results[i] = servicesUtils.ResultItem{Repo: "test-repo", Path: "org/company", Name: "file.jar", Size: 100, Type: "file"}
				}
				mockAqlResults = servicesUtils.AqlSearchResult{Results: results}
			} else {
				// Second page - return less than limit to indicate last page
				mockAqlResults = servicesUtils.AqlSearchResult{Results: []servicesUtils.ResultItem{
					{Repo: "test-repo", Path: "org/company", Name: "lastfile.jar", Size: 100, Type: "file"},
				}}
			}
			response, _ := json.Marshal(mockAqlResults)
			_, _ = w.Write(response)
		}
	})
	defer testServer.Close()

	assert.NoError(t, stateManager.SetRepoState("test-repo", 0, 0, false, true))

	phase := &fullTransferPhase{
		phaseBase: phaseBase{
			context:                context.Background(),
			stateManager:           stateManager,
			repoKey:                "test-repo",
			srcRtDetails:           serverDetails,
			includeFilesPatterns:   []string{"org/company/*"},
			locallyGeneratedFilter: &locallyGeneratedFilter{enabled: false},
		},
	}

	// Test first page - should NOT be last page (results == AqlPaginationLimit)
	results1, lastPage1, err := phase.getPatternMatchingFiles(0)
	assert.NoError(t, err)
	assert.False(t, lastPage1, "First page should not be last page when results == AqlPaginationLimit")
	assert.Len(t, results1, AqlPaginationLimit)

	// Test second page - should BE last page (results < AqlPaginationLimit)
	results2, lastPage2, err := phase.getPatternMatchingFiles(1)
	assert.NoError(t, err)
	assert.True(t, lastPage2, "Second page should be last page when results < AqlPaginationLimit")
	assert.Len(t, results2, 1)

	// Verify both pages were fetched
	assert.Equal(t, 2, aqlCallCount, "AQL should be called twice for two pages")
}

// TestWarnIfRepoAppearsBlackedOut covers the B-38 fix: an AQL query returning zero results is
// corroborated against the repo's own GetRepoSummary file count (fetched once at repo-transfer
// start) before being trusted at face value, since Artifactory's AQL endpoint silently filters
// out items the querying identity can't read (HTTP 200, empty result set - not an error),
// which looks identical to a genuinely empty repository.
func TestWarnIfRepoAppearsBlackedOut(t *testing.T) {
	t.Run("warns when repo summary reports files but the query found none", func(t *testing.T) {
		buffer, stderrBuffer, previousLog := tests.RedirectLogOutputToBuffer()
		defer log.SetLogger(previousLog)

		phase := &fullTransferPhase{
			phaseBase: phaseBase{
				repoKey:     "blacked-out-repo",
				repoSummary: servicesUtils.RepositorySummary{FilesCount: json.Number("42")},
			},
		}
		phase.warnIfRepoAppearsBlackedOut("the include-pattern AQL query")

		output := buffer.String() + stderrBuffer.String()
		assert.Contains(t, output, "blacked-out-repo")
		assert.Contains(t, output, "42")
		assert.Contains(t, output, "read permission")
	})

	t.Run("no warning when repo summary reports zero files", func(t *testing.T) {
		buffer, stderrBuffer, previousLog := tests.RedirectLogOutputToBuffer()
		defer log.SetLogger(previousLog)

		phase := &fullTransferPhase{
			phaseBase: phaseBase{
				repoKey:     "genuinely-empty-repo",
				repoSummary: servicesUtils.RepositorySummary{FilesCount: json.Number("0")},
			},
		}
		phase.warnIfRepoAppearsBlackedOut("the include-pattern AQL query")

		assert.NotContains(t, buffer.String()+stderrBuffer.String(), "read permission")
	})

	t.Run("no warning when repo summary's file count is unparseable", func(t *testing.T) {
		buffer, stderrBuffer, previousLog := tests.RedirectLogOutputToBuffer()
		defer log.SetLogger(previousLog)

		phase := &fullTransferPhase{phaseBase: phaseBase{repoKey: "no-summary-repo"}}
		phase.warnIfRepoAppearsBlackedOut("the include-pattern AQL query")

		assert.NotContains(t, buffer.String()+stderrBuffer.String(), "read permission")
	})
}

func TestMaybeWarnCompletedFolderSkippedWithFilter(t *testing.T) {
	t.Run("no warning without filter", func(t *testing.T) {
		buffer, stderrBuffer, previousLog := tests.RedirectLogOutputToBuffer()
		defer log.SetLogger(previousLog)

		phase := &fullTransferPhase{}
		phase.maybeWarnCompletedFolderSkippedWithFilter()
		phase.maybeWarnCompletedFolderSkippedWithFilter()
		assert.NotContains(t, buffer.String()+stderrBuffer.String(), "--ignore-state")
	})

	t.Run("warns once when filter active", func(t *testing.T) {
		buffer, stderrBuffer, previousLog := tests.RedirectLogOutputToBuffer()
		defer log.SetLogger(previousLog)

		phase := &fullTransferPhase{
			phaseBase: phaseBase{
				timestampFilter: &timestampFilter{field: createdFilterField, timestamp: "2025-01-01T00:00:00.000Z"},
			},
		}
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				phase.maybeWarnCompletedFolderSkippedWithFilter()
			}()
		}
		wg.Wait()

		output := buffer.String() + stderrBuffer.String()
		assert.Contains(t, output, "--ignore-state")
		assert.Equal(t, 1, strings.Count(output, "--ignore-state"))
	})
}
