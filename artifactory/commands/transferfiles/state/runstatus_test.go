package state

import (
	"os"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
)

func TestSaveAndLoadRunStatus(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()
	stateManager.CurrentRepo = newRepositoryTransferState(repo4Key).CurrentRepo
	stateManager.CurrentRepoPhase = 2

	assert.NoError(t, stateManager.persistTransferRunStatus())
	actualStatus, exists, err := loadTransferRunStatus()
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, transferRunStatusVersion, actualStatus.Version)
	actualStatus.stateManager = stateManager
	assert.Equal(t, stateManager.TransferRunStatus, actualStatus)
}

// A run-status.json written by a pre-GET/PUT CLI version (plugin-based transfer) still carries a
// populated "stale_chunks" field. GET/PUT transfer no longer produces or displays this data, but the
// field is kept on TransferRunStatus for schema compatibility, so loading such a file must still
// succeed and round-trip the legacy value rather than erroring out or silently dropping it.
func TestLoadRunStatus_legacyStaleChunksFieldParses(t *testing.T) {
	_, cleanUp := InitStateTest(t)
	defer cleanUp()

	legacyRunStatus := `{
		"version": 1,
		"current_repo": "repo1",
		"stale_chunks": [
			{
				"node_id": "node-1",
				"stale_node_chunks": [
					{"chunk_id": "chunk-1", "files": ["a.txt", "b.txt"], "sent": 1690000000}
				]
			}
		]
	}`
	statusFilePath, err := coreutils.GetJfrogTransferRunStatusFilePath()
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(statusFilePath, []byte(legacyRunStatus), 0600))

	actualStatus, exists, err := loadTransferRunStatus()
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, transferRunStatusVersion, actualStatus.Version)
	assert.Equal(t, "repo1", actualStatus.CurrentRepoKey)
	assert.Equal(t, []StaleChunks{
		{
			NodeID: "node-1",
			Chunks: []StaleChunk{
				{ChunkID: "chunk-1", Files: []string{"a.txt", "b.txt"}, Sent: 1690000000},
			},
		},
	}, actualStatus.StaleChunks)
}
