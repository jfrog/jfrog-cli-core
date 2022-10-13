package transferfiles

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetEstimatedRemainingTime(t *testing.T) {
	timeEstMng := newDefaultTimeEstimationManager()

	// Chunk 1: one of the files is checksum-deployed
	chunkStatus1 := ChunkStatus{
		DurationMillis: 10 * milliSecsInSecond,
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse("", 10*bytesInMB, false, Success),
			createFileUploadStatusResponse("", 15*bytesInMB, false, Success),
			createFileUploadStatusResponse("", 8*bytesInMB, true, Success),
		},
	}
	timeEstMng.addChunkStatus(chunkStatus1, 3, true)
	assert.Equal(t, 7.5, timeEstMng.getSpeed())
	assert.Equal(t, "7.500 MB/s", timeEstMng.getSpeedString())
	assert.Equal(t, int64(62), timeEstMng.getEstimatedRemainingTime())
	assert.Equal(t, "About 1 minute", timeEstMng.getEstimatedRemainingTimeString())

	// Chunk 2: the upload of one of the files failed and the files are not included in the repository's total size (includedInTotalSize == false)
	chunkStatus2 := ChunkStatus{
		DurationMillis: 5 * milliSecsInSecond,
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse("", 21.25*bytesInMB, false, Success),
			createFileUploadStatusResponse("", 6*bytesInMB, false, Fail),
		},
	}
	timeEstMng.addChunkStatus(chunkStatus2, 2, false)
	assert.Equal(t, float64(8), timeEstMng.getSpeed())
	assert.Equal(t, "8.000 MB/s", timeEstMng.getSpeedString())
	assert.Equal(t, int64(58), timeEstMng.getEstimatedRemainingTime())
	assert.Equal(t, "Less than a minute", timeEstMng.getEstimatedRemainingTimeString())
}

func TestGetEstimatedRemainingTimeStringNotAvailableYet(t *testing.T) {
	timeEstMng := newDefaultTimeEstimationManager()

	// Chunk 1: one of the files is checksum-deployed
	chunkStatus1 := ChunkStatus{
		DurationMillis: 10 * milliSecsInSecond,
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse("", 8*bytesInMB, true, Success),
		},
	}
	timeEstMng.addChunkStatus(chunkStatus1, 3, true)
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
	// For tests convenience, we use 0 transferred bytes and 1/milliseconds speedsAverage because then the total bytes equals the remaining time.
	timeEstMng := newTimeEstimationManager(totalBytes, 0)
	timeEstMng.speedsAverage = 1.0 / milliSecsInSecond
	// Not taken into account in calculation, just needed to mark the transfer has started and estimation should be done.
	timeEstMng.timeEstimationUnavailable = false
	timeEstMng.lastSpeeds = []float64{1.1}

	assert.Equal(t, expectedEstimation, timeEstMng.getEstimatedRemainingTimeString())
}

func TestEstimationNotAvailable(t *testing.T) {
	timeEstMng := newDefaultTimeEstimationManager()
	// Assert unavailable if set.
	timeEstMng.setTimeEstimationUnavailable(true)
	assert.Equal(t, "Not available in this phase", timeEstMng.getEstimatedRemainingTimeString())

	// After made available, assert not available until lastSpeeds are set.
	timeEstMng.setTimeEstimationUnavailable(false)
	assert.Equal(t, "Not available yet", timeEstMng.getEstimatedRemainingTimeString())

	timeEstMng.lastSpeeds = []float64{1.23}
	assert.Equal(t, "Less than a minute", timeEstMng.getEstimatedRemainingTimeString())
}

func newDefaultTimeEstimationManager() *timeEstimationManager {
	return newTimeEstimationManager(600*bytesInMB, 100*bytesInMB)
}

func TestAddingToFullLastSpeedsSlice(t *testing.T) {
	timeEstMng := newDefaultTimeEstimationManager()
	numOfSpeedsToKeepPerWorkingThread = 3

	// Fill the last speeds slice (up to threads * numOfSpeedsToKeepPerWorkingThread).
	timeEstMng.lastSpeeds = []float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6}

	// Add a chunk and assert the oldest speed is removed, new is added, and the chunk len remains the same.
	firstChunkSpeed := addOneFileChunk(timeEstMng, 2, 20, 1)
	assert.Equal(t, []float64{2.2, 3.3, 4.4, 5.5, 6.6, firstChunkSpeed}, timeEstMng.lastSpeeds)

	// Lower threads and add a chunk. Expecting the slice to shrink and the new speed to be added in the end.
	secondChunkSpeed := addOneFileChunk(timeEstMng, 1, 30, 2)
	assert.Equal(t, []float64{6.6, firstChunkSpeed, secondChunkSpeed}, timeEstMng.lastSpeeds)

	// Increase threads and add a chunk. Expecting the slice len to increase with the new speed.
	thirdChunkSpeed := addOneFileChunk(timeEstMng, 3, 40, 3)
	assert.Equal(t, []float64{6.6, firstChunkSpeed, secondChunkSpeed, thirdChunkSpeed}, timeEstMng.lastSpeeds)
}

// Adds a chunk with one non checksum-deployed file and calculates and returns the chunk speed.
func addOneFileChunk(timeEstMng *timeEstimationManager, workingThreads, chunkDurationMilli, chunkSizeMb int, totalSize int64) float64 {
	chunkDuration := int64(chunkDurationMilli * milliSecsInSecond)
	chunkSize := int64(chunkSizeMb * bytesInMB)
	chunkStatus := ChunkStatus{
		DurationMillis: chunkDuration,
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse("", chunkSize, false, Success),
		},
	}

	timeEstMng.addChunkStatus(chunkStatus, workingThreads, true, totalSize)
	return calculateChunkSpeed(workingThreads, chunkSize, chunkDuration)
}

func TestTransferredSizeInState(t *testing.T) {
	cleanUp := initStateTest(t)
	defer cleanUp()

	timeEstMng := newDefaultTimeEstimationManager()

	// Create repos in state.
	assert.NoError(t, resetRepoState(repo1Key))
	assert.NoError(t, resetRepoState(repo2Key))
	saveAndAssertTransferredSizes(t, timeEstMng, 0, 0)

	// Add a chunk of repo1 with multiple successful files, which are included in total.
	chunkStatus1 := ChunkStatus{
		DurationMillis: 10 * milliSecsInSecond,
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse(repo1Key, 10*bytesInMB, false, Success),
			// Checksum-deploy should not affect the update size.
			createFileUploadStatusResponse(repo1Key, 15*bytesInMB, true, Success),
		},
	}
	timeEstMng.addChunkStatus(chunkStatus1, 3, true)

	// Add another chunk of repo1 which is not included in total. Expected not to be included in update.
	chunkStatus2 := ChunkStatus{
		DurationMillis: 10 * milliSecsInSecond,
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse(repo1Key, 21*bytesInMB, false, Success),
		},
	}
	timeEstMng.addChunkStatus(chunkStatus2, 3, false)

	// Add a chunk of repo2 which is included in total. The failed file should be ignored.
	chunkStatus3 := ChunkStatus{
		DurationMillis: 10 * milliSecsInSecond,
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse(repo2Key, 13*bytesInMB, false, Success),
			createFileUploadStatusResponse(repo2Key, 133*bytesInMB, false, Fail),
		},
	}
	timeEstMng.addChunkStatus(chunkStatus3, 3, true)
	saveAndAssertTransferredSizes(t, timeEstMng, chunkStatus1.Files[0].SizeBytes+chunkStatus1.Files[1].SizeBytes, chunkStatus3.Files[0].SizeBytes)

	// Add one more chunk of repo2.
	chunkStatus4 := ChunkStatus{
		DurationMillis: 10 * milliSecsInSecond,
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse(repo2Key, 9*bytesInMB, false, Success),
		},
	}
	timeEstMng.addChunkStatus(chunkStatus4, 3, true)
	saveAndAssertTransferredSizes(t, timeEstMng, chunkStatus1.Files[0].SizeBytes+chunkStatus1.Files[1].SizeBytes, chunkStatus3.Files[0].SizeBytes+chunkStatus4.Files[0].SizeBytes)
}

func saveAndAssertTransferredSizes(t *testing.T, timeEstMng *timeEstimationManager, repo1expected, repo2expected int64) {
	assert.NoError(t, timeEstMng.saveTransferredSizeInState())
	assertTransferredSize(t, repo1expected, repo1Key)
	assertTransferredSize(t, repo2expected, repo2Key)
}

func assertTransferredSize(t *testing.T, expectedSize int64, repoKeys ...string) {
	totalTransferredSize, err := getReposTransferredSizeBytes(repoKeys...)
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
