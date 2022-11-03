package state

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

const transferRunStatusVersion = 0

var saveRunStatusMutex sync.Mutex

type ActionOnStatusFunc func(transferRunStatus *TransferRunStatus) error

// This struct holds the run status of the current transfer.
// It is saved to a file in JFrog CLI's home, but gets reset every time the transfer begins.
// This state is used to allow showing the current run status by the 'jf rt tranfer-files --status' command.
// It is also used for the time estimation and more.
type TransferRunStatus struct {
	lastSaveTimestamp time.Time `json:"-"`
	ProgressState
	TotalFiles       int    `json:"total_files,omitempty"`
	TransferredFiles int    `json:"transferred_files,omitempty"`
	Version          int    `json:"version,omitempty"`
	CurrentRepo      string `json:"current_repo,omitempty"`
	CurrentRepoPhase int    `json:"current_repo_phase,omitempty"`
	WorkingThreads   int    `json:"working_threads,omitempty"`
}

func (ts *TransferRunStatus) action(action ActionOnStatusFunc) error {
	if err := action(ts); err != nil {
		return err
	}

	now := time.Now()
	if now.Sub(ts.lastSaveTimestamp).Seconds() < saveIntervalSecs {
		return nil
	}

	if !saveRunStatusMutex.TryLock() {
		return nil
	}
	defer saveRunStatusMutex.Unlock()

	ts.lastSaveTimestamp = now
	return ts.persistTransferRunStatus()
}

func (ts *TransferRunStatus) persistTransferRunStatus() (err error) {
	statusFilePath, err := coreutils.GetJfrogTransferRunStatusFilePath()
	if err != nil {
		return err
	}

	ts.Version = transferRunStatusVersion
	content, err := json.Marshal(ts)
	if err != nil {
		return errorutils.CheckError(err)
	}

	err = os.WriteFile(statusFilePath, content, 0600)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return nil
}

func loadTransferRunStatus() (*TransferRunStatus, error) {
	statusFilePath, err := coreutils.GetJfrogTransferRunStatusFilePath()
	if err != nil {
		return nil, err
	}
	exists, err := fileutils.IsFileExists(statusFilePath, false)
	if err != nil {
		return nil, err
	}
	transferRunStatus := &TransferRunStatus{}
	if !exists {
		return transferRunStatus, nil
	}

	content, err := fileutils.ReadFile(statusFilePath)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(content, &transferRunStatus); err != nil {
		return nil, errorutils.CheckError(err)
	}
	return transferRunStatus, nil
}
