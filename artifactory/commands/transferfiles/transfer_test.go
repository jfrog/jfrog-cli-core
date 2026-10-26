package transferfiles

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gocarina/gocsv"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	coreUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	artifactoryUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSetup_usesTargetPing(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	require.NoError(t, err)
	defer cleanUpJfrogHome()

	var targetPingCalls atomic.Int32
	var pluginExecuteCalls atomic.Int32

	sourceServer, sourceDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.RequestURI, "api/plugins/execute") {
			pluginExecuteCalls.Add(1)
		}
		switch r.RequestURI {
		case "/api/system/version":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"version":"7.90.0"}`))
			assert.NoError(t, err)
		case "/api/repositories?type=local":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`[]`))
			assert.NoError(t, err)
		case "/api/repositories?type=federated":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`[]`))
			assert.NoError(t, err)
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
	defer sourceServer.Close()

	targetServer, targetDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.RequestURI, "api/plugins/execute") {
			pluginExecuteCalls.Add(1)
		}
		if r.RequestURI == "/api/system/ping" {
			targetPingCalls.Add(1)
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("OK"))
			assert.NoError(t, err)
		}
	})
	defer targetServer.Close()

	cmd, err := NewTransferFilesCommand(sourceDetails, targetDetails)
	require.NoError(t, err)
	cmd.SetPreChecks(true)

	err = cmd.Run()
	require.NoError(t, err)
	assert.Greater(t, targetPingCalls.Load(), int32(0), "command setup should ping target /api/system/ping")
	assert.Equal(t, int32(0), pluginExecuteCalls.Load(), "command setup must not call /api/plugins/execute")
}

func TestRun_schemeLessProxyKeyWithoutSourceFailsClosed(t *testing.T) {
	cmd, err := NewTransferFilesCommand(nil, nil)
	require.NoError(t, err)
	cmd.SetProxyKey("source-to-target")

	err = cmd.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"source-to-target"`)
	assert.Contains(t, err.Error(), "looked up on the source Artifactory")
	assert.Contains(t, err.Error(), "source Artifactory is not configured")
	assert.Contains(t, err.Error(), "http://proxy:3128")
	assert.Contains(t, err.Error(), "HTTPS_PROXY")
	assert.Contains(t, err.Error(), "NO_PROXY")
}

func TestHandleStopInitAndClose(t *testing.T) {
	transferFilesCommand, err := NewTransferFilesCommand(nil, nil)
	assert.NoError(t, err)
	finishStopping, _ := transferFilesCommand.handleStop()
	finishStopping()
}

func TestCancelFunc(t *testing.T) {
	transferFilesCommand, err := NewTransferFilesCommand(nil, nil)
	assert.NoError(t, err)
	assert.False(t, transferFilesCommand.shouldStop())

	transferFilesCommand.cancelFunc()
	assert.True(t, transferFilesCommand.shouldStop())
}

func TestCancelFuncCancelsInFlightTransferFile(t *testing.T) {
	transferFilesCommand, err := NewTransferFilesCommand(nil, nil)
	require.NoError(t, err)

	source := &cancelAwareTransferSource{started: make(chan struct{})}
	fileTransfer := NewFileTransfer(source, &mockTransferTarget{}, FileTransferOptions{})
	resultChannel := make(chan TransferResult, 1)
	go func() {
		resultChannel <- fileTransfer.TransferFile(transferFilesCommand.context, testFileCandidate())
	}()

	<-source.started
	transferFilesCommand.cancelFunc()

	select {
	case result := <-resultChannel:
		require.ErrorIs(t, result.Err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight TransferFile did not return promptly after stop")
	}
}

func TestHandleStopDumpsThreadsBeforeCancelingInFlightTransferFile(t *testing.T) {
	transferFilesCommand, err := NewTransferFilesCommand(nil, nil)
	require.NoError(t, err)
	dumpStarted := make(chan struct{})
	finishDump := make(chan struct{})
	transferFilesCommand.threadDump = func() error {
		close(dumpStarted)
		<-finishDump
		return nil
	}
	finishStopping, _ := transferFilesCommand.handleStop()
	defer finishStopping()

	source := &cancelAwareTransferSource{started: make(chan struct{})}
	fileTransfer := NewFileTransfer(source, &mockTransferTarget{}, FileTransferOptions{})
	resultChannel := make(chan TransferResult, 1)
	go func() {
		resultChannel <- fileTransfer.TransferFile(transferFilesCommand.context, testFileCandidate())
	}()

	<-source.started
	transferFilesCommand.stopSignal <- os.Interrupt
	<-dumpStarted
	select {
	case <-transferFilesCommand.context.Done():
		t.Fatal("transfer context was canceled before thread dump finished")
	default:
	}

	close(finishDump)
	select {
	case result := <-resultChannel:
		require.ErrorIs(t, result.Err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight TransferFile did not return promptly after thread dump finished")
	}
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
			response := &artifactoryUtils.StorageInfo{RepositoriesSummaryList: []artifactoryUtils.RepositorySummary{
				{RepoKey: "repo-1"}, {RepoKey: "repo-2"},
				{RepoKey: "federated-repo-1"}, {RepoKey: "federated-repo-2"},
				{RepoKey: "artifactory-build-info", PackageType: "BuildInfo"}, {RepoKey: "proj-build-info", PackageType: "BuildInfo"}},
			}
			bytes, err := json.Marshal(response)
			assert.NoError(t, err)
			_, err = w.Write(bytes)
			assert.NoError(t, err)
		case "/api/repositories?type=local":
			// Response for GetWithFilter
			w.WriteHeader(http.StatusOK)
			response := &[]services.RepositoryDetails{{Key: "repo-1"}, {Key: "repo-2"}}
			bytes, err := json.Marshal(response)
			assert.NoError(t, err)
			_, err = w.Write(bytes)
			assert.NoError(t, err)
		case "/api/repositories?type=federated":
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
	localRepos, localBuildInfoRepo, err := transferFilesCommand.getAllLocalRepos(serverDetails, storageInfoManager, false)
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

// TestRemoveOldFilesIfNeeded_ignoreState_clearsSkippedErrors: --ignore-state starts clean, so the
// skipped-errors rows of earlier runs (e.g. "source item gone") must go too; otherwise a clean rerun
// reports stale rows in its errors CSV (M1).
func TestRemoveOldFilesIfNeeded_ignoreState_clearsSkippedErrors(t *testing.T) {
	for _, ignoreState := range []bool{true, false} {
		t.Run(fmt.Sprintf("ignoreState=%v", ignoreState), func(t *testing.T) {
			cleanUpJfrogHome, err := tests.SetJfrogHome()
			require.NoError(t, err)
			defer cleanUpJfrogHome()
			writeErrorRows(t, testRepoKey, 1, goneRow(testRepoKey, "a.bin"), api.FileUploadStatusResponse{
				FileRepresentation: api.FileRepresentation{Repo: testRepoKey, Path: "path", Name: "failed.bin"}, Status: api.Fail, Reason: "boom"})

			cmd, err := NewTransferFilesCommand(nil, nil)
			require.NoError(t, err)
			cmd.ignoreState = ignoreState
			require.NoError(t, cmd.removeOldFilesIfNeeded([]string{testRepoKey}))

			skipped, err := getErrorsFiles([]string{testRepoKey}, false)
			require.NoError(t, err)
			retryable, err := getErrorsFiles([]string{testRepoKey}, true)
			require.NoError(t, err)
			if ignoreState {
				assert.Empty(t, skipped, "stale skipped rows must be removed")
				assert.Empty(t, retryable)
			} else {
				assert.Len(t, skipped, 1)
				assert.Len(t, retryable, 1)
			}
		})
	}
}

// TestCleanup_warnsAboutSourceGoneSkips covers the end-of-run Warn: the count of items skipped
// because they vanished from the source is reported once and then reset.
func TestCleanup_warnsAboutSourceGoneSkips(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	require.NoError(t, err)
	defer cleanUpJfrogHome()
	takeSourceItemGoneSkipCount()
	buffer, stderrBuffer, previousLog := tests.RedirectLogOutputToBuffer()
	defer log.SetLogger(previousLog)

	cmd, err := NewTransferFilesCommand(nil, nil)
	require.NoError(t, err)
	logSourceItemGone(api.FileRepresentation{Repo: "r", Path: "p", Name: "a"}, "test")
	logSourceItemGone(api.FileRepresentation{Repo: "r", Path: "p", Name: "b"}, "test")
	require.NoError(t, cmd.cleanup(nil, []string{}))
	out := buffer.String() + stderrBuffer.String()
	assert.Contains(t, out, "2 item(s) were skipped because they no longer exist in the source")
	assert.Contains(t, out, string(api.SkippedSourceItemGone))

	buffer.Reset()
	stderrBuffer.Reset()
	require.NoError(t, cmd.cleanup(nil, []string{}))
	assert.NotContains(t, buffer.String()+stderrBuffer.String(), "were skipped because they no longer exist", "the count is reset after being reported")
}

func TestResolveTimestampFilter(t *testing.T) {
	const validTs = "2025-01-01T00:00:00.000Z"

	cases := []struct {
		name            string
		createdAfter    string
		downloadedAfter string
		wantField       timestampFilterField
		wantTimestamp   string
		wantNil         bool
		wantErrContains string
		wantWarn        bool
	}{
		{
			name:    "no filter",
			wantNil: true,
		},
		{
			name:          "created after only",
			createdAfter:  validTs,
			wantField:     createdFilterField,
			wantTimestamp: validTs,
		},
		{
			name:            "downloaded after only",
			downloadedAfter: validTs,
			wantField:       downloadedFilterField,
			wantTimestamp:   validTs,
		},
		{
			name:            "created takes precedence when both set",
			createdAfter:    validTs,
			downloadedAfter: "2024-06-01T12:30:45.123Z",
			wantField:       createdFilterField,
			wantTimestamp:   validTs,
			wantWarn:        true,
		},
		{
			name:            "rejects date only",
			createdAfter:    "2025-01-01",
			wantErrContains: "YYYY-MM-DDTHH:mm:ss.sssZ",
		},
		{
			name:            "rejects missing milliseconds",
			createdAfter:    "2025-01-01T00:00:00Z",
			wantErrContains: "YYYY-MM-DDTHH:mm:ss.sssZ",
		},
		{
			name:            "rejects timezone offset",
			createdAfter:    "2025-01-01T00:00:00.000+00:00",
			wantErrContains: "YYYY-MM-DDTHH:mm:ss.sssZ",
		},
		{
			name:            "rejects invalid date",
			createdAfter:    "2025-13-01T00:00:00.000Z",
			wantErrContains: "YYYY-MM-DDTHH:mm:ss.sssZ",
		},
		{
			name:            "rejects trailing data",
			createdAfter:    "2025-01-01T00:00:00.000Zextra",
			wantErrContains: "YYYY-MM-DDTHH:mm:ss.sssZ",
		},
		{
			name:            "rejects downloaded after with bad format",
			downloadedAfter: "not-a-timestamp",
			wantErrContains: "YYYY-MM-DDTHH:mm:ss.sssZ",
		},
		{
			name:            "rejects future created after",
			createdAfter:    "9999-01-01T00:00:00.000Z",
			wantErrContains: "must not be in the future",
		},
		{
			name:            "rejects future downloaded after",
			downloadedAfter: "9999-01-01T00:00:00.000Z",
			wantErrContains: "must not be in the future",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buffer, stderrBuffer, previousLog := tests.RedirectLogOutputToBuffer()
			defer log.SetLogger(previousLog)

			cmd, err := NewTransferFilesCommand(nil, nil)
			assert.NoError(t, err)
			cmd.SetCreatedAfter(tc.createdAfter)
			cmd.SetDownloadedAfter(tc.downloadedAfter)

			filter, err := cmd.resolveTimestampFilter()
			if tc.wantErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrContains)
				assert.Nil(t, filter)
				return
			}
			assert.NoError(t, err)
			if tc.wantNil {
				assert.Nil(t, filter)
			} else {
				require.NotNil(t, filter)
				assert.Equal(t, tc.wantField, filter.field)
				assert.Equal(t, tc.wantTimestamp, filter.timestamp)
			}
			logOutput := buffer.String() + stderrBuffer.String()
			if tc.wantWarn {
				assert.Contains(t, logOutput, "ignoring --downloaded-after")
			} else {
				assert.NotContains(t, logOutput, "ignoring --downloaded-after")
			}
		})
	}
}

func TestInitNewPhasePropagatesTimestampFilter(t *testing.T) {
	cmd, err := NewTransferFilesCommand(nil, nil)
	assert.NoError(t, err)
	cmd.fileTransfer = NewFileTransfer(nil, nil, FileTransferOptions{})
	cmd.SetCreatedAfter("2025-01-01T00:00:00.000Z")
	filter, err := cmd.resolveTimestampFilter()
	assert.NoError(t, err)
	require.NotNil(t, filter)
	cmd.timestampFilter = filter

	phase := &fullTransferPhase{}
	cmd.initNewPhase(phase, artifactoryUtils.RepositorySummary{}, "repo1", false, 0)
	assert.Equal(t, filter, phase.timestampFilter)
}
