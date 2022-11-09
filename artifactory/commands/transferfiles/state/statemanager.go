package state

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"time"

	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/lock"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

// The interval in which to save the state and run transfer files to the file system.
// Every change made will be held in memory till the saving time comes.
const saveIntervalSecsDefault = 10

var SaveIntervalSecs = saveIntervalSecsDefault

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
	// This function unlocks the state manager after the transfer-files command is finished
	unlockStateManager func() error
}

func NewTransferStateManager(loadRunStatus bool) (*TransferStateManager, error) {
	transferState, err := loadTransferState()
	if err != nil {
		return nil, err
	}
	stateManager := TransferStateManager{
		TransferState: *transferState,
	}
	if loadRunStatus {
		transferRunStatus, err := loadTransferRunStatus()
		if err != nil {
			return nil, err
		}
		stateManager.TransferRunStatus = *transferRunStatus
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
// reset          - Delete the transferred info
func (ts *TransferStateManager) SetRepoState(repoKey string, totalSizeBytes, totalFiles int64, buildInfoRepo, reset bool) error {
	err := ts.TransferState.action(func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, true)
		if err != nil || repo == nil {
			return err
		}
		if reset {
			*repo = newRepositoryState(repoKey)
		}
		repo.TotalSizeBytes = totalSizeBytes
		repo.TotalUnits = totalFiles
		return nil
	})
	if err != nil {
		return err
	}
	return ts.TransferRunStatus.action(func(transferRunStatus *TransferRunStatus) error {
		transferRunStatus.CurrentRepo = repoKey
		transferRunStatus.BuildInfoRepo = buildInfoRepo
		return nil
	})
}

func (ts *TransferStateManager) SetRepoFullTransferStarted(repoKey string, startTime time.Time) error {
	// Set the start time in the Transfer State
	return ts.TransferState.action(func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		repo.FullTransfer.Started = ConvertTimeToRFC3339(startTime)
		return nil
	})
}

func (ts *TransferStateManager) SetRepoFullTransferCompleted(repoKey string) error {
	return ts.TransferState.action(func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		repo.FullTransfer.Ended = ConvertTimeToRFC3339(time.Now())
		return nil
	})
}

func (ts *TransferStateManager) IncTransferredSizeAndFiles(repoKey string, chunkTotalFiles, chunkTotalSizeInBytes int64) error {
	err := ts.TransferState.action(func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		repo.TransferredSizeBytes += chunkTotalSizeInBytes
		repo.TransferredUnits += chunkTotalFiles
		return nil
	})
	if err != nil {
		return err
	}
	return ts.TransferRunStatus.action(func(transferRunStatus *TransferRunStatus) error {
		transferRunStatus.TransferOverall.TransferredSizeBytes += chunkTotalSizeInBytes
		transferRunStatus.TransferOverall.TransferredUnits += chunkTotalFiles
		if transferRunStatus.BuildInfoRepo {
			transferRunStatus.OverallBiFiles.TransferredUnits += chunkTotalFiles
		}
		return nil
	})
}

func (ts *TransferStateManager) IncTransferredSizeAndFilesDiff(repoKey string, chunkTotalFiles, chunkTotalSizeInBytes int64) error {
	return ts.TransferState.action(func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		repo.DiffInfo.TransferredSizeBytes += chunkTotalSizeInBytes
		repo.DiffInfo.TransferredUnits += chunkTotalFiles
		return nil
	})
}

func (ts *TransferStateManager) IncTotalSizeAndFilesDiff(repoKey string, filesNumber, totalSize int64) error {
	return ts.TransferState.action(func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		repo.DiffInfo.TotalSizeBytes += totalSize
		repo.DiffInfo.TotalUnits += filesNumber
		return nil
	})
}

func (ts *TransferStateManager) GetStorageAndFilesPointers(repoKey string) (totalStorage, totalUploadedStorage, totalFiles, totalUploadedFiles *int64, err error) {
	err = ts.TransferState.action(func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {

			return err
		}
		totalStorage = &repo.TotalSizeBytes
		totalUploadedStorage = &repo.TransferredSizeBytes
		totalFiles = &repo.TotalUnits
		totalUploadedFiles = &repo.TransferredUnits
		return nil
	})
	return
}
func (ts *TransferStateManager) GetStorageAndFilesPointersForDiff(repoKey string) (totalDiffStorage, totalUploadedDiffStorage, totalDiffFiles, totalUploadedDiffFiles *int64, err error) {
	err = ts.TransferState.action(func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		totalDiffStorage = &repo.DiffInfo.TotalSizeBytes
		totalUploadedDiffStorage = &repo.DiffInfo.TransferredSizeBytes
		totalDiffFiles = &repo.DiffInfo.TotalUnits
		totalUploadedDiffFiles = &repo.DiffInfo.TransferredUnits
		return nil
	})
	return
}

// Adds new diff details to the repo's diff array in state.
// Marks files handling as started, and sets the handling range.
func (ts *TransferStateManager) AddNewDiffToState(repoKey string, startTime time.Time) error {
	return ts.TransferState.action(func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		newDiff := DiffDetails{}

		// Set Files Diff Handling started.
		newDiff.FilesDiffRunTime.Started = ConvertTimeToRFC3339(startTime)

		// Determines the range on which files diffs should be handled.
		// If the repo previously completed files diff phases, we will continue handling diffs from where the last phase finished handling.
		// Otherwise, we will start handling diffs from the start time of the full transfer.
		for i := len(repo.Diffs) - 1; i >= 0; i-- {
			if repo.Diffs[i].Completed {
				newDiff.HandledRange.Started = repo.Diffs[i].HandledRange.Ended
				break
			}
		}
		if newDiff.HandledRange.Started == "" {
			newDiff.HandledRange.Started = repo.FullTransfer.Started
		}
		newDiff.HandledRange.Ended = ConvertTimeToRFC3339(startTime)
		repo.Diffs = append(repo.Diffs, newDiff)
		return nil
	})
}

func (ts *TransferStateManager) SetFilesDiffHandlingCompleted(repoKey string) error {
	return ts.TransferState.action(func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		repo.Diffs[len(repo.Diffs)-1].FilesDiffRunTime.Ended = ConvertTimeToRFC3339(time.Now())
		repo.Diffs[len(repo.Diffs)-1].Completed = true
		return nil
	})
}

func (ts *TransferStateManager) GetReposTransferredSizeBytes(repoKeys ...string) (transferredSizeBytes int64, err error) {
	return transferredSizeBytes, ts.TransferState.action(func(state *TransferState) error {
		reposSet := datastructures.MakeSet[string]()
		for _, repoKey := range repoKeys {
			reposSet.Add(repoKey)
		}
		for i := range state.Repositories {
			if reposSet.Exists(state.Repositories[i].Name) {
				transferredSizeBytes += state.Repositories[i].TransferredSizeBytes
			}
		}
		return nil
	})
}

func (ts *TransferStateManager) GetTransferredSizeBytes() (transferredSizeBytes int64, err error) {
	return transferredSizeBytes, ts.TransferRunStatus.action(func(transferRunStatus *TransferRunStatus) error {
		transferredSizeBytes = transferRunStatus.TransferOverall.TransferredSizeBytes
		return nil
	})
}

func (ts *TransferStateManager) GetDiffHandlingRange(repoKey string) (start, end time.Time, err error) {
	return start, end, ts.TransferState.action(func(state *TransferState) error {
		repo, inErr := state.getRepository(repoKey, false)
		if inErr != nil {
			return inErr
		}
		start, inErr = ConvertRFC3339ToTime(repo.Diffs[len(repo.Diffs)-1].HandledRange.Started)
		if inErr != nil {
			return inErr
		}
		end, inErr = ConvertRFC3339ToTime(repo.Diffs[len(repo.Diffs)-1].HandledRange.Ended)
		return inErr
	})
}

func (ts *TransferStateManager) IsRepoTransferred(repoKey string) (isTransferred bool, err error) {
	return isTransferred, ts.TransferState.action(func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, true)
		if err != nil {
			return err
		}
		isTransferred = repo.FullTransfer.Ended != ""
		return nil
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

func (ts *TransferStateManager) SaveState() error {
	ts.TransferRunStatus.lastSaveTimestamp = time.Now()
	return ts.persistTransferState()
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
	return secondsToLiteralTime(runningSecs, ""), true, nil
}

func UpdateChunkInState(stateManager *TransferStateManager, repoKey string, chunk *api.ChunkStatus) (chunkTotalSizeInBytes int64, err error) {
	chunkTotalSizeInBytes = 0
	var chunkTotalFiles int64 = 0
	for _, file := range chunk.Files {
		if file.Status == api.Success {
			chunkTotalSizeInBytes += file.SizeBytes
			chunkTotalFiles++
		}
	}
	if stateManager.CurrentRepoPhase == api.FilesDiffPhase {
		err := stateManager.IncTransferredSizeAndFilesDiff(repoKey, chunkTotalFiles, chunkTotalSizeInBytes)
		if err != nil {
			return 0, err
		}
	}
	return chunkTotalSizeInBytes, stateManager.IncTransferredSizeAndFiles(repoKey, chunkTotalFiles, chunkTotalSizeInBytes)
}
