package state

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetRepositoryState(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	// Assert repo does not exist before it was created.
	loadAndAssertRepo(t, repo1Key, false)

	// Create new repos.
	createAndAssertRepo(t, stateManager, repo1Key)
	createAndAssertRepo(t, stateManager, repo2Key)

	// Add data to current repo.
	stateManager.CurrentRepo.FullTransfer.Started = "start"
	stateManager.CurrentRepo.FullTransfer.Ended = "end"
	assert.NoError(t, stateManager.persistTransferState())

	// Create new repo, and assert data on previous repo is loaded correctly.
	createAndAssertRepo(t, stateManager, repo3Key)
	transferState := loadAndAssertRepo(t, repo2Key, true)
	assert.Equal(t, "start", transferState.CurrentRepo.FullTransfer.Started)
	assert.Equal(t, "end", transferState.CurrentRepo.FullTransfer.Ended)
}

func loadAndAssertRepo(t *testing.T, repoKey string, expectedToExist bool) *TransferState {
	transferState, exists, err := LoadTransferState(repoKey)
	assert.NoError(t, err)
	assert.Equal(t, expectedToExist, exists)
	if expectedToExist {
		assert.Equal(t, repoKey, transferState.CurrentRepo.Name)
	}
	return &transferState
}

func createAndAssertRepo(t *testing.T, stateManager *TransferStateManager, repoKey string) {
	assert.NoError(t, stateManager.SetRepoState(repoKey, 0, 0, false, false))
	transferState := loadAndAssertRepo(t, repoKey, true)
	assert.Empty(t, transferState.CurrentRepo.FullTransfer)
	assert.Empty(t, transferState.CurrentRepo.Diffs)
}

func TestSaveAndLoadState(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()
	stateManager.CurrentRepo = newRepositoryTransferState(repo4Key).CurrentRepo

	assert.NoError(t, stateManager.persistTransferState())
	actualState, exists, err := LoadTransferState(repo4Key)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, stateManager.TransferState, actualState)

	// Persist the transfer state with a high version and assert loading yields an error.
	stateManager.TransferState.Version = 1000
	assert.NoError(t, stateManager.persistTransferState())
	_, _, err = LoadTransferState(repo4Key)
	assert.ErrorContains(t, err, transferStateFileInvalidVersionErrorMsg)
}
