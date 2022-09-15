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
			{
				Status:           Success,
				ChecksumDeployed: false,
				SizeBytes:        10 * bytesInMB,
			},
			{
				Status:           Success,
				ChecksumDeployed: false,
				SizeBytes:        15 * bytesInMB,
			},
			{
				Status:           Success,
				ChecksumDeployed: true,
				SizeBytes:        8 * bytesInMB,
			},
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
			{
				Status:           Success,
				ChecksumDeployed: false,
				SizeBytes:        21.25 * bytesInMB,
			},
			{
				Status:           Fail,
				ChecksumDeployed: false,
				SizeBytes:        6 * bytesInMB,
			},
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
			{
				Status:           Success,
				ChecksumDeployed: true,
				SizeBytes:        8 * bytesInMB,
			},
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
			{
				Status:           Success,
				ChecksumDeployed: false,
				SizeBytes:        chunkSize,
			},
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
			{
				FileRepresentation: FileRepresentation{
					Repo: repo1Key,
				},
				Status:           Success,
				ChecksumDeployed: false,
				SizeBytes:        10 * bytesInMB,
			},
			{
				FileRepresentation: FileRepresentation{
					Repo: repo1Key,
				},
				Status: Success,
				// Checksum-deploy should not affect the update size.
				ChecksumDeployed: true,
				SizeBytes:        15 * bytesInMB,
			},
		},
	}
	timeEstMng.addChunkStatus(chunkStatus1, 3, true)

	// Add another chunk of repo1 which is not included in total. Expected not to be included in update.
	chunkStatus2 := ChunkStatus{
		DurationMillis: 10 * milliSecsInSecond,
		Files: []FileUploadStatusResponse{
			{
				FileRepresentation: FileRepresentation{
					Repo: repo1Key,
				},
				Status:           Success,
				ChecksumDeployed: false,
				SizeBytes:        21 * bytesInMB,
			},
		},
	}
	timeEstMng.addChunkStatus(chunkStatus2, 3, false)

	// Add a chunk of repo2 which is included in total. The failed file should be ignored.
	chunkStatus3 := ChunkStatus{
		DurationMillis: 10 * milliSecsInSecond,
		Files: []FileUploadStatusResponse{
			{
				FileRepresentation: FileRepresentation{
					Repo: repo2Key,
				},
				Status:           Success,
				ChecksumDeployed: false,
				SizeBytes:        13 * bytesInMB,
			},
			{
				FileRepresentation: FileRepresentation{
					Repo: repo2Key,
				},
				Status:           Fail,
				ChecksumDeployed: false,
				SizeBytes:        133 * bytesInMB,
			},
		},
	}
	timeEstMng.addChunkStatus(chunkStatus3, 3, true)
	saveAndAssertTransferredSizes(t, timeEstMng, chunkStatus1.Files[0].SizeBytes+chunkStatus1.Files[1].SizeBytes, chunkStatus3.Files[0].SizeBytes)

	// Add one more chunk of repo2.
	chunkStatus4 := ChunkStatus{
		DurationMillis: 10 * milliSecsInSecond,
		Files: []FileUploadStatusResponse{
			{
				FileRepresentation: FileRepresentation{
					Repo: repo2Key,
				},
				Status:           Success,
				ChecksumDeployed: false,
				SizeBytes:        9 * bytesInMB,
			},
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
