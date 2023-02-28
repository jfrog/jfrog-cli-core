package commands

import (
	"bytes"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/pipelines/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func initDisplaySyncStatusTest(t *testing.T) (*bytes.Buffer, func()) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)

	// Redirect log to buffer
	buffer, _, previousLog := tests.RedirectLogOutputToBuffer()

	undoSaveInterval := state.SetAutoSaveState()
	return buffer, func() {
		undoSaveInterval()
		log.SetLogger(previousLog)
		cleanUpJfrogHome()
	}
}

func TestSyncStatusCommand_displaySyncStatus(t *testing.T) {
	buffer, cleanup := initDisplaySyncStatusTest(t)
	defer cleanup()

	// Create pipeline sync status response
	pipelineSyncStatuses := createPipelinesSyncStatus()

	t.Run("Should print these expected details to standard output", func(t *testing.T) {
		sc := &SyncStatusCommand{
			serverDetails: &config.ServerDetails{},
			branch:        "master",
			repoPath:      "jfrog/jfrog-cli-core",
		}
		sc.displaySyncStatus(pipelineSyncStatuses)
		results := buffer.String()
		assert.Contains(t, results, "Committer: testUser")
		assert.Contains(t, results, "CommitSHA: 83749i34urbjbrjkrwoeurheiwrhtt35")
		assert.Contains(t, results, "CommitMessage: Added test cases")
		assert.Contains(t, results, "SyncSummary: Sync is in progress")
		assert.Contains(t, results, "IsSyncing: true")
		assert.Contains(t, results, "Status: success")
	})
}

func createPipelinesSyncStatus() []services.PipelineSyncStatus {
	isSyncing := true
	commitDetails := services.CommitData{
		CommitSha: "83749i34urbjbrjkrwoeurheiwrhtt35",
		Committer: "testUser",
		CommitMsg: "Added test cases",
	}
	pipelineSyncStatus := services.PipelineSyncStatus{
		IsSyncing:          &isSyncing,
		LastSyncStartedAt:  time.Now(),
		LastSyncEndedAt:    time.Now().Add(2 * time.Minute),
		CommitData:         commitDetails,
		LastSyncLogs:       "Sync is in progress",
		LastSyncStatusCode: 4002,
	}
	return []services.PipelineSyncStatus{pipelineSyncStatus}
}
