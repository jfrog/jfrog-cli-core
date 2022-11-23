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

var saveStateMutex sync.Mutex

type ActionOnStateFunc func(state *TransferState) error

// This struct holds the current state of the whole transfer of the source Artifactory instance.
// It is saved to a file in JFrog CLI's home.
// The transfer-files command uses this state to determine which phases need to be executed for each a repository,
// as well as other decisions related to the process execution.
type TransferState struct {
	lastSaveTimestamp time.Time
	Repositories      []Repository `json:"repositories,omitempty"`
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

func NewTransferState() *TransferState {
	return &TransferState{}
}

func (ts *TransferState) getRepository(repoKey string, createIfMissing bool) (*Repository, error) {
	for i := range ts.Repositories {
		if ts.Repositories[i].Name == repoKey {
			return &ts.Repositories[i], nil
		}
	}
	if !createIfMissing {
		return nil, errorutils.CheckErrorf(getRepoMissingErrorMsg(repoKey))
	}
	repo := newRepositoryState(repoKey)
	ts.Repositories = append(ts.Repositories, repo)
	return &ts.Repositories[len(ts.Repositories)-1], nil
}

func newRepositoryState(repoKey string) Repository {
	return Repository{Name: repoKey}
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
	stateFilePath, err := coreutils.GetJfrogTransferStateFilePath()
	if err != nil {
		return err
	}

	content, err := json.Marshal(ts)
	if err != nil {
		return errorutils.CheckError(err)
	}

	err = os.WriteFile(stateFilePath, content, 0600)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return nil
}

func loadTransferState() (*TransferState, error) {
	stateFilePath, err := coreutils.GetJfrogTransferStateFilePath()
	if err != nil {
		return nil, err
	}
	exists, err := fileutils.IsFileExists(stateFilePath, false)
	if err != nil {
		return nil, err
	}
	transferState := &TransferState{}
	if !exists {
		return transferState, nil
	}

	content, err := fileutils.ReadFile(stateFilePath)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(content, &transferState); err != nil {
		return nil, errorutils.CheckError(err)
	}
	return transferState, nil
}
