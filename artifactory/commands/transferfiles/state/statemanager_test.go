package state

import (
	"testing"
	"time"

	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/stretchr/testify/assert"
)

const (
	repo1Key = "repo1"
	repo2Key = "repo2"
	repo3Key = "repo3"
	repo4Key = "repo4"
)

func TestFilesDiffRange(t *testing.T) {
	stateManager, cleanUp := initStateTest(t)
	defer cleanUp()

	repoKey := "repo"
	transferStartTime := time.Now()
	// Repo should be marked as not transferred. This also adds repo to state.
	assertRepoTransferred(t, stateManager, repoKey, false)
	setAndAssertRepoFullyTransfer(t, stateManager, repoKey, transferStartTime)

	// Set diff start and assert handling range begins in transfer start time and ends in new diff start time.
	_ = addAndAssertNewDiffPhase(t, stateManager, repoKey, 1, transferStartTime)
	// Set diff start again, as if previous was interrupted. Handling range start should be the same. Handling range end should be new diff start time.
	diffStart := addAndAssertNewDiffPhase(t, stateManager, repoKey, 2, transferStartTime)
	// Set diff completed.
	setAndAssertFilesDiffCompleted(t, stateManager, repoKey, 2)
	// Next diff handling range should begin on last completed diff start time.
	_ = addAndAssertNewDiffPhase(t, stateManager, repoKey, 3, diffStart)
}

func assertRepoTransferred(t *testing.T, stateManager *TransferStateManager, repoKey string, expected bool) {
	transferred, err := stateManager.IsRepoTransferred(repoKey)
	assert.NoError(t, err)
	assert.Equal(t, expected, transferred)
}

func setAndAssertRepoFullyTransfer(t *testing.T, stateManager *TransferStateManager, repoKey string, startTime time.Time) {
	err := stateManager.SetRepoFullTransferStarted(repoKey, startTime)
	assert.NoError(t, err)
	assertRepoTransferred(t, stateManager, repoKey, false)

	time.Sleep(time.Second)
	err = stateManager.SetRepoFullTransferCompleted(repoKey)
	assert.NoError(t, err)
	assertRepoTransferred(t, stateManager, repoKey, true)

	repo := getRepoFromState(t, stateManager, repoKey)
	assert.Equal(t, ConvertTimeToRFC3339(startTime), repo.FullTransfer.Started)
	assert.NotEmpty(t, repo.FullTransfer.Ended)
	assert.NotEqual(t, repo.FullTransfer.Ended, repo.FullTransfer.Started)
}

func addAndAssertNewDiffPhase(t *testing.T, stateManager *TransferStateManager, repoKey string, expectedDiffs int, handlingExpectedTime time.Time) (diffStart time.Time) {
	diffStart = time.Now()
	err := stateManager.AddNewDiffToState(repoKey, diffStart)
	assert.NoError(t, err)
	repo := getRepoFromState(t, stateManager, repoKey)
	assert.Equal(t, expectedDiffs, len(repo.Diffs))
	assert.Equal(t, ConvertTimeToRFC3339(diffStart), repo.Diffs[expectedDiffs-1].FilesDiffRunTime.Started)

	handlingStart, handlingEnd, err := stateManager.GetDiffHandlingRange(repoKey)
	assert.NoError(t, err)
	// Truncating the expected time because milliseconds are lost in conversions.
	assert.True(t, handlingExpectedTime.Truncate(time.Second).Equal(handlingStart))
	assert.True(t, diffStart.Truncate(time.Second).Equal(handlingEnd))
	return
}

func setAndAssertFilesDiffCompleted(t *testing.T, stateManager *TransferStateManager, repoKey string, diffNum int) {
	err := stateManager.SetFilesDiffHandlingCompleted(repoKey)
	assert.NoError(t, err)
	repo := getRepoFromState(t, stateManager, repoKey)
	assert.True(t, repo.Diffs[diffNum-1].Completed)
}

func getRepoFromState(t *testing.T, stateManager *TransferStateManager, repoKey string) *Repository {
	repo, err := stateManager.getRepository(repoKey, false)
	assert.NoError(t, err)
	return repo
}

func initStateTest(t *testing.T) (stateManager *TransferStateManager, cleanUp func()) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	cleanUp = cleanUpJfrogHome

	// Create transfer directory
	transferDir, err := coreutils.GetJfrogTransferDir()
	assert.NoError(t, err)
	err = utils.CreateDirIfNotExist(transferDir)
	assert.NoError(t, err)

	stateManager, err = NewTransferStateManager(true)
	assert.NoError(t, err)
	return
}

func TestResetRepoState(t *testing.T) {
	stateManager, cleanUp := initStateTest(t)
	defer cleanUp()

	// Reset a repository state on an empty state
	err := stateManager.SetRepoState(repo1Key, 0, 0, true)
	assert.NoError(t, err)
	// Set repository fully transferred. It will fail the test if the repository is not in the state.
	setAndAssertRepoFullyTransfer(t, stateManager, repo1Key, time.Now())

	// Create another repository state
	err = stateManager.SetRepoState(repo2Key, 0, 0, true)
	assert.NoError(t, err)
	setAndAssertRepoFullyTransfer(t, stateManager, repo2Key, time.Now())

	// Reset repo1 only
	err = stateManager.SetRepoState(repo1Key, 0, 0, true)
	assert.NoError(t, err)
	assertRepoTransferred(t, stateManager, repo1Key, false)
}

func TestReposTransferredSizeBytes(t *testing.T) {
	stateManager, cleanUp := initStateTest(t)
	defer cleanUp()

	// Create repos in state.
	assert.NoError(t, stateManager.SetRepoState(repo1Key, 0, 0, true))
	assert.NoError(t, stateManager.SetRepoState(repo2Key, 0, 0, true))

	// Inc repos transferred sizes.
	assert.NoError(t, stateManager.IncTransferredSizeAndFiles(repo1Key, 1, 10))
	assert.NoError(t, stateManager.IncTransferredSizeAndFiles(repo1Key, 5, 11))
	assert.NoError(t, stateManager.IncTransferredSizeAndFiles(repo2Key, 3, 200))
	err := stateManager.IncTransferredSizeAndFiles(repo3Key, 4, 3000)
	assert.EqualError(t, err, getRepoMissingErrorMsg(repo3Key))

	// Get repos transferred sizes, one at a time.
	assertTransferredSize(t, stateManager, 21, repo1Key)
	assertTransferredSize(t, stateManager, 200, repo2Key)
	assertTransferredSize(t, stateManager, 0, repo3Key)

	// Get a combination of all repos. Pass repo2 twice to verify its size is not duplicated.
	assertTransferredSize(t, stateManager, 221, repo1Key, repo2Key, repo2Key, repo3Key)

	// No repos.
	assertTransferredSize(t, stateManager, 0)

	// Assert the sum bytes of repo1 + repo2 in the run-status.
	transferredSizeBytes, err := stateManager.GetTransferredSizeBytes()
	assert.NoError(t, err)
	assert.Equal(t, int64(221), transferredSizeBytes)

	// Assert the number of transferred files in the state.
	assertTransferredFiles(t, stateManager, 6, repo1Key)
	assertTransferredFiles(t, stateManager, 3, repo2Key)
}

func assertTransferredSize(t *testing.T, stateManager *TransferStateManager, expectedSize int64, repoKeys ...string) {
	totalTransferredSize, err := stateManager.GetReposTransferredSizeBytes(repoKeys...)
	assert.NoError(t, err)
	assert.Equal(t, expectedSize, totalTransferredSize)
}

func assertTransferredFiles(t *testing.T, stateManager *TransferStateManager, expectedFiles int, repoKey string) {
	repo, err := stateManager.getRepository(repoKey, false)
	assert.NoError(t, err)
	assert.Equal(t, expectedFiles, repo.TransferredUnits)
}

func TestIncRepositoriesTransferred(t *testing.T) {
	stateManager, cleanUp := initStateTest(t)
	defer cleanUp()

	assert.Zero(t, stateManager.TransferredUnits)
	assert.NoError(t, stateManager.IncRepositoriesTransferred())
	assert.Equal(t, 1, stateManager.TransferredUnits)
}

func TestSetRepoPhase(t *testing.T) {
	stateManager, cleanUp := initStateTest(t)
	defer cleanUp()

	assert.Zero(t, stateManager.CurrentRepoPhase)
	assert.NoError(t, stateManager.SetRepoPhase(1))
	assert.Equal(t, 1, stateManager.CurrentRepoPhase)
}

func TestSetAndGetWorkingThreads(t *testing.T) {
	stateManager, cleanUp := initStateTest(t)
	defer cleanUp()

	assert.Zero(t, stateManager.WorkingThreads)
	assert.NoError(t, stateManager.SetWorkingThreads(1))
	assert.Equal(t, 1, stateManager.WorkingThreads)
	workingThreads, err := stateManager.GetWorkingThreads()
	assert.NoError(t, err)
	assert.Equal(t, 1, workingThreads)
}

func TestTryLockStateManager(t *testing.T) {
	stateManager, cleanUp := initStateTest(t)
	defer cleanUp()

	assert.NoError(t, stateManager.tryLockStateManager())
	assert.ErrorIs(t, new(AlreadyLockedError), stateManager.tryLockStateManager())
}
