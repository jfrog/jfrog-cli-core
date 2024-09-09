package state

import (
	"github.com/jfrog/gofrog/safeconvert"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	repo1Key = "repo1"
	repo2Key = "repo2"
	repo3Key = "repo3"
	repo4Key = "repo4"
)

func TestFilesDiffRange(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	transferStartTime := time.Now()
	// Repo should be marked as not transferred. This also adds repo to state.
	assertRepoTransferred(t, stateManager, false)
	setAndAssertRepoFullyTransfer(t, stateManager, transferStartTime)

	// Set diff start and assert handling range begins in transfer start time and ends in new diff start time.
	_ = addAndAssertNewDiffPhase(t, stateManager, 1, transferStartTime)
	// Set diff start again, as if previous was interrupted. Handling range start should be the same. Handling range end should be new diff start time.
	diffStart := addAndAssertNewDiffPhase(t, stateManager, 2, transferStartTime)
	// Set diff completed.
	setAndAssertFilesDiffCompleted(t, stateManager, 2)
	// Next diff handling range should begin on last completed diff start time.
	_ = addAndAssertNewDiffPhase(t, stateManager, 3, diffStart)
}

func assertRepoTransferred(t *testing.T, stateManager *TransferStateManager, expected bool) {
	transferred, err := stateManager.IsRepoTransferred()
	assert.NoError(t, err)
	assert.Equal(t, expected, transferred)
}

func setAndAssertRepoFullyTransfer(t *testing.T, stateManager *TransferStateManager, startTime time.Time) {
	err := stateManager.SetRepoFullTransferStarted(startTime)
	assert.NoError(t, err)
	assertRepoTransferred(t, stateManager, false)

	time.Sleep(time.Second)
	err = stateManager.SetRepoFullTransferCompleted()
	assert.NoError(t, err)
	assertRepoTransferred(t, stateManager, true)

	repo := stateManager.CurrentRepo
	assert.Equal(t, ConvertTimeToRFC3339(startTime), repo.FullTransfer.Started)
	assert.NotEmpty(t, repo.FullTransfer.Ended)
	assert.NotEqual(t, repo.FullTransfer.Ended, repo.FullTransfer.Started)
}

func addAndAssertNewDiffPhase(t *testing.T, stateManager *TransferStateManager, expectedDiffs int, handlingExpectedTime time.Time) (diffStart time.Time) {
	diffStart = time.Now()
	err := stateManager.AddNewDiffToState(diffStart)
	assert.NoError(t, err)
	repo := stateManager.CurrentRepo
	assert.Equal(t, expectedDiffs, len(repo.Diffs))
	assert.Equal(t, ConvertTimeToRFC3339(diffStart), repo.Diffs[expectedDiffs-1].FilesDiffRunTime.Started)

	handlingStart, handlingEnd, err := stateManager.GetDiffHandlingRange()
	assert.NoError(t, err)
	// Truncating the expected time because milliseconds are lost in conversions.
	assert.True(t, handlingExpectedTime.Truncate(time.Second).Equal(handlingStart))
	assert.True(t, diffStart.Truncate(time.Second).Equal(handlingEnd))
	return
}

func setAndAssertFilesDiffCompleted(t *testing.T, stateManager *TransferStateManager, diffNum int) {
	err := stateManager.SetFilesDiffHandlingCompleted()
	assert.NoError(t, err)
	assert.True(t, stateManager.CurrentRepo.Diffs[diffNum-1].Completed)
}

func TestResetRepoState(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	// Reset a repository state on an empty state
	err := stateManager.SetRepoState(repo1Key, 0, 0, false, true)
	assert.NoError(t, err)
	// Set repository fully transferred. It will fail the test if the repository is not in the state.
	setAndAssertRepoFullyTransfer(t, stateManager, time.Now())

	// Create another repository state
	err = stateManager.SetRepoState(repo2Key, 0, 0, false, true)
	assert.NoError(t, err)
	setAndAssertRepoFullyTransfer(t, stateManager, time.Now())

	// Reset repo1 only
	err = stateManager.SetRepoState(repo1Key, 0, 0, false, true)
	assert.NoError(t, err)
	assertRepoTransferred(t, stateManager, false)
}

func TestReposTransferredSizeBytes(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	// Inc repos transferred sizes.
	assert.NoError(t, stateManager.SetRepoState(repo1Key, 0, 0, false, true))
	assert.NoError(t, stateManager.IncTransferredSizeAndFilesPhase1(1, 10))
	assert.NoError(t, stateManager.IncTransferredSizeAndFilesPhase1(5, 11))
	assertCurrentRepoTransferredFiles(t, stateManager, 6)
	assert.NoError(t, stateManager.SetRepoState(repo2Key, 0, 0, false, true))
	assert.NoError(t, stateManager.IncTransferredSizeAndFilesPhase1(3, 200))
	assertCurrentRepoTransferredFiles(t, stateManager, 3)

	// Get repos transferred sizes, one at a time.
	assertReposTransferredSize(t, stateManager, 21, repo1Key)
	assertReposTransferredSize(t, stateManager, 200, repo2Key)
	assertReposTransferredSize(t, stateManager, 0, repo3Key)

	// Get a combination of all repos. Pass repo2 twice to verify its size is not duplicated.
	assertReposTransferredSize(t, stateManager, 221, repo1Key, repo2Key, repo2Key, repo3Key)

	// No repos.
	assertReposTransferredSize(t, stateManager, 0)

	// Assert the sum bytes of repo1 + repo2 in the run-status.
	transferredSizeBytes, err := stateManager.GetTransferredSizeBytes()
	assert.NoError(t, err)
	assert.Equal(t, int64(221), transferredSizeBytes)
}

func TestReposOverallBiFiles(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	// Inc repos transferred sizes and files.
	assert.NoError(t, stateManager.SetRepoState(repo1Key, 0, 0, true, true))
	assert.NoError(t, stateManager.IncTransferredSizeAndFilesPhase1(2, 9))
	assert.NoError(t, stateManager.SetRepoState(repo2Key, 0, 0, true, true))
	assert.NoError(t, stateManager.IncTransferredSizeAndFilesPhase1(1, 10))
	assert.NoError(t, stateManager.IncTransferredSizeAndFilesPhase1(5, 11))

	// Assert the number of transferred bi files in the state.
	assert.Equal(t, repo2Key, stateManager.CurrentRepo.Name)
	assert.Equal(t, repo2Key, stateManager.CurrentRepoKey)
	assert.True(t, stateManager.BuildInfoRepo)
	assert.Equal(t, int64(8), stateManager.OverallBiFiles.TransferredUnits)
}

func TestChangeDelayedFilesCountBy(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	// Increase delayed files count
	assert.NoError(t, stateManager.ChangeDelayedFilesCountBy(2, true))
	assert.NoError(t, stateManager.ChangeDelayedFilesCountBy(4, true))
	assert.Equal(t, uint64(6), stateManager.DelayedFiles)

	// Decrease delayed files count
	assert.NoError(t, stateManager.ChangeDelayedFilesCountBy(3, false))
	assert.Equal(t, uint64(3), stateManager.DelayedFiles)
}

func TestVisitedFolders(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	// Increase visited folders count
	assert.NoError(t, stateManager.IncVisitedFolders())
	assert.NoError(t, stateManager.IncVisitedFolders())
	assert.Equal(t, uint64(2), stateManager.VisitedFolders)

	// Set repository state and ensure the visited folders became 0
	assert.NoError(t, stateManager.SetRepoState(repo1Key, 0, 0, true, true))
	assert.Zero(t, stateManager.VisitedFolders)
}

func TestChangeTransferFailureCountBy(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	// Increase failures count
	assert.NoError(t, stateManager.ChangeTransferFailureCountBy(2, true))
	assert.NoError(t, stateManager.ChangeTransferFailureCountBy(4, true))
	assert.Equal(t, uint64(6), stateManager.TransferFailures)

	// Decrease failures count
	assert.NoError(t, stateManager.ChangeTransferFailureCountBy(3, false))
	assert.Equal(t, uint64(3), stateManager.TransferFailures)
}

func assertReposTransferredSize(t *testing.T, stateManager *TransferStateManager, expectedSize int64, repoKeys ...string) {
	totalTransferredSize, err := stateManager.GetReposTransferredSizeBytes(repoKeys...)
	assert.NoError(t, err)
	assert.Equal(t, expectedSize, totalTransferredSize)
}

func assertCurrentRepoTransferredFiles(t *testing.T, stateManager *TransferStateManager, expectedFiles int64) {
	assert.Equal(t, expectedFiles, stateManager.CurrentRepo.Phase1Info.TransferredUnits)
}

func TestIncRepositoriesTransferred(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	assert.Zero(t, stateManager.TotalRepositories.TransferredUnits)
	assert.NoError(t, stateManager.IncRepositoriesTransferred())
	assert.Equal(t, int64(1), stateManager.TotalRepositories.TransferredUnits)
}

func TestSetRepoPhase(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	assert.Zero(t, stateManager.CurrentRepoPhase)
	assert.NoError(t, stateManager.SetRepoPhase(1))
	assert.Equal(t, 1, stateManager.CurrentRepoPhase)
}

func TestSetAndGetWorkingThreads(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	assert.Zero(t, stateManager.WorkingThreads)
	assert.NoError(t, stateManager.SetWorkingThreads(1))
	assert.Equal(t, 1, stateManager.WorkingThreads)
	workingThreads, err := stateManager.GetWorkingThreads()
	assert.NoError(t, err)
	assert.Equal(t, 1, workingThreads)
}

func TestTryLockStateManager(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	assert.NoError(t, stateManager.tryLockStateManager())
	assert.ErrorIs(t, new(AlreadyLockedError), stateManager.tryLockStateManager())
}

func TestRunning(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	// Assert no running=false
	running, err := stateManager.Running()
	assert.NoError(t, err)
	assert.False(t, running)

	// Lock to simulate transfer
	assert.NoError(t, stateManager.TryLockTransferStateManager())

	// Assert running=true
	running, err = stateManager.Running()
	assert.NoError(t, err)
	assert.True(t, running)
}

func TestInitStartTimestamp(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	// Init start timestamp and expect timestamp zero
	running, err := stateManager.InitStartTimestamp()
	assert.NoError(t, err)
	assert.False(t, running)
	assert.True(t, stateManager.startTimestamp.IsZero())

	// Lock to simulate transfer
	assert.NoError(t, stateManager.TryLockTransferStateManager())

	// Init start timestamp and expect timestamp non-zero
	running, err = stateManager.InitStartTimestamp()
	assert.NoError(t, err)
	assert.True(t, running)
	assert.False(t, stateManager.startTimestamp.IsZero())
}

var getRunningTimeStringCases = []struct {
	startTimestamp time.Time
	expectedString string
}{
	{time.Now(), "Less than a minute"},
	{time.Now().Add(-time.Second), "Less than a minute"},
	{time.Now().Add(-time.Minute), "1 minute"},
	{time.Now().Add(-time.Hour), "1 hour"},
	{time.Now().Add(-time.Hour).Add(time.Minute), "59 minutes"},
	{time.Now().Add(-time.Hour).Add(time.Minute).Add(10 * time.Second), "58 minutes"},
}

func TestGetRunningTimeString(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	runningTime := stateManager.GetRunningTimeString()
	assert.Empty(t, runningTime)

	// Lock and init start timestamp to simulate transfer
	assert.NoError(t, stateManager.TryLockTransferStateManager())
	running, err := stateManager.InitStartTimestamp()
	assert.NoError(t, err)
	assert.True(t, running)

	// Run test cases
	for _, testCase := range getRunningTimeStringCases {
		t.Run(testCase.startTimestamp.String(), func(t *testing.T) {
			// Set start timestamp
			stateManager.startTimestamp = testCase.startTimestamp

			// Assert running time string
			assert.Equal(t, testCase.expectedString, stateManager.GetRunningTimeString())
		})
	}
}

func TestStateConcurrency(t *testing.T) {
	stateManager, cleanUp := InitStateTest(t)
	defer cleanUp()

	// Concurrently increment variables in the state
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, stateManager.IncTransferredSizeAndFilesPhase1(1, 1))
			assert.NoError(t, stateManager.IncTransferredSizeAndFilesPhase2(1, 1))
			assert.NoError(t, stateManager.IncTransferredSizeAndFilesPhase3(1, 1))
			assert.NoError(t, stateManager.IncVisitedFolders())
			assert.NoError(t, stateManager.ChangeDelayedFilesCountBy(1, true))
			assert.NoError(t, stateManager.ChangeTransferFailureCountBy(1, true))
		}()
	}
	wg.Wait()

	// Assert 1000 in all values
	assert.Equal(t, 1000, int(stateManager.CurrentRepo.Phase1Info.TransferredSizeBytes))
	assert.Equal(t, 1000, int(stateManager.CurrentRepo.Phase1Info.TransferredUnits))
	assert.Equal(t, 1000, int(stateManager.CurrentRepo.Phase2Info.TransferredSizeBytes))
	assert.Equal(t, 1000, int(stateManager.CurrentRepo.Phase2Info.TransferredUnits))
	assert.Equal(t, 1000, int(stateManager.CurrentRepo.Phase3Info.TransferredSizeBytes))
	assert.Equal(t, 1000, int(stateManager.CurrentRepo.Phase3Info.TransferredUnits))
	assert.Equal(t, 1000, int(stateManager.OverallTransfer.TransferredSizeBytes))
	signedVisitedFolders, err := safeconvert.Uint64ToInt(stateManager.VisitedFolders)
	assert.NoError(t, err)
	assert.Equal(t, 1000, signedVisitedFolders)
	signedDelayedFiles, err := safeconvert.Uint64ToInt(stateManager.DelayedFiles)
	assert.NoError(t, err)
	assert.Equal(t, 1000, signedDelayedFiles)
	signedTransferFailures, err := safeconvert.Uint64ToInt(stateManager.TransferFailures)
	assert.NoError(t, err)
	assert.Equal(t, 1000, signedTransferFailures)

	// Concurrently decrement delayed artifacts and transfer failures
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, stateManager.ChangeDelayedFilesCountBy(1, false))
			assert.NoError(t, stateManager.ChangeTransferFailureCountBy(1, false))
		}()
	}
	wg.Wait()

	// Assert 500 in delayed artifacts and transfer failures
	signedDelayedFiles, err = safeconvert.Uint64ToInt(stateManager.DelayedFiles)
	assert.NoError(t, err)
	assert.Equal(t, 500, signedDelayedFiles)
	signedTransferFailures, err = safeconvert.Uint64ToInt(stateManager.TransferFailures)
	assert.NoError(t, err)
	assert.Equal(t, 500, signedTransferFailures)
}
