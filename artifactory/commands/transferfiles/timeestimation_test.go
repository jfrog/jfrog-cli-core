package transferfiles

import (
	"testing"

	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/stretchr/testify/assert"
)

const (
	repo1Key = "repo1"
	repo2Key = "repo2"
)

func initTimeEstimationTest(t *testing.T) (*timeEstimationManager, func()) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)

	// Create transfer directory
	transferDir, err := coreutils.GetJfrogTransferDir()
	assert.NoError(t, err)
	err = utils.CreateDirIfNotExist(transferDir)
	assert.NoError(t, err)

	return newDefaultTimeEstimationManager(t), cleanUpJfrogHome
}

func TestGetEstimatedRemainingTime(t *testing.T) {
	timeEstMng, cleanUp := initTimeEstimationTest(t)
	defer cleanUp()

	// Chunk 1: one of the files is checksum-deployed
	chunkStatus1 := ChunkStatus{
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse(repo1Key, 10*bytesInMB, false, Success),
			createFileUploadStatusResponse(repo1Key, 15*bytesInMB, false, Success),
			createFileUploadStatusResponse(repo1Key, 8*bytesInMB, true, Success),
		},
	}
	addChunkStatus(t, timeEstMng, chunkStatus1, 3, true, 10*milliSecsInSecond)
	assert.Equal(t, 7.5, timeEstMng.getSpeed())
	assert.Equal(t, "7.500 MB/s", timeEstMng.getSpeedString())
	estimatedRemainingTime, err := timeEstMng.getEstimatedRemainingTime()
	assert.NoError(t, err)
	assert.Equal(t, int64(62), estimatedRemainingTime)
	assert.Equal(t, "About 1 minute", timeEstMng.getEstimatedRemainingTimeString())

	// Chunk 2: the upload of one of the files failed and the files are not included in the repository's total size (includedInTotalSize == false)
	chunkStatus2 := ChunkStatus{
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse(repo1Key, 21.25*bytesInMB, false, Success),
			createFileUploadStatusResponse(repo1Key, 6*bytesInMB, false, Fail),
		},
	}
	addChunkStatus(t, timeEstMng, chunkStatus2, 2, false, 5*milliSecsInSecond)
	assert.Equal(t, float64(8), timeEstMng.getSpeed())
	assert.Equal(t, "8.000 MB/s", timeEstMng.getSpeedString())
	estimatedRemainingTime, err = timeEstMng.getEstimatedRemainingTime()
	assert.NoError(t, err)
	assert.Equal(t, int64(58), estimatedRemainingTime)
	assert.Equal(t, "Less than a minute", timeEstMng.getEstimatedRemainingTimeString())
}

func TestGetEstimatedRemainingTimeStringNotAvailableYet(t *testing.T) {
	timeEstMng, cleanUp := initTimeEstimationTest(t)
	defer cleanUp()

	// Chunk 1: one of the files is checksum-deployed
	chunkStatus1 := ChunkStatus{
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse(repo1Key, 8*bytesInMB, true, Success),
		},
	}
	addChunkStatus(t, timeEstMng, chunkStatus1, 3, true, 10*milliSecsInSecond)
	assert.Equal(t, "Not available yet", timeEstMng.getSpeedString())
	assert.Equal(t, "Not available yet", timeEstMng.getEstimatedRemainingTimeString())
}

func TestGetEstimatedRemainingTimeString(t *testing.T) {
	getEstimatedRemainingTimeStringCases := []struct {
		name                 string
		expectedEstimation   string
		timeRemainingSeconds int64
	}{
		{"plural days and plural hours", "About 11 days and 13 hours", getTimeInSecs(11, 13, 3, 7)},
		{"plural days and singular hour", "About 5 days and 1 hour", getTimeInSecs(5, 1, 2, 0)},
		{"plural days", "About 3 days", getTimeInSecs(3, 0, 4, 0)},
		{"singular day and plural hours", "About 1 day and 2 hours", getTimeInSecs(1, 2, 6, 6)},
		{"singular day and singular hour", "About 1 day and 1 hour", getTimeInSecs(1, 1, 6, 6)},
		{"singular day", "About 1 day", getTimeInSecs(1, 0, 4, 0)},
		{"plural hours and plural minutes", "About 11 hours and 13 minutes", getTimeInSecs(0, 11, 13, 0)},
		{"plural hours and singular minute", "About 5 hours and 1 minute", getTimeInSecs(0, 5, 1, 6)},
		{"plural hours", "About 3 hours", getTimeInSecs(0, 3, 0, 3)},
		{"singular hours and plural minutes", "About 1 hour and 13 minutes", getTimeInSecs(0, 1, 13, 0)},
		{"singular hours and singular minute", "About 1 hour and 1 minute", getTimeInSecs(0, 1, 1, 6)},
		{"singular hour", "About 1 hour", getTimeInSecs(0, 1, 0, 3)},
		{"plural minutes", "About 10 minutes", getTimeInSecs(0, 0, 10, 3)},
		{"singular minute", "About 1 minute", getTimeInSecs(0, 0, 1, 3)},
		{"seconds", "Less than a minute", getTimeInSecs(0, 0, 0, 3)},
	}

	for _, testCase := range getEstimatedRemainingTimeStringCases {
		t.Run(testCase.name, func(t *testing.T) {
			assertGetEstimatedRemainingTimeString(t, testCase.timeRemainingSeconds, testCase.expectedEstimation)
		})
	}
}

func getTimeInSecs(days, hours, minutes, seconds int64) int64 {
	return 86400*days + 3600*hours + 60*minutes + seconds
}

func assertGetEstimatedRemainingTimeString(t *testing.T, totalBytes int64, expectedEstimation string) {
	timeEstMng, cleanUp := initTimeEstimationTest(t)
	defer cleanUp()

	// For tests convenience, we use 0 transferred bytes and 1/milliseconds speedsAverage because then the total bytes equals the remaining time.
	timeEstMng.stateManager.TransferredSizeBytes = 0
	timeEstMng.stateManager.TotalSizeBytes = totalBytes
	timeEstMng.speedsAverage = 1.0 / milliSecsInSecond
	// Not taken into account in calculation, just needed to mark the transfer has started and estimation should be done.
	timeEstMng.timeEstimationUnavailable = false
	timeEstMng.lastSpeeds = []float64{1.1}

	assert.Equal(t, expectedEstimation, timeEstMng.getEstimatedRemainingTimeString())
}

func TestEstimationNotAvailable(t *testing.T) {
	timeEstMng, cleanUp := initTimeEstimationTest(t)
	defer cleanUp()

	// Assert unavailable if set.
	timeEstMng.setTimeEstimationUnavailable(true)
	assert.Equal(t, "Not available in this phase", timeEstMng.getEstimatedRemainingTimeString())

	// After made available, assert not available until lastSpeeds are set.
	timeEstMng.setTimeEstimationUnavailable(false)
	assert.Equal(t, "Not available yet", timeEstMng.getEstimatedRemainingTimeString())

	timeEstMng.lastSpeeds = []float64{1.23}
	assert.Equal(t, "Less than a minute", timeEstMng.getEstimatedRemainingTimeString())
}

func newDefaultTimeEstimationManager(t *testing.T) *timeEstimationManager {
	stateManager, err := state.NewTransferStateManager(true)
	assert.NoError(t, err)
	assert.NoError(t, stateManager.SetRepoState(repo1Key, 0, 0, true))
	assert.NoError(t, stateManager.SetRepoState(repo2Key, 0, 0, true))

	assert.NoError(t, stateManager.IncTransferredSizeAndFiles(repo1Key, 0, 100*bytesInMB))
	stateManager.TotalSizeBytes = 600 * bytesInMB
	return newTimeEstimationManager(stateManager, 0)
}

func TestAddingToFullLastSpeedsSlice(t *testing.T) {
	timeEstMng, cleanUp := initTimeEstimationTest(t)
	defer cleanUp()

	numOfSpeedsToKeepPerWorkingThread = 3

	// Fill the last speeds slice (up to threads * numOfSpeedsToKeepPerWorkingThread).
	timeEstMng.lastSpeeds = []float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6}

	// Add a chunk and assert the oldest speed is removed, new is added, and the chunk len remains the same.
	firstChunkSpeed := addOneFileChunk(t, timeEstMng, 2, 20, 1)
	assert.Equal(t, []float64{2.2, 3.3, 4.4, 5.5, 6.6, firstChunkSpeed}, timeEstMng.lastSpeeds)

	// Lower threads and add a chunk. Expecting the slice to shrink and the new speed to be added in the end.
	secondChunkSpeed := addOneFileChunk(t, timeEstMng, 1, 30, 2)
	assert.Equal(t, []float64{6.6, firstChunkSpeed, secondChunkSpeed}, timeEstMng.lastSpeeds)

	// Increase threads and add a chunk. Expecting the slice len to increase with the new speed.
	thirdChunkSpeed := addOneFileChunk(t, timeEstMng, 3, 40, 3)
	assert.Equal(t, []float64{6.6, firstChunkSpeed, secondChunkSpeed, thirdChunkSpeed}, timeEstMng.lastSpeeds)
}

// Adds a chunk with one non checksum-deployed file and calculates and returns the chunk speed.
func addOneFileChunk(t *testing.T, timeEstMng *timeEstimationManager, workingThreads, chunkDurationMilli, chunkSizeMb int) float64 {
	chunkDuration := int64(chunkDurationMilli * milliSecsInSecond)
	chunkSize := int64(chunkSizeMb * bytesInMB)
	chunkStatus := ChunkStatus{
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse("", chunkSize, false, Success),
		},
	}
	addChunkStatus(t, timeEstMng, chunkStatus, workingThreads, true, chunkDuration)
	return calculateChunkSpeed(workingThreads, chunkSize, chunkDuration)
}

func TestTransferredSizeInState(t *testing.T) {
	timeEstMng, cleanUp := initTimeEstimationTest(t)
	defer cleanUp()

	// Create repos in state.
	assert.NoError(t, timeEstMng.stateManager.SetRepoState(repo1Key, 0, 0, true))
	assert.NoError(t, timeEstMng.stateManager.SetRepoState(repo2Key, 0, 0, true))
	assertTransferredSizes(t, timeEstMng.stateManager, timeEstMng, 0, 0)

	// Add a chunk of repo1 with multiple successful files, which are included in total.
	chunkStatus1 := ChunkStatus{
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse(repo1Key, 10*bytesInMB, false, Success),
			// Checksum-deploy should not affect the update size.
			createFileUploadStatusResponse(repo1Key, 15*bytesInMB, true, Success),
		},
	}
	addChunkStatus(t, timeEstMng, chunkStatus1, 3, true, 10*milliSecsInSecond)

	// Add another chunk of repo1 which is not included in total. Expected not to be included in update.
	chunkStatus2 := ChunkStatus{
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse(repo1Key, 21*bytesInMB, false, Success),
		},
	}
	addChunkStatus(t, timeEstMng, chunkStatus2, 3, false, 10*milliSecsInSecond)

	// Add a chunk of repo2 which is included in total. The failed file should be ignored.
	chunkStatus3 := ChunkStatus{
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse(repo2Key, 13*bytesInMB, false, Success),
			createFileUploadStatusResponse(repo2Key, 133*bytesInMB, false, Fail),
		},
	}
	addChunkStatus(t, timeEstMng, chunkStatus3, 3, true, 10*milliSecsInSecond)
	assertTransferredSizes(t, timeEstMng.stateManager, timeEstMng, chunkStatus1.Files[0].SizeBytes+chunkStatus1.Files[1].SizeBytes, chunkStatus3.Files[0].SizeBytes)

	// Add one more chunk of repo2.
	chunkStatus4 := ChunkStatus{
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse(repo2Key, 9*bytesInMB, false, Success),
		},
	}
	addChunkStatus(t, timeEstMng, chunkStatus4, 3, true, 10*milliSecsInSecond)
	assertTransferredSizes(t, timeEstMng.stateManager, timeEstMng, chunkStatus1.Files[0].SizeBytes+chunkStatus1.Files[1].SizeBytes, chunkStatus3.Files[0].SizeBytes+chunkStatus4.Files[0].SizeBytes)
}

func addChunkStatus(t *testing.T, timeEstMng *timeEstimationManager, chunkStatus ChunkStatus, workingThreads int, includedInTotalSize bool, durationMillis int64) {
	if includedInTotalSize {
		assert.NoError(t, updateChunkInState(timeEstMng.stateManager, chunkStatus.Files[0].Repo, &chunkStatus))
	}
	assert.NoError(t, timeEstMng.stateManager.SetWorkingThreads(workingThreads))
	timeEstMng.addChunkStatus(chunkStatus, durationMillis)
}

func assertTransferredSizes(t *testing.T, stateManager *state.TransferStateManager, timeEstMng *timeEstimationManager, repo1expected, repo2expected int64) {
	assertTransferredSize(t, stateManager, repo1expected, repo1Key)
	assertTransferredSize(t, stateManager, repo2expected, repo2Key)
}

func assertTransferredSize(t *testing.T, stateManager *state.TransferStateManager, expectedSize int64, repoKeys ...string) {
	totalTransferredSize, err := stateManager.GetReposTransferredSizeBytes(repoKeys...)
	assert.NoError(t, err)
	assert.Equal(t, expectedSize, totalTransferredSize)
}

func createFileUploadStatusResponse(repoKey string, sizeBytes int64, checksumDeployed bool, status ChunkFileStatusType) FileUploadStatusResponse {
	return FileUploadStatusResponse{
		FileRepresentation: FileRepresentation{
			Repo: repoKey,
		},
		SizeBytes:        sizeBytes,
		ChecksumDeployed: checksumDeployed,
		Status:           status,
	}
}
