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
	assert.Equal(t, "About 1 minutes", timeEstMng.getEstimatedRemainingTimeString())

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
func addOneFileChunk(timeEstMng *timeEstimationManager, workingThreads, chunkDurationMilli, chunkSizeMb int) float64 {
	chunkDuration := int64(chunkDurationMilli * milliSecsInSecond)
	chunkSize := int64(chunkSizeMb * bytesInMB)
	chunkStatus := ChunkStatus{
		DurationMillis: chunkDuration,
		Files: []FileUploadStatusResponse{
			createFileUploadStatusResponse("", chunkSize, false, Success),
		},
	}
	timeEstMng.addChunkStatus(chunkStatus, workingThreads, true)
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
