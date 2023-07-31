package state

import (
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/reposnapshot"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
)

func TestGetRepositoryState(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	// Assert repo does not exist before it was created.
	loadAndAssertRepo(t, repo1Key, false)

	// Create new repos.
	createAndAssertRepoState(t, stateManager, repo1Key)
	createAndAssertRepoState(t, stateManager, repo2Key)

	// Add data to current repo.
	stateManager.CurrentRepo.FullTransfer.Started = "start"
	stateManager.CurrentRepo.FullTransfer.Ended = "end"
	assert.NoError(t, stateManager.persistTransferState(false))

	// Create new repo, and assert data on previous repo is loaded correctly.
	createAndAssertRepoState(t, stateManager, repo3Key)
	transferState := loadAndAssertRepo(t, repo2Key, true)
	assert.Equal(t, "start", transferState.CurrentRepo.FullTransfer.Started)
	assert.Equal(t, "end", transferState.CurrentRepo.FullTransfer.Ended)
}

func loadAndAssertRepo(t *testing.T, repoKey string, expectedToExist bool) *TransferState {
	transferState, exists, err := LoadTransferState(repoKey, false)
	assert.NoError(t, err)
	assert.Equal(t, expectedToExist, exists)
	if expectedToExist {
		assert.Equal(t, repoKey, transferState.CurrentRepo.Name)
	}
	return &transferState
}

func createAndAssertRepoState(t *testing.T, stateManager *TransferStateManager, repoKey string) {
	assert.NoError(t, stateManager.SetRepoState(repoKey, 0, 0, false, false))
	transferState := loadAndAssertRepo(t, repoKey, true)
	assert.Empty(t, transferState.CurrentRepo.FullTransfer)
	assert.Empty(t, transferState.CurrentRepo.Diffs)
}

func TestSaveAndLoadState(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()
	stateManager.CurrentRepo = newRepositoryTransferState(repo4Key).CurrentRepo

	assert.NoError(t, stateManager.persistTransferState(false))
	actualState, exists, err := LoadTransferState(repo4Key, false)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, stateManager.TransferState, actualState)

	// Persist the transfer state with a high version and assert loading yields an error.
	stateManager.TransferState.Version = 1000
	assert.NoError(t, stateManager.persistTransferState(false))
	_, _, err = LoadTransferState(repo4Key, false)
	assert.ErrorContains(t, err, transferStateFileInvalidVersionErrorMsg)
}

func TestGetTransferStateAndSnapshotClean(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	// Make the snapshot not auto save.
	snapshotCleanUp := setAutoSaveSnapshot(1000)
	defer snapshotCleanUp()

	// Assert getting state before it was created returns clean state.
	cleanTransferState, cleanRepoTransferSnapshot, err := getCleanStateAndSnapshot(repo1Key)
	assert.NoError(t, err)
	assertGetTransferStateAndSnapshot(t, false, cleanTransferState, cleanRepoTransferSnapshot, false)

	// Create repo-state.
	createAndAssertRepoState(t, stateManager, repo1Key)
	// Assert getting clean state on reset.
	assertGetTransferStateAndSnapshot(t, true, cleanTransferState, cleanRepoTransferSnapshot, false)

	// Mark phase 1 as started.
	assert.NoError(t, stateManager.SetRepoFullTransferStarted(time.Now()))
	getRootAndAddSnapshotData(t, stateManager)
	// Create only the snapshot file in snapshot dir (without repo state). Since the state file is missing in the snapshots dir the load should start clean.
	assert.NoError(t, stateManager.repoTransferSnapshot.snapshotManager.PersistRepoSnapshot())
	snapshotPath, err := GetRepoSnapshotFilePath(repo1Key)
	assert.NoError(t, err)
	exists, err := fileutils.IsFileExists(snapshotPath, false)
	assert.NoError(t, err)
	assert.True(t, exists)
	assertGetTransferStateAndSnapshot(t, false, cleanTransferState, cleanRepoTransferSnapshot, false)
}

// Set the snapshot's save-interval and return a cleanup function.
func setAutoSaveSnapshot(interval int) (cleanUp func()) {
	previousSaveInterval := snapshotSaveIntervalMin
	snapshotSaveIntervalMin = interval
	return func() {
		snapshotSaveIntervalMin = previousSaveInterval
	}
}

// Testing two scenarios here:
// 1. loading (snapshot file and state) from existing snapshot
// 2. loading state of fully transferred repository (phase 1 completed)
func TestGetTransferStateAndSnapshotLoading(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	// Create repo-state.
	createAndAssertRepoState(t, stateManager, repo1Key)

	// Add content to state and snapshot.
	assert.NoError(t, stateManager.SetRepoFullTransferStarted(time.Now()))
	assert.NoError(t, stateManager.IncTransferredSizeAndFilesPhase1(2, 3))
	_ = getRootAndAddSnapshotData(t, stateManager)
	// Get state before saving.
	originalState := stateManager.TransferState
	assert.NoError(t, stateManager.SaveStateAndSnapshots())
	// Modify state again, and assert that the loaded state from snapshot was not modified and remained as saved.
	assert.NoError(t, stateManager.IncTransferredSizeAndFilesPhase1(2, 3))
	assert.NotEqual(t, stateManager.TransferState.CurrentRepo, originalState.CurrentRepo)
	assertGetTransferStateAndSnapshot(t, false, originalState, stateManager.repoTransferSnapshot, true)

	// After repo fully transferred, expected to load state without snapshots.
	assert.NoError(t, stateManager.SetRepoFullTransferCompleted())
	assertGetTransferStateAndSnapshot(t, false, stateManager.TransferState, nil, false)
}

func assertGetTransferStateAndSnapshot(t *testing.T, reset bool, expectedTransferState TransferState,
	expectedRepoTransferSnapshot *RepoTransferSnapshot, expectedSnapshotLoaded bool) {
	transferState, repoTransferSnapshot, err := getTransferStateAndSnapshot(repo1Key, reset)
	assert.NoError(t, err)
	assert.Equal(t, expectedTransferState.Version, transferState.Version)
	assert.Equal(t, expectedTransferState.CurrentRepo, transferState.CurrentRepo)

	// If one or both snapshots are nil, don't assert by field.
	if expectedRepoTransferSnapshot == nil || repoTransferSnapshot == nil {
		assert.Equal(t, expectedRepoTransferSnapshot, repoTransferSnapshot)
		return
	}
	assert.Equal(t, expectedSnapshotLoaded, repoTransferSnapshot.loadedFromSnapshot)
}

func getRootAndAddSnapshotData(t *testing.T, stateManager *TransferStateManager) (root *reposnapshot.Node) {
	root, err := stateManager.LookUpNode(".")
	assert.NoError(t, err)
	assert.NoError(t, root.IncrementFilesCount())
	assert.NoError(t, root.AddChildNode("child", nil))
	return root
}
