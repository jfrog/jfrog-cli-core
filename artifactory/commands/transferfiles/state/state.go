package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
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
	Phase1Info   ProgressState `json:"Phase1_info,omitempty"`
	Phase2Info   ProgressState `json:"Phase2_info,omitempty"`
	Phase3Info   ProgressState `json:"Phase3_info,omitempty"`
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

func (ts *TransferState) action(action ActionOnStateFunc) error {
	if err := action(ts); err != nil {
		return err
	}

	now := time.Now()
	if now.Sub(ts.lastSaveTimestamp).Seconds() < float64(SaveIntervalSecs) {
		return nil
	}

	if !saveStateMutex.TryLock() {
		return nil
	}
	defer saveStateMutex.Unlock()

	ts.lastSaveTimestamp = now
	return ts.persistTransferState()
}

func (ts *TransferState) persistTransferState() (err error) {
	repoDir, err := GetRepositoryTransferDir(ts.CurrentRepo.Name)
	if err != nil {
		return err
	}

	repoStateFilePath := filepath.Join(repoDir, coreutils.JfrogTransferRepoStateFileName)

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

func LoadTransferState(repoKey string) (transferState TransferState, exists bool, err error) {
	stateFilePath, err := getRepoStateFilepath(repoKey)
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
	if transferState.Version > transferStateFileVersion {
		return TransferState{}, false, errorutils.CheckErrorf(transferStateFileInvalidVersionErrorMsg)
	}
	return
}

func getRepoStateFilepath(repoKey string) (string, error) {
	repoDir, err := GetRepositoryTransferDir(repoKey)
	if err != nil {
		return "", err
	}
	return filepath.Join(repoDir, coreutils.JfrogTransferRepoStateFileName), nil
}
