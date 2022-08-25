package transferfiles

import (
	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestGetRepositoryState(t *testing.T) {
	state := &TransferState{}
	repo1Key, repo2Key, repo3Key := "repo1", "repo2", "repo3"

	// Get repo on empty state, without creating if missing.
	getAndAssertNonExistingRepo(t, state, repo1Key)

	// Create new repo.
	_ = createAndAssertRepo(t, state, repo1Key)
	_ = createAndAssertRepo(t, state, repo2Key)
	_ = createAndAssertRepo(t, state, repo3Key)

	// Get a non-existing repo from a non-empty state, without creating if missing.
	getAndAssertNonExistingRepo(t, state, "repo4")

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

func TestFilesDiffRange(t *testing.T) {
	cleanUp := initStateTest(t)
	defer cleanUp()

	repoKey := "repo"
	transferStartTime := time.Now()
	// Repo should be marked as not transferred. This also adds repo to state.
	assertRepoTransferred(t, repoKey, false)
	setAndAssertRepoFullyTransfer(t, repoKey, transferStartTime)

	// Set diff start and assert handling range begins in transfer start time and ends in new diff start time.
	_ = addAndAssertNewDiffPhase(t, repoKey, 1, transferStartTime)
	// Set diff start again, as if previous was interrupted. Handling range start should be the same. Handling range end should be new diff start time.
	diffStart := addAndAssertNewDiffPhase(t, repoKey, 2, transferStartTime)
	// Set diff completed.
	setAndAssertFilesDiffCompleted(t, repoKey, 2)
	// Next diff handling range should begin on last completed diff start time.
	_ = addAndAssertNewDiffPhase(t, repoKey, 3, diffStart)
}

func assertRepoTransferred(t *testing.T, repoKey string, expected bool) {
	transferred, err := isRepoTransferred(repoKey)
	assert.NoError(t, err)
	assert.Equal(t, expected, transferred)
}

func setAndAssertRepoFullyTransfer(t *testing.T, repoKey string, startTime time.Time) {
	err := setRepoFullTransferStarted(repoKey, startTime)
	assert.NoError(t, err)
	assertRepoTransferred(t, repoKey, false)

	time.Sleep(time.Second)
	err = setRepoFullTransferCompleted(repoKey)
	assert.NoError(t, err)
	assertRepoTransferred(t, repoKey, true)

	repo := getRepoFromState(t, repoKey)
	assert.Equal(t, convertTimeToRFC3339(startTime), repo.FullTransfer.Started)
	assert.NotEmpty(t, repo.FullTransfer.Ended)
	assert.NotEqual(t, repo.FullTransfer.Ended, repo.FullTransfer.Started)
}

func addAndAssertNewDiffPhase(t *testing.T, repoKey string, expectedDiffs int, handlingExpectedTime time.Time) (diffStart time.Time) {
	diffStart = time.Now()
	err := addNewDiffToState(repoKey, diffStart)
	assert.NoError(t, err)
	repo := getRepoFromState(t, repoKey)
	assert.Equal(t, expectedDiffs, len(repo.Diffs))
	assert.Equal(t, convertTimeToRFC3339(diffStart), repo.Diffs[expectedDiffs-1].FilesDiffRunTime.Started)

	handlingStart, handlingEnd, err := getDiffHandlingRange(repoKey)
	assert.NoError(t, err)
	// Truncating the expected time because milliseconds are lost in conversions.
	assert.True(t, handlingExpectedTime.Truncate(time.Second).Equal(handlingStart))
	assert.True(t, diffStart.Truncate(time.Second).Equal(handlingEnd))
	return
}

func setAndAssertFilesDiffCompleted(t *testing.T, repoKey string, diffNum int) {
	err := setFilesDiffHandlingCompleted(repoKey)
	assert.NoError(t, err)
	repo := getRepoFromState(t, repoKey)
	assert.True(t, repo.Diffs[diffNum-1].Completed)
}

func getRepoFromState(t *testing.T, repoKey string) *Repository {
	state, err := getTransferState()
	assert.NoError(t, err)
	repo, err := state.getRepository(repoKey, false)
	assert.NoError(t, err)
	return repo
}

func initStateTest(t *testing.T) (cleanUp func()) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	cleanUp = cleanUpJfrogHome

	// Create transfer directory
	transferDir, err := coreutils.GetJfrogTransferDir()
	assert.NoError(t, err)
	err = utils.CreateDirIfNotExist(transferDir)
	assert.NoError(t, err)
	return
}

func TestResetRepoState(t *testing.T) {
	cleanUp := initStateTest(t)
	defer cleanUp()
	repo1Key, repo2Key := "repo1", "repo2"

	// Reset a repository state on an empty state
	err := resetRepoState(repo1Key)
	assert.NoError(t, err)
	// Set repository fully transferred. It will fail the test if the repository is not in the state.
	setAndAssertRepoFullyTransfer(t, repo1Key, time.Now())

	// Create another repository state
	err = resetRepoState(repo2Key)
	assert.NoError(t, err)
	setAndAssertRepoFullyTransfer(t, repo2Key, time.Now())

	// Reset repo1 only
	err = resetRepoState(repo1Key)
	assert.NoError(t, err)
	assertRepoTransferred(t, repo1Key, false)
}
