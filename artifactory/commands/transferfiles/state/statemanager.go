package state

import (
	"time"

	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/lock"
)

const saveIntervalSecs = 10

type ProgressState struct {
	TotalSizeBytes       int64 `json:"total_size_bytes,omitempty"`
	TransferredSizeBytes int64 `json:"transferred_size_bytes,omitempty"`
	TotalUnits           int   `json:"total_units,omitempty"`
	TransferredUnits     int   `json:"transferred_units,omitempty"`
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

	return &stateManager, nil
}

func (ts *TransferStateManager) LockTransferStateManager() error {
	return ts.lockStateManager()
}

func (ts *TransferStateManager) UnlockTransferStateManager() error {
	return ts.unlockStateManager()
}

// Delete the transferred info from the repository and set the total info.
// repoKey        - Repository key
// totalSizeBytes - Repository size in bytes
// totalFiles     - Total files in the repository
func (ts *TransferStateManager) ResetRepoState(repoKey string, totalSizeBytes int64, totalFiles int) error {
	return ts.SetRepoState(repoKey, totalSizeBytes, totalFiles, true)
}

// Set the repository state.
// repoKey        - Repository key
// totalSizeBytes - Repository size in bytes
// totalFiles     - Total files in the repository
// reset          - Delete the transferred info
func (ts *TransferStateManager) SetRepoState(repoKey string, totalSizeBytes int64, totalFiles int, reset bool) error {
	return ts.TransferState.action(func(state *TransferState) error {
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
}

func (ts *TransferStateManager) SetRepoFullTransferStarted(repoKey string, startTime time.Time) error {
	// Set the start time in the Transfer State
	err := ts.TransferState.action(func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		repo.FullTransfer.Started = ConvertTimeToRFC3339(startTime)
		return nil
	})
	if err != nil {
		return err
	}
	// Set the current repository key in the Run Status
	return ts.TransferRunStatus.action(func(state *TransferRunStatus) error {
		state.CurrentRepo = repoKey
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

func (ts *TransferStateManager) IncTransferredSizeAndFiles(repoKey string, totalFiles int, totalSizeInBytes int64) error {
	err := ts.TransferState.action(func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		repo.TransferredSizeBytes += totalSizeInBytes
		repo.TransferredUnits += totalFiles
		return nil
	})
	if err != nil {
		return err
	}
	return ts.TransferRunStatus.action(func(transferRunStatus *TransferRunStatus) error {
		transferRunStatus.TransferredSizeBytes += totalSizeInBytes
		return nil
	})
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
		transferredSizeBytes = transferRunStatus.TransferredSizeBytes
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

func (ts *TransferStateManager) IncRepositoriesTransferred() error {
	return ts.TransferRunStatus.action(func(transferRunStatus *TransferRunStatus) error {
		transferRunStatus.TransferredUnits++
		return nil
	})
}

func (ts *TransferStateManager) SetRepoPhase(phaseId int) error {
	return ts.TransferRunStatus.action(func(transferRunStatus *TransferRunStatus) error {
		transferRunStatus.CurrentRepoPhase = phaseId
		return nil
	})
}

func (ts *TransferStateManager) SaveState() error {
	ts.TransferRunStatus.lastSaveTimestamp = time.Now()
	return ts.persistTransferState()
}

// Lock the state manager. We currently use this lock only to determine whether a transfer is in process.
func (ts *TransferStateManager) lockStateManager() error {
	lockDirPath, err := coreutils.GetJfrogTransferLockDir()
	if err != nil {
		return err
	}
	unlockFunc, err := lock.CreateLock(lockDirPath)
	if err != nil {
		return err
	}
	ts.unlockStateManager = unlockFunc
	return nil
}

func GetStartTimestamp() (int64, error) {
	lockDirPath, err := coreutils.GetJfrogTransferLockDir()
	if err != nil {
		return 0, err
	}
	return lock.GetLastLockTimestamp(lockDirPath)
}
