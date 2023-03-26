package state

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var saveStateMutex sync.Mutex

type ActionOnStateFunc func(state *TransferState) error

const (
	transferStateFileVersion                = 0
	transferStateFileInvalidVersionErrorMsg = "unexpected repository state file found"
)

// This struct holds the state of the current repository being transferred from the source Artifactory instance.
// It is saved to a file located in a directory named with the sha of the repository, under the transfer directory.
// The transfer-files command uses this state to determine which phases should be executed for the repository,
// as well as other decisions related to the process execution.
type TransferState struct {
	lastSaveTimestamp time.Time
	// The Version of the state.json file of a repository.
	Version     int        `json:"state_version,omitempty"`
	CurrentRepo Repository `json:"repository,omitempty"`
}

type Repository struct {
	Phase1Info   ProgressState `json:"phase1_info,omitempty"`
	Phase2Info   ProgressState `json:"phase2_info,omitempty"`
	Phase3Info   ProgressState `json:"phase3_info,omitempty"`
	Name         string        `json:"name,omitempty"`
	FullTransfer PhaseDetails  `json:"full_transfer,omitempty"`
	Diffs        []DiffDetails `json:"diffs,omitempty"`
}

type PhaseDetails struct {
	Started string `json:"started,omitempty"`
	Ended   string `json:"ended,omitempty"`
}

type DiffDetails struct {
	// The start and end time of a complete transferring Files Diff phase
	FilesDiffRunTime PhaseDetails `json:"files_diff,omitempty"`
	// The start and end time of the last handled range
	HandledRange PhaseDetails `json:"handled_range,omitempty"`
	// If false, start the Diff phase from the start time of the full transfer
	Completed bool `json:"completed,omitempty"`
}

func newRepositoryTransferState(repoKey string) TransferState {
	return TransferState{
		Version:     transferStateFileVersion,
		CurrentRepo: Repository{Name: repoKey},
	}
}

func (ts *TransferState) Action(action ActionOnStateFunc) error {
	if err := action(ts); err != nil {
		return err
	}

	now := time.Now()
	if now.Sub(ts.lastSaveTimestamp).Seconds() < float64(stateAndStatusSaveIntervalSecs) {
		return nil
	}

	if !saveStateMutex.TryLock() {
		return nil
	}
	defer saveStateMutex.Unlock()

	ts.lastSaveTimestamp = now
	return ts.persistTransferState(false)
}

// Persist TransferState to file, at the repository dir. If snapshot requested, persist it to the repository's snapshot dir.
func (ts *TransferState) persistTransferState(snapshot bool) (err error) {
	repoStateFilePath, err := GetRepoStateFilepath(ts.CurrentRepo.Name, snapshot)
	if err != nil {
		return err
	}

	content, err := json.Marshal(ts)
	if err != nil {
		return errorutils.CheckError(err)
	}

	err = os.WriteFile(repoStateFilePath, content, 0600)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return nil
}

func (ts *TransferState) IsRepoTransferred() (isTransferred bool, err error) {
	return isTransferred, ts.Action(func(state *TransferState) error {
		isTransferred = state.CurrentRepo.FullTransfer.Ended != ""
		return nil
	})
}

func LoadTransferState(repoKey string, snapshot bool) (transferState TransferState, exists bool, err error) {
	stateFilePath, err := GetRepoStateFilepath(repoKey, snapshot)
	if err != nil {
		return
	}
	exists, err = fileutils.IsFileExists(stateFilePath, false)
	if err != nil || !exists {
		return
	}

	content, err := fileutils.ReadFile(stateFilePath)
	if err != nil {
		return
	}

	if err = json.Unmarshal(content, &transferState); errorutils.CheckError(err) != nil {
		return
	}
	if transferState.Version != transferStateFileVersion {
		return TransferState{}, false, errorutils.CheckErrorf(transferStateFileInvalidVersionErrorMsg)
	}
	return
}

func GetRepoStateFilepath(repoKey string, snapshot bool) (string, error) {
	var dirPath string
	var err error
	if snapshot {
		dirPath, err = GetJfrogTransferRepoSnapshotDir(repoKey)
	} else {
		dirPath, err = GetRepositoryTransferDir(repoKey)
	}
	if err != nil {
		return "", err
	}
	return filepath.Join(dirPath, coreutils.JfrogTransferRepoStateFileName), nil
}

// Returns a transfer state and repo transfer snapshot according to the state of the repository as found in the repository transfer directory.
// A repo transfer snapshot is only returned if running phase 1 is required.
// The state and snapshot will be loaded from snapshot dir if a previous run of phase 1 was interrupted, and reset was not required.
func getTransferStateAndSnapshot(repoKey string, reset bool) (transferState TransferState, repoTransferSnapshot *RepoTransferSnapshot, err error) {
	if reset {
		return getCleanStateAndSnapshot(repoKey)
	}

	// Check if repo state exists. If not, start clean.
	transferState, exists, err := LoadTransferState(repoKey, false)
	if err != nil {
		return
	}
	if !exists {
		return getCleanStateAndSnapshot(repoKey)
	}

	// If it exists and repo already fully completed phase 1, load current state.
	transferred, err := transferState.IsRepoTransferred()
	if err != nil || transferred {
		return transferState, nil, err
	}

	// Phase 1 was started and not completed. Try loading snapshots to continue from the same point.
	return loadRepoSnapshots(repoKey)
}

// Loads the state and repo snapshots from the repository's snapshot directory.
func loadRepoSnapshots(repoKey string) (transferState TransferState, repoTransferSnapshot *RepoTransferSnapshot, err error) {
	transferState, stateExists, err := LoadTransferState(repoKey, true)
	if err != nil {
		return
	}
	snapshotPath, err := GetRepoSnapshotFilePath(repoKey)
	if err != nil {
		return
	}
	repoTransferSnapshot, snapshotExists, err := loadRepoTransferSnapshot(repoKey, snapshotPath)
	if !stateExists || !snapshotExists {
		log.Info("An attempt to transfer repository '" + repoKey + "' was previously stopped but no snapshot was found to continue from. " +
			"Starting to transfer from scratch...")
		return getCleanStateAndSnapshot(repoKey)
	}
	return
}

func getCleanStateAndSnapshot(repoKey string) (transferState TransferState, repoTransferSnapshot *RepoTransferSnapshot, err error) {
	snapshotFilePath, err := GetRepoSnapshotFilePath(repoKey)
	if err != nil {
		return
	}
	return newRepositoryTransferState(repoKey), createRepoTransferSnapshot(repoKey, snapshotFilePath), nil
}
