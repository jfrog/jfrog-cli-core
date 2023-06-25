package state

import (
	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"testing"

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

func initTimeEstimationBITest(t *testing.T) (*TimeEstimationManager, func()) {
	cleanUpJfrogHome := initTimeEstimationTestSuite(t)
	return newDefaultTimeEstimationManager(t, true), cleanUpJfrogHome
}

func TestGetDataEstimatedRemainingTime(t *testing.T) {
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
	assertGetEstimatedRemainingTime(t, timeEstMng, int64(62))
	assert.Equal(t, "About 1 minute", timeEstMng.GetEstimatedRemainingTimeString())

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
	assertGetEstimatedRemainingTime(t, timeEstMng, int64(58))
	assert.Equal(t, "Less than a minute", timeEstMng.GetEstimatedRemainingTimeString())
}

func TestGetBuildInfoEstimatedRemainingTime(t *testing.T) {
	timeEstMng, cleanUp := initTimeEstimationBITest(t)
	defer cleanUp()

	totalBiFiles := 100.0
	timeEstMng.stateManager.OverallBiFiles.TotalUnits = int64(totalBiFiles)
	assertGetEstimatedRemainingTime(t, timeEstMng, int64(totalBiFiles*buildInfoAverageIndexTimeSec))

	chunkStatus1 := api.ChunkStatus{
		Files: []api.FileUploadStatusResponse{
			createFileUploadStatusResponse(repo1Key, 10*bytesInMB, false, api.Success),
			createFileUploadStatusResponse(repo1Key, 15*bytesInMB, false, api.Success),
		},
	}
	err := UpdateChunkInState(timeEstMng.stateManager, &chunkStatus1)
	assert.NoError(t, err)
	assertGetEstimatedRemainingTime(t, timeEstMng, int64((totalBiFiles-2)*buildInfoAverageIndexTimeSec))
}

func TestGetCombinedEstimatedRemainingTime(t *testing.T) {
	timeEstMng, cleanUp := initTimeEstimationDataTest(t)
	defer cleanUp()

	totalBiFiles := 100.0
	timeEstMng.stateManager.OverallBiFiles.TotalUnits = int64(totalBiFiles)

	// Start transferring a data repository, make sure the remaining time includes the estimation of both bi and data.
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
	assert.NoError(t, timeEstMng.calculateDataEstimatedRemainingTime())
	assert.Equal(t, int64(62), timeEstMng.DataEstimatedRemainingTime)
	assert.Equal(t, int64(41), timeEstMng.getBuildInfoEstimatedRemainingTime())
	assertGetEstimatedRemainingTime(t, timeEstMng, int64(103))

	// Change to transferring a bi repository.
	assert.NoError(t, timeEstMng.stateManager.SetRepoState(repo3Key, 0, 0, true, true))
	chunkStatus2 := api.ChunkStatus{
		Files: []api.FileUploadStatusResponse{
			createFileUploadStatusResponse(repo3Key, 10*bytesInMB, false, api.Success),
			createFileUploadStatusResponse(repo3Key, 15*bytesInMB, false, api.Success),
		},
	}
	addChunkStatus(t, timeEstMng, chunkStatus2, 3, true, 10*milliSecsInSecond)
	assert.Equal(t, "Not available while transferring a build-info repository", timeEstMng.GetSpeedString())
	// Data estimated time should remain as it was before:
	assert.NoError(t, timeEstMng.calculateDataEstimatedRemainingTime())
	assert.Equal(t, int64(62), timeEstMng.DataEstimatedRemainingTime)
	// Build info estimation should be lowered because two build info files were transferred.
	assert.Equal(t, int64(40), timeEstMng.getBuildInfoEstimatedRemainingTime())
	assertGetEstimatedRemainingTime(t, timeEstMng, int64(102))
}

func assertGetEstimatedRemainingTime(t *testing.T, timeEstMng *TimeEstimationManager, expected int64) {
	estimatedRemainingTime, err := timeEstMng.getEstimatedRemainingTime()
	assert.NoError(t, err)
	assert.Equal(t, expected, estimatedRemainingTime)
}

func TestGetWorkingThreadsForBuildInfoEstimation(t *testing.T) {
	timeEstMng, cleanUp := initTimeEstimationBITest(t)
	defer cleanUp()

	setWorkingThreadsAndAssertBiThreads(t, timeEstMng, 0, 1)
	setWorkingThreadsAndAssertBiThreads(t, timeEstMng, 1, 1)
	setWorkingThreadsAndAssertBiThreads(t, timeEstMng, 8, 8)
	setWorkingThreadsAndAssertBiThreads(t, timeEstMng, 9, 8)
}

func setWorkingThreadsAndAssertBiThreads(t *testing.T, timeEstMng *TimeEstimationManager, threads, expectedBiThreads int) {
	timeEstMng.stateManager.WorkingThreads = threads
	actualBiThreads, err := timeEstMng.getWorkingThreadsForBuildInfoEstimation()
	assert.NoError(t, err)
	assert.Equal(t, expectedBiThreads, actualBiThreads)
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
	addChunkStatus(t, timeEstMng, chunkStatus1, 3, true, 10*milliSecsInSecond)
	assert.Equal(t, "Not available yet", timeEstMng.GetSpeedString())
	assert.Equal(t, "Not available yet", timeEstMng.GetEstimatedRemainingTimeString())
}

func TestEstimationNotAvailable(t *testing.T) {
	timeEstMng, cleanUp := initTimeEstimationDataTest(t)
	defer cleanUp()

	// Assert unavailable if on unsupported phase.
	timeEstMng.stateManager.CurrentRepoPhase = api.Phase2
	assert.Equal(t, "Not available in this phase", timeEstMng.GetEstimatedRemainingTimeString())

	// After made available, assert not available until LastSpeeds are set.
	timeEstMng.stateManager.CurrentRepoPhase = api.Phase3
	assert.Equal(t, "Not available yet", timeEstMng.GetEstimatedRemainingTimeString())

	timeEstMng.LastSpeeds = []float64{1.23}
	assert.Equal(t, "Less than a minute", timeEstMng.GetEstimatedRemainingTimeString())
}

func TestSpeedUnavailableForBuildInfoRepo(t *testing.T) {
	timeEstMng, cleanUp := initTimeEstimationDataTest(t)
	defer cleanUp()

	assert.NoError(t, timeEstMng.stateManager.SetRepoState(repo3Key, 0, 0, true, true))
	assert.Equal(t, "Not available while transferring a build-info repository", timeEstMng.GetSpeedString())
}

func newDefaultTimeEstimationManager(t *testing.T, buildInfoRepos bool) *TimeEstimationManager {
	stateManager, err := NewTransferStateManager(true)
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
