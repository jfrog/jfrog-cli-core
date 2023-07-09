package state

import (
	"path/filepath"
	"time"

	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/lock"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

// The interval in which to save the state and run transfer files to the file system.
// Every change made will be held in memory till the saving time comes.
const stateAndStatusSaveIntervalSecsDefault = 10

var stateAndStatusSaveIntervalSecs = stateAndStatusSaveIntervalSecsDefault

type ProgressState struct {
	TotalSizeBytes       int64 `json:"total_size_bytes,omitempty"`
	TransferredSizeBytes int64 `json:"transferred_size_bytes,omitempty"`
	ProgressStateUnits
}

type ProgressStateUnits struct {
	TotalUnits       int64 `json:"total_units,omitempty"`
	TransferredUnits int64 `json:"transferred_units,omitempty"`
}

type TransferStateManager struct {
	TransferState
	TransferRunStatus
	repoTransferSnapshot *RepoTransferSnapshot
	// This function unlocks the state manager after the transfer-files command is finished
	unlockStateManager func() error
}

func NewTransferStateManager(loadRunStatus bool) (*TransferStateManager, error) {
	stateManager := TransferStateManager{}
	if loadRunStatus {
		transferRunStatus, _, err := loadTransferRunStatus()
		if err != nil {
			return nil, err
		}
		stateManager.TransferRunStatus = transferRunStatus
	}
	stateManager.TimeEstimationManager.stateManager = &stateManager
	return &stateManager, nil
}

// Try to lock the transfer state manager.
// If file-transfer is already running, return "Already locked" error.
func (ts *TransferStateManager) TryLockTransferStateManager() error {
	return ts.tryLockStateManager()
}

func (ts *TransferStateManager) UnlockTransferStateManager() error {
	return ts.unlockStateManager()
}

// Set the repository state.
// repoKey        - Repository key
// totalSizeBytes - Repository size in bytes
// totalFiles     - Total files in the repository
// buildInfoRepo  - True if build info repository
// reset          - Delete the repository's previous transfer info
func (ts *TransferStateManager) SetRepoState(repoKey string, totalSizeBytes, totalFiles int64, buildInfoRepo, reset bool) error {
	err := ts.TransferState.Action(func(state *TransferState) error {
		transferState, repoTransferSnapshot, err := getTransferStateAndSnapshot(repoKey, reset)
		if err != nil {
			return err
		}
		transferState.CurrentRepo.Phase1Info.TotalSizeBytes = totalSizeBytes
		transferState.CurrentRepo.Phase1Info.TotalUnits = totalFiles

		ts.TransferState = transferState
		ts.repoTransferSnapshot = repoTransferSnapshot
		return nil
	})
	if err != nil {
		return err
	}
	return ts.TransferRunStatus.action(func(transferRunStatus *TransferRunStatus) error {
		transferRunStatus.CurrentRepoKey = repoKey
		transferRunStatus.BuildInfoRepo = buildInfoRepo

		transferRunStatus.OverallTransfer.TransferredUnits += ts.CurrentRepo.Phase1Info.TransferredUnits
		transferRunStatus.OverallTransfer.TransferredSizeBytes += ts.CurrentRepo.Phase1Info.TransferredSizeBytes
		return nil
	})
}

func (ts *TransferStateManager) SetRepoFullTransferStarted(startTime time.Time) error {
	// We do not want to change the start time if it already exists, because it means we continue transferring from a snapshot.
	// Some dirs may not be searched again (if done exploring or completed), so handling their diffs from the original time is required.
	return ts.TransferState.Action(func(state *TransferState) error {
		if state.CurrentRepo.FullTransfer.Started == "" {
			state.CurrentRepo.FullTransfer.Started = ConvertTimeToRFC3339(startTime)
		}
		return nil
	})
}

func (ts *TransferStateManager) SetRepoFullTransferCompleted() error {
	return ts.TransferState.Action(func(state *TransferState) error {
		state.CurrentRepo.FullTransfer.Ended = ConvertTimeToRFC3339(time.Now())
		return nil
	})
}

// Increasing Transferred Diff files (modified files) and SizeByBytes value in suitable repository progress state
func (ts *TransferStateManager) IncTransferredSizeAndFilesPhase1(chunkTotalFiles, chunkTotalSizeInBytes int64) error {
	err := ts.TransferState.Action(func(state *TransferState) error {
		state.CurrentRepo.Phase1Info.TransferredSizeBytes += chunkTotalSizeInBytes
		state.CurrentRepo.Phase1Info.TransferredUnits += chunkTotalFiles
		return nil
	})
	if err != nil {
		return err
	}
	return ts.TransferRunStatus.action(func(transferRunStatus *TransferRunStatus) error {
		transferRunStatus.OverallTransfer.TransferredSizeBytes += chunkTotalSizeInBytes
		transferRunStatus.OverallTransfer.TransferredUnits += chunkTotalFiles
		if transferRunStatus.BuildInfoRepo {
			transferRunStatus.OverallBiFiles.TransferredUnits += chunkTotalFiles
		}
		return nil
	})
}

func (ts *TransferStateManager) IncTransferredSizeAndFilesPhase2(chunkTotalFiles, chunkTotalSizeInBytes int64) error {
	return ts.TransferState.Action(func(state *TransferState) error {
		state.CurrentRepo.Phase2Info.TransferredSizeBytes += chunkTotalSizeInBytes
		state.CurrentRepo.Phase2Info.TransferredUnits += chunkTotalFiles
		return nil
	})
}

func (ts *TransferStateManager) IncTotalSizeAndFilesPhase2(filesNumber, totalSize int64) error {
	return ts.TransferState.Action(func(state *TransferState) error {
		state.CurrentRepo.Phase2Info.TotalSizeBytes += totalSize
		state.CurrentRepo.Phase2Info.TotalUnits += filesNumber
		return nil
	})
}

// Set relevant information of files and storage we need to transfer in phase3
func (ts *TransferStateManager) SetTotalSizeAndFilesPhase3(filesNumber, totalSize int64) error {
	return ts.TransferState.Action(func(state *TransferState) error {
		state.CurrentRepo.Phase3Info.TotalSizeBytes = totalSize
		state.CurrentRepo.Phase3Info.TotalUnits = filesNumber
		return nil
	})
}

// Increase transferred storage and files in phase 3
func (ts *TransferStateManager) IncTransferredSizeAndFilesPhase3(chunkTotalFiles, chunkTotalSizeInBytes int64) error {
	return ts.TransferState.Action(func(state *TransferState) error {
		state.CurrentRepo.Phase3Info.TransferredSizeBytes += chunkTotalSizeInBytes
		state.CurrentRepo.Phase3Info.TransferredUnits += chunkTotalFiles
		return nil
	})
}

// Returns pointers to TotalStorage, TotalFiles, TransferredFiles and TransferredStorage from progressState of a specific Repository.
func (ts *TransferStateManager) GetStorageAndFilesRepoPointers(phase int) (totalFailedStorage, totalUploadedFailedStorage, totalFailedFiles, totalUploadedFailedFiles *int64, err error) {
	err = ts.TransferState.Action(func(state *TransferState) error {
		switch phase {
		case api.Phase1:
			totalFailedStorage = &ts.CurrentRepo.Phase1Info.TotalSizeBytes
			totalUploadedFailedStorage = &ts.CurrentRepo.Phase1Info.TransferredSizeBytes
			totalFailedFiles = &ts.CurrentRepo.Phase1Info.TotalUnits
			totalUploadedFailedFiles = &ts.CurrentRepo.Phase1Info.TransferredUnits
		case api.Phase2:
			totalFailedStorage = &ts.CurrentRepo.Phase2Info.TotalSizeBytes
			totalUploadedFailedStorage = &ts.CurrentRepo.Phase2Info.TransferredSizeBytes
			totalFailedFiles = &ts.CurrentRepo.Phase2Info.TotalUnits
			totalUploadedFailedFiles = &ts.CurrentRepo.Phase2Info.TransferredUnits
		case api.Phase3:
			totalFailedStorage = &ts.CurrentRepo.Phase3Info.TotalSizeBytes
			totalUploadedFailedStorage = &ts.CurrentRepo.Phase3Info.TransferredSizeBytes
			totalFailedFiles = &ts.CurrentRepo.Phase3Info.TotalUnits
			totalUploadedFailedFiles = &ts.CurrentRepo.Phase3Info.TransferredUnits
		}
		return nil
	})
	return
}

// Adds new diff details to the repo's diff array in state.
// Marks files handling as started, and sets the handling range.
func (ts *TransferStateManager) AddNewDiffToState(startTime time.Time) error {
	return ts.TransferState.Action(func(state *TransferState) error {

		newDiff := DiffDetails{}

		// Set Files Diff Handling started.
		newDiff.FilesDiffRunTime.Started = ConvertTimeToRFC3339(startTime)

		// Determines the range on which files diffs should be handled.
		// If the repo previously completed files diff phases, we will continue handling diffs from where the last phase finished handling.
		// Otherwise, we will start handling diffs from the start time of the full transfer.
		for i := len(state.CurrentRepo.Diffs) - 1; i >= 0; i-- {
			if state.CurrentRepo.Diffs[i].Completed {
				newDiff.HandledRange.Started = state.CurrentRepo.Diffs[i].HandledRange.Ended
				break
			}
		}
		if newDiff.HandledRange.Started == "" {
			newDiff.HandledRange.Started = state.CurrentRepo.FullTransfer.Started
		}
		newDiff.HandledRange.Ended = ConvertTimeToRFC3339(startTime)
		state.CurrentRepo.Diffs = append(state.CurrentRepo.Diffs, newDiff)
		return nil
	})
}

func (ts *TransferStateManager) SetFilesDiffHandlingCompleted() error {
	return ts.TransferState.Action(func(state *TransferState) error {
		state.CurrentRepo.Diffs[len(state.CurrentRepo.Diffs)-1].FilesDiffRunTime.Ended = ConvertTimeToRFC3339(time.Now())
		state.CurrentRepo.Diffs[len(state.CurrentRepo.Diffs)-1].Completed = true
		return nil
	})
}

func (ts *TransferStateManager) GetReposTransferredSizeBytes(repoKeys ...string) (transferredSizeBytes int64, err error) {
	reposSet := datastructures.MakeSet[string]()
	for _, repoKey := range repoKeys {
		if reposSet.Exists(repoKey) {
			continue
		}
		reposSet.Add(repoKey)
		transferState, exists, err := LoadTransferState(repoKey, false)
		if err != nil {
			return transferredSizeBytes, err
		}
		if !exists {
			continue
		}
		transferredSizeBytes += transferState.CurrentRepo.Phase1Info.TransferredSizeBytes
	}
	return
}

func (ts *TransferStateManager) GetTransferredSizeBytes() (transferredSizeBytes int64, err error) {
	return transferredSizeBytes, ts.TransferRunStatus.action(func(transferRunStatus *TransferRunStatus) error {
		transferredSizeBytes = transferRunStatus.OverallTransfer.TransferredSizeBytes
		return nil
	})
}

func (ts *TransferStateManager) GetDiffHandlingRange() (start, end time.Time, err error) {
	return start, end, ts.TransferState.Action(func(state *TransferState) error {
		var inErr error
		start, inErr = ConvertRFC3339ToTime(state.CurrentRepo.Diffs[len(state.CurrentRepo.Diffs)-1].HandledRange.Started)
		if inErr != nil {
			return inErr
		}
		end, inErr = ConvertRFC3339ToTime(state.CurrentRepo.Diffs[len(state.CurrentRepo.Diffs)-1].HandledRange.Ended)
		return inErr
	})
}

func (ts *TransferStateManager) ChangeTransferFailureCountBy(count uint, increase bool) error {
	return ts.TransferRunStatus.action(func(transferRunStatus *TransferRunStatus) error {
		if increase {
			transferRunStatus.TransferFailures += count
		} else {
			transferRunStatus.TransferFailures -= count
		}
		return nil
	})
}

func (ts *TransferStateManager) IncRepositoriesTransferred() error {
	return ts.TransferRunStatus.action(func(transferRunStatus *TransferRunStatus) error {
		transferRunStatus.TotalRepositories.TransferredUnits++
		return nil
	})
}

func (ts *TransferStateManager) SetRepoPhase(phaseId int) error {
	return ts.TransferRunStatus.action(func(transferRunStatus *TransferRunStatus) error {
		transferRunStatus.CurrentRepoPhase = phaseId
		return nil
	})
}

func (ts *TransferStateManager) SetWorkingThreads(workingThreads int) error {
	return ts.TransferRunStatus.action(func(transferRunStatus *TransferRunStatus) error {
		transferRunStatus.WorkingThreads = workingThreads
		return nil
	})
}

func (ts *TransferStateManager) GetWorkingThreads() (workingThreads int, err error) {
	return workingThreads, ts.TransferRunStatus.action(func(transferRunStatus *TransferRunStatus) error {
		workingThreads = transferRunStatus.WorkingThreads
		return nil
	})
}

func (ts *TransferStateManager) SetStaleChunks(staleChunks []StaleChunks) error {
	return ts.action(func(transferRunStatus *TransferRunStatus) error {
		transferRunStatus.StaleChunks = staleChunks
		return nil
	})
}

func (ts *TransferStateManager) GetStaleChunks() (staleChunks []StaleChunks, err error) {
	return staleChunks, ts.action(func(transferRunStatus *TransferRunStatus) error {
		staleChunks = transferRunStatus.StaleChunks
		return nil
	})
}

func (ts *TransferStateManager) SaveStateAndSnapshots() error {
	ts.TransferState.lastSaveTimestamp = time.Now()
	if err := ts.persistTransferState(false); err != nil {
		return err
	}
	// Save snapshots if needed.
	if ts.repoTransferSnapshot == nil {
		return nil
	}
	ts.repoTransferSnapshot.lastSaveTimestamp = time.Now()
	if err := ts.repoTransferSnapshot.snapshotManager.PersistRepoSnapshot(); err != nil {
		return err
	}
	return ts.persistTransferState(true)
}

type AlreadyLockedError struct{}

func (m *AlreadyLockedError) Error() string {
	return "Files transfer is already running"
}

// Lock the state manager. We use the File Lock to acquire two purposes:
// 1. Make sure that only one transfer-files process is running
// 2. Check whether there is an active transfer-file process when providing the --status flag. We also extract the start timestamp (See 'GetStartTimestamp').
func (ts *TransferStateManager) tryLockStateManager() error {
	lockDirPath, err := coreutils.GetJfrogTransferLockDir()
	if err != nil {
		return err
	}
	startTimestamp, err := lock.GetLastLockTimestamp(lockDirPath)
	if err != nil {
		return err
	}
	if startTimestamp != 0 {
		return errorutils.CheckError(new(AlreadyLockedError))
	}
	unlockFunc, err := lock.CreateLock(lockDirPath)
	if err != nil {
		return err
	}
	ts.unlockStateManager = unlockFunc
	return nil
}

func getStartTimestamp() (int64, error) {
	lockDirPath, err := coreutils.GetJfrogTransferLockDir()
	if err != nil {
		return 0, err
	}
	return lock.GetLastLockTimestamp(lockDirPath)
}

func GetRunningTime() (runningTime string, isRunning bool, err error) {
	startTimestamp, err := getStartTimestamp()
	if err != nil || startTimestamp == 0 {
		return
	}
	runningSecs := int64(time.Since(time.Unix(0, startTimestamp)).Seconds())
	return SecondsToLiteralTime(runningSecs, ""), true, nil
}

func UpdateChunkInState(stateManager *TransferStateManager, chunk *api.ChunkStatus) (err error) {
	var chunkTotalSizeInBytes int64 = 0
	var chunkTotalFiles int64 = 0
	for _, file := range chunk.Files {
		if file.Status != api.Fail && file.Name != "" {
			// Count only successfully transferred files
			chunkTotalSizeInBytes += file.SizeBytes
			chunkTotalFiles++
		}
	}
	switch stateManager.CurrentRepoPhase {
	case api.Phase1:
		err = stateManager.IncTransferredSizeAndFilesPhase1(chunkTotalFiles, chunkTotalSizeInBytes)
	case api.Phase2:
		err = stateManager.IncTransferredSizeAndFilesPhase2(chunkTotalFiles, chunkTotalSizeInBytes)
	case api.Phase3:
		err = stateManager.IncTransferredSizeAndFilesPhase3(chunkTotalFiles, chunkTotalSizeInBytes)
	}
	return err
}

func GetJfrogTransferRepoSnapshotDir(repoKey string) (string, error) {
	return GetJfrogTransferRepoSubDir(repoKey, coreutils.JfrogTransferSnapshotDirName)
}

func GetRepoSnapshotFilePath(repoKey string) (string, error) {
	snapshotDir, err := GetJfrogTransferRepoSnapshotDir(repoKey)
	if err != nil {
		return "", err
	}
	err = fileutils.CreateDirIfNotExist(snapshotDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(snapshotDir, coreutils.JfrogTransferRepoSnapshotFileName), nil
}
