package state

import (
	"testing"
	"time"

	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/stretchr/testify/assert"
)

func initTimeEstimationTestSuite(t *testing.T) func() {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)

	// Create transfer directory
	transferDir, err := coreutils.GetJfrogTransferDir()
	assert.NoError(t, err)
	err = utils.CreateDirIfNotExist(transferDir)
	assert.NoError(t, err)

	undoSaveInterval := SetAutoSaveState()
	return func() {
		undoSaveInterval()
		cleanUpJfrogHome()
	}
}

func initTimeEstimationDataTest(t *testing.T) (*TimeEstimationManager, func()) {
	cleanUpJfrogHome := initTimeEstimationTestSuite(t)
	return newDefaultTimeEstimationManager(t, false), cleanUpJfrogHome
}

func TestGetSpeed(t *testing.T) {
	timeEstMng, cleanUp := initTimeEstimationDataTest(t)
	defer cleanUp()

	// Chunk 1: one of the files is checksum-deployed
	chunkStatus1 := api.ChunkStatus{
		Files: []api.FileUploadStatusResponse{
			createFileUploadStatusResponse(repo1Key, 10*bytesInMB, false, api.Success),
			createFileUploadStatusResponse(repo1Key, 15*bytesInMB, false, api.Success),
			createFileUploadStatusResponse(repo1Key, 8*bytesInMB, true, api.Success),
		},
	}
	addChunkStatus(t, timeEstMng, chunkStatus1, 3, true, 10*milliSecsInSecond)
	assert.Equal(t, 7.5, timeEstMng.getSpeed())
	assert.Equal(t, "7.500 MB/s", timeEstMng.GetSpeedString())
	assert.NotZero(t, timeEstMng.getEstimatedRemainingSeconds())

	// Chunk 2: the upload of one of the files failed and the files are not included in the repository's total size (includedInTotalSize == false)
	chunkStatus2 := api.ChunkStatus{
		Files: []api.FileUploadStatusResponse{
			createFileUploadStatusResponse(repo1Key, 21.25*bytesInMB, false, api.Success),
			createFileUploadStatusResponse(repo1Key, 6*bytesInMB, false, api.Fail),
		},
	}
	addChunkStatus(t, timeEstMng, chunkStatus2, 2, false, 5*milliSecsInSecond)
	assert.Equal(t, float64(8), timeEstMng.getSpeed())
	assert.Equal(t, "8.000 MB/s", timeEstMng.GetSpeedString())
	assert.NotZero(t, timeEstMng.getEstimatedRemainingSeconds())
}

func TestGetEstimatedRemainingSeconds(t *testing.T) {
	timeEstMng, cleanUp := initTimeEstimationDataTest(t)
	defer cleanUp()

	timeEstMng.CurrentTotalTransferredBytes = uint64(timeEstMng.stateManager.OverallTransfer.TotalSizeBytes)
	timeEstMng.stateManager.OverallTransfer.TransferredSizeBytes = timeEstMng.stateManager.OverallTransfer.TotalSizeBytes
	assert.Zero(t, timeEstMng.getEstimatedRemainingSeconds())

	timeEstMng.CurrentTotalTransferredBytes = uint64(timeEstMng.stateManager.OverallTransfer.TotalSizeBytes) / 2
	timeEstMng.stateManager.OverallTransfer.TransferredSizeBytes = timeEstMng.stateManager.OverallTransfer.TotalSizeBytes / 2
	calculatedEstimatedSeconds := timeEstMng.getEstimatedRemainingSeconds()
	assert.NotZero(t, calculatedEstimatedSeconds)
}

func TestGetEstimatedRemainingTimeStringNotAvailableYet(t *testing.T) {
	timeEstMng, cleanUp := initTimeEstimationDataTest(t)
	defer cleanUp()

	// Chunk 1: one of the files is checksum-deployed
	chunkStatus1 := api.ChunkStatus{
		Files: []api.FileUploadStatusResponse{
			createFileUploadStatusResponse(repo1Key, 8*bytesInMB, true, api.Success),
		},
	}
	assert.Equal(t, "Not available yet", timeEstMng.GetEstimatedRemainingTimeString())
	addChunkStatus(t, timeEstMng, chunkStatus1, 3, true, 10*milliSecsInSecond)
	assert.Equal(t, "Not available yet", timeEstMng.GetSpeedString())
}

func TestGetEstimatedRemainingTimeString(t *testing.T) {
	timeEstMng, cleanUp := initTimeEstimationDataTest(t)
	defer cleanUp()

	// Test "Not available yet" by setting the TotalTransferredBytes to 0
	timeEstMng.CurrentTotalTransferredBytes = 0
	assert.Equal(t, "Not available yet", timeEstMng.GetEstimatedRemainingTimeString())

	// Test "About 1 minute" by setting the transferred bytes to 80%
	timeEstMng.CurrentTotalTransferredBytes = uint64(float64(timeEstMng.stateManager.OverallTransfer.TotalSizeBytes) * 0.8)
	timeEstMng.stateManager.OverallTransfer.TransferredSizeBytes = int64(float64(timeEstMng.stateManager.OverallTransfer.TotalSizeBytes) * 0.8)
	assert.Equal(t, "About 1 minute", timeEstMng.GetEstimatedRemainingTimeString())

	// Test "Less than a minute" by setting the transferred bytes to 90%
	timeEstMng.CurrentTotalTransferredBytes = uint64(float64(timeEstMng.stateManager.OverallTransfer.TotalSizeBytes) * 0.9)
	timeEstMng.stateManager.OverallTransfer.TransferredSizeBytes = int64(float64(timeEstMng.stateManager.OverallTransfer.TotalSizeBytes) * 0.9)
	assert.Equal(t, "Less than a minute", timeEstMng.GetEstimatedRemainingTimeString())
}

func newDefaultTimeEstimationManager(t *testing.T, buildInfoRepos bool) *TimeEstimationManager {
	stateManager, err := NewTransferStateManager(true)
	stateManager.startTimestamp = time.Now().Add(-minTransferTimeToShowEstimation)
	assert.NoError(t, err)
	assert.NoError(t, stateManager.SetRepoState(repo1Key, 0, 0, buildInfoRepos, true))
	assert.NoError(t, stateManager.SetRepoState(repo2Key, 0, 0, buildInfoRepos, true))

	assert.NoError(t, stateManager.IncTransferredSizeAndFilesPhase1(0, 100*bytesInMB))
	stateManager.OverallTransfer.TotalSizeBytes = 600 * bytesInMB
	return &TimeEstimationManager{stateManager: stateManager}
}

func TestAddingToFullLastSpeedsSlice(t *testing.T) {
	timeEstMng, cleanUp := initTimeEstimationDataTest(t)
	defer cleanUp()

	numOfSpeedsToKeepPerWorkingThread = 3

	// Fill the last speeds slice (up to threads * numOfSpeedsToKeepPerWorkingThread).
	timeEstMng.LastSpeeds = []float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6}

	// Add a chunk and assert the oldest speed is removed, new is added, and the chunk len remains the same.
	firstChunkSpeed := addOneFileChunk(t, timeEstMng, 2, 20, 1)
	assert.Equal(t, []float64{2.2, 3.3, 4.4, 5.5, 6.6, firstChunkSpeed}, timeEstMng.LastSpeeds)

	// Lower threads and add a chunk. Expecting the slice to shrink and the new speed to be added in the end.
	secondChunkSpeed := addOneFileChunk(t, timeEstMng, 1, 30, 2)
	assert.Equal(t, []float64{6.6, firstChunkSpeed, secondChunkSpeed}, timeEstMng.LastSpeeds)

	// Increase threads and add a chunk. Expecting the slice len to increase with the new speed.
	thirdChunkSpeed := addOneFileChunk(t, timeEstMng, 3, 40, 3)
	assert.Equal(t, []float64{6.6, firstChunkSpeed, secondChunkSpeed, thirdChunkSpeed}, timeEstMng.LastSpeeds)
}

// Adds a chunk with one non checksum-deployed file and calculates and returns the chunk speed.
func addOneFileChunk(t *testing.T, timeEstMng *TimeEstimationManager, workingThreads, chunkDurationMilli, chunkSizeMb int) float64 {
	chunkDuration := int64(chunkDurationMilli * milliSecsInSecond)
	chunkSize := int64(chunkSizeMb * bytesInMB)
	chunkStatus := api.ChunkStatus{
		Files: []api.FileUploadStatusResponse{
			createFileUploadStatusResponse(repo1Key, chunkSize, false, api.Success),
		},
	}
	addChunkStatus(t, timeEstMng, chunkStatus, workingThreads, true, chunkDuration)
	return calculateChunkSpeed(workingThreads, chunkSize, chunkDuration)
}

func TestTransferredSizeInState(t *testing.T) {
	cleanUp := initTimeEstimationTestSuite(t)
	defer cleanUp()

	stateManager, err := NewTransferStateManager(true)
	assert.NoError(t, err)
	timeEstMng := &TimeEstimationManager{stateManager: stateManager}

	// Create repo1 in state.
	assert.NoError(t, timeEstMng.stateManager.SetRepoState(repo1Key, 0, 0, false, true))
	assertTransferredSizes(t, timeEstMng.stateManager, 0, 0)

	// Add a chunk of repo1 with multiple successful files, which are included in total.
	chunkStatus1 := api.ChunkStatus{
		Files: []api.FileUploadStatusResponse{
			createFileUploadStatusResponse(repo1Key, 10*bytesInMB, false, api.Success),
			// Checksum-deploy should not affect the update size.
			createFileUploadStatusResponse(repo1Key, 15*bytesInMB, true, api.Success),
		},
	}
	addChunkStatus(t, timeEstMng, chunkStatus1, 3, true, 10*milliSecsInSecond)

	// Add another chunk of repo1 which is not included in total. Expected not to be included in update.
	chunkStatus2 := api.ChunkStatus{
		Files: []api.FileUploadStatusResponse{
			createFileUploadStatusResponse(repo1Key, 21*bytesInMB, false, api.Success),
		},
	}
	addChunkStatus(t, timeEstMng, chunkStatus2, 3, false, 10*milliSecsInSecond)

	// Create repo2 in state.
	assert.NoError(t, timeEstMng.stateManager.SetRepoState(repo2Key, 0, 0, false, true))

	// Add a chunk of repo2 which is included in total. The failed file should be ignored.
	chunkStatus3 := api.ChunkStatus{
		Files: []api.FileUploadStatusResponse{
			createFileUploadStatusResponse(repo2Key, 13*bytesInMB, false, api.Success),
			createFileUploadStatusResponse(repo2Key, 133*bytesInMB, false, api.Fail),
		},
	}
	addChunkStatus(t, timeEstMng, chunkStatus3, 3, true, 10*milliSecsInSecond)
	assertTransferredSizes(t, timeEstMng.stateManager, chunkStatus1.Files[0].SizeBytes+chunkStatus1.Files[1].SizeBytes, chunkStatus3.Files[0].SizeBytes)

	// Add one more chunk of repo2.
	chunkStatus4 := api.ChunkStatus{
		Files: []api.FileUploadStatusResponse{
			createFileUploadStatusResponse(repo2Key, 9*bytesInMB, false, api.Success),
		},
	}
	addChunkStatus(t, timeEstMng, chunkStatus4, 3, true, 10*milliSecsInSecond)
	assertTransferredSizes(t, timeEstMng.stateManager, chunkStatus1.Files[0].SizeBytes+chunkStatus1.Files[1].SizeBytes, chunkStatus3.Files[0].SizeBytes+chunkStatus4.Files[0].SizeBytes)
}

func addChunkStatus(t *testing.T, timeEstMng *TimeEstimationManager, chunkStatus api.ChunkStatus, workingThreads int, includedInTotalSize bool, durationMillis int64) {
	if includedInTotalSize {
		err := UpdateChunkInState(timeEstMng.stateManager, &chunkStatus)
		assert.NoError(t, err)
	}
	assert.NoError(t, timeEstMng.stateManager.SetWorkingThreads(workingThreads))
	timeEstMng.AddChunkStatus(chunkStatus, durationMillis)
}

func assertTransferredSizes(t *testing.T, stateManager *TransferStateManager, repo1expected, repo2expected int64) {
	assertReposTransferredSize(t, stateManager, repo1expected, repo1Key)
	assertReposTransferredSize(t, stateManager, repo2expected, repo2Key)
}

func createFileUploadStatusResponse(repoKey string, sizeBytes int64, checksumDeployed bool, status api.ChunkFileStatusType) api.FileUploadStatusResponse {
	return api.FileUploadStatusResponse{
		FileRepresentation: api.FileRepresentation{
			Repo: repoKey,
			Path: "path",
			Name: "name",
		},
		SizeBytes:        sizeBytes,
		ChecksumDeployed: checksumDeployed,
		Status:           status,
	}
}
