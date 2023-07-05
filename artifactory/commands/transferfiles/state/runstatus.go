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

// The current version of the run-status.json file.
// Can be used to identify when the version of the CLI doesn't support the structure of the transfer directory.
const transferRunStatusVersion = 1

var saveRunStatusMutex sync.Mutex

type ActionOnStatusFunc func(transferRunStatus *TransferRunStatus) error

// This struct holds the run status of the current transfer.
// It is saved to a file in JFrog CLI's home, but gets reset every time the transfer begins.
// This state is used to allow showing the current run status by the 'jf rt transfer-files --status' command.
// It is also used for the time estimation and more.
type TransferRunStatus struct {
	lastSaveTimestamp time.Time
	// This variable holds the total/transferred number of repositories (not their files).
	OverallTransfer   ProgressState      `json:"overall_transfer,omitempty"`
	TotalRepositories ProgressStateUnits `json:"total_repositories,omitempty"`
	OverallBiFiles    ProgressStateUnits `json:"overall_bi_files,omitempty"`
	// Version of the TransferRunStatus file.
	Version        int    `json:"version,omitempty"`
	CurrentRepoKey string `json:"current_repo,omitempty"`
	// True if currently transferring a build info repository.
	BuildInfoRepo         bool `json:"build_info_repo,omitempty"`
	CurrentRepoPhase      int  `json:"current_repo_phase,omitempty"`
	WorkingThreads        int  `json:"working_threads,omitempty"`
	TransferFailures      uint `json:"transfer_failures,omitempty"`
	TimeEstimationManager `json:"time_estimation,omitempty"`
	StaleChunks           []StaleChunks `json:"stale_chunks,omitempty"`
}

// This structure contains a collection of chunks that have been undergoing processing for over 30 minutes
type StaleChunks struct {
	NodeID string       `json:"node_id,omitempty"`
	Chunks []StaleChunk `json:"stale_node_chunks,omitempty"`
}

type StaleChunk struct {
	ChunkID string   `json:"chunk_id,omitempty"`
	Files   []string `json:"files,omitempty"`
	Sent    int64    `json:"sent,omitempty"`
}

func (ts *TransferRunStatus) action(action ActionOnStatusFunc) error {
	if err := action(ts); err != nil {
		return err
	}

	now := time.Now()
	if now.Sub(ts.lastSaveTimestamp).Seconds() < float64(stateAndStatusSaveIntervalSecs) {
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

func loadTransferRunStatus() (transferRunStatus TransferRunStatus, exists bool, err error) {
	statusFilePath, err := coreutils.GetJfrogTransferRunStatusFilePath()
	if err != nil {
		return
	}
	exists, err = fileutils.IsFileExists(statusFilePath, false)
	if err != nil || !exists {
		return
	}

	content, err := fileutils.ReadFile(statusFilePath)
	if err != nil {
		return
	}

	err = errorutils.CheckError(json.Unmarshal(content, &transferRunStatus))
	return
}

func VerifyTransferRunStatusVersion() error {
	transferRunStatus, exists, err := loadTransferRunStatus()
	if err != nil || !exists {
		return err
	}
	if transferRunStatus.Version != transferRunStatusVersion {
		return GetOldTransferDirectoryStructureError()
	}
	return nil
}
