package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRepositoryState(t *testing.T) {
	state := &TransferState{}

	// Get repo on empty state, without creating if missing.
	getAndAssertNonExistingRepo(t, state, repo1Key)

	// Create new repo.
	_ = createAndAssertRepo(t, state, repo1Key)
	_ = createAndAssertRepo(t, state, repo2Key)
	_ = createAndAssertRepo(t, state, repo3Key)

	// Get a non-existing repo from a non-empty state, without creating if missing.
	getAndAssertNonExistingRepo(t, state, repo4Key)

	// Add data to existing repo, get and assert.
	repo2 := createAndAssertRepo(t, state, repo2Key)
	repo2.FullTransfer.Started = "start"
	repo2.FullTransfer.Ended = "end"
	repo2, err := state.getRepository(repo2Key, false)
	assert.NoError(t, err)
	assert.Equal(t, repo2Key, repo2.Name)
	assert.Equal(t, "start", repo2.FullTransfer.Started)
	assert.Equal(t, "end", repo2.FullTransfer.Ended)
}

func createAndAssertRepo(t *testing.T, state *TransferState, repoKey string) *Repository {
	repo, err := state.getRepository(repoKey, true)
	assert.NoError(t, err)
	assert.Equal(t, repoKey, repo.Name)
	assert.Empty(t, repo.FullTransfer)
	assert.Empty(t, repo.Diffs)
	return repo
}

func getAndAssertNonExistingRepo(t *testing.T, state *TransferState, repoKey string) {
	repo, err := state.getRepository(repoKey, false)
	assert.EqualError(t, err, getRepoMissingErrorMsg(repoKey))
	assert.Nil(t, repo)
}

func TestSaveAndLoadState(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()
	stateManager.Repositories = []Repository{{Name: repo4Key}}

	assert.NoError(t, stateManager.persistTransferState())
	actualState, err := loadTransferState()
	assert.NoError(t, err)
	assert.Equal(t, stateManager.TransferState, *actualState)
}
