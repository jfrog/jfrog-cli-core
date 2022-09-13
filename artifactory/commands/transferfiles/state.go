package transferfiles

import (
	"encoding/json"
	"github.com/jfrog/gofrog/datastructures"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

// This struct holds the current state of the whole transfer of the source Artifactory instance.
// It is saved to a file in JFrog CLI's home.
// The command determines actions based on the state, such as if full transfer need or was completed before,
// on what time range files diffs should be fixed, etc.
type TransferState struct {
	Repositories []Repository `json:"repositories,omitempty"`
}

type Repository struct {
	Name                 string        `json:"name,omitempty"`
	FullTransfer         PhaseDetails  `json:"full_transfer,omitempty"`
	Diffs                []DiffDetails `json:"diffs,omitempty"`
	TransferredSizeBytes int64         `json:"transferred_size_bytes,omitempty"`
}

type PhaseDetails struct {
	Started string `json:"started,omitempty"`
	Ended   string `json:"ended,omitempty"`
}

type DiffDetails struct {
	FilesDiffRunTime      PhaseDetails `json:"files_diff,omitempty"`
	PropertiesDiffRunTime PhaseDetails `json:"properties_diff,omitempty"`
	HandledRange          PhaseDetails `json:"handled_range,omitempty"`
	Completed             bool         `json:"completed,omitempty"`
}

type actionOnStateFunc func(state *TransferState) error

func getTransferState() (*TransferState, error) {
	stateFilePath, err := coreutils.GetJfrogTransferStateFilePath()
	if err != nil {
		return nil, err
	}
	exists, err := fileutils.IsFileExists(stateFilePath, false)
	if err != nil {
		return nil, err
	}
	if !exists {
		return &TransferState{}, nil
	}

	content, err := fileutils.ReadFile(stateFilePath)
	if err != nil {
		return nil, err
	}

	state := new(TransferState)
	err = json.Unmarshal(content, state)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return state, nil
}

func saveTransferState(state *TransferState) error {
	stateFilePath, err := coreutils.GetJfrogTransferStateFilePath()
	if err != nil {
		return err
	}

	content, err := json.Marshal(state)
	if err != nil {
		return errorutils.CheckError(err)
	}

	err = ioutil.WriteFile(stateFilePath, content, 0600)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return nil
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
	return &repo, nil
}

func getRepoMissingErrorMsg(repoKey string) string {
	return "Could not find repository '" + repoKey + "' in state file. Aborting."
}

func doAndSaveState(action actionOnStateFunc) error {
	state, err := getTransferState()
	if err != nil {
		return err
	}

	err = action(state)
	if err != nil {
		return err
	}

	return saveTransferState(state)
}

func setRepoFullTransferStarted(repoKey string, startTime time.Time) error {
	action := func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		repo.FullTransfer.Started = convertTimeToRFC3339(startTime)
		return nil
	}
	return doAndSaveState(action)
}

func setRepoFullTransferCompleted(repoKey string) error {
	action := func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		repo.FullTransfer.Ended = convertTimeToRFC3339(time.Now())
		return nil
	}
	return doAndSaveState(action)
}

func incRepoTransferredSizeBytes(repoKey string, sizeToAdd int64) error {
	action := func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		repo.TransferredSizeBytes += sizeToAdd
		return nil
	}
	return doAndSaveState(action)
}

func getReposTransferredSizeBytes(repoKeys ...string) (transferredSizeBytes int64, err error) {
	reposSet := datastructures.MakeSet[string]()
	for _, repoKey := range repoKeys {
		reposSet.Add(repoKey)
	}
	action := func(state *TransferState) error {
		for i := range state.Repositories {
			if reposSet.Exists(state.Repositories[i].Name) {
				transferredSizeBytes += state.Repositories[i].TransferredSizeBytes
			}
		}
		return nil
	}
	err = doAndSaveState(action)
	return
}

// Adds new diff details to the repo's diff array in state.
// Marks files handling as started, and sets the handling range.
func addNewDiffToState(repoKey string, startTime time.Time) error {
	action := func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		newDiff := DiffDetails{}

		// Set Files Diff Handling started.
		newDiff.FilesDiffRunTime.Started = convertTimeToRFC3339(startTime)

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
		newDiff.HandledRange.Ended = convertTimeToRFC3339(startTime)
		repo.Diffs = append(repo.Diffs, newDiff)
		return nil
	}
	return doAndSaveState(action)
}

func getDiffHandlingRange(repoKey string) (start, end time.Time, err error) {
	action := func(state *TransferState) error {
		repo, inErr := state.getRepository(repoKey, false)
		if inErr != nil {
			return inErr
		}
		start, inErr = convertRFC3339ToTime(repo.Diffs[len(repo.Diffs)-1].HandledRange.Started)
		if inErr != nil {
			return inErr
		}
		end, inErr = convertRFC3339ToTime(repo.Diffs[len(repo.Diffs)-1].HandledRange.Ended)
		if inErr != nil {
			return inErr
		}
		return nil
	}
	err = doAndSaveState(action)
	return
}

func setFilesDiffHandlingCompleted(repoKey string) error {
	action := func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		repo.Diffs[len(repo.Diffs)-1].FilesDiffRunTime.Ended = convertTimeToRFC3339(time.Now())
		repo.Diffs[len(repo.Diffs)-1].Completed = isPropertiesPhaseDisabled()
		return nil
	}
	return doAndSaveState(action)
}

func setPropsDiffHandlingStarted(repoKey string, startTime time.Time) error {
	action := func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		repo.Diffs[len(repo.Diffs)-1].PropertiesDiffRunTime.Started = convertTimeToRFC3339(startTime)
		return nil
	}
	return doAndSaveState(action)
}

func setPropsDiffHandlingCompleted(repoKey string) error {
	action := func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		repo.Diffs[len(repo.Diffs)-1].PropertiesDiffRunTime.Ended = convertTimeToRFC3339(time.Now())
		repo.Diffs[len(repo.Diffs)-1].Completed = true
		return nil
	}
	return doAndSaveState(action)
}

func isRepoTransferred(repoKey string) (bool, error) {
	isTransferred := false
	action := func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, true)
		if err != nil {
			return err
		}
		isTransferred = repo.FullTransfer.Ended != ""
		return nil
	}
	err := doAndSaveState(action)
	return isTransferred, err
}

func resetRepoState(repoKey string) error {
	action := func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, true)
		if err != nil || repo == nil {
			return err
		}
		*repo = newRepositoryState(repoKey)
		return nil
	}
	return doAndSaveState(action)
}

func newRepositoryState(repoKey string) Repository {
	return Repository{Name: repoKey}
}

func convertTimeToRFC3339(timeToConvert time.Time) string {
	return timeToConvert.Format(time.RFC3339)
}

func convertRFC3339ToTime(timeToConvert string) (time.Time, error) {
	return time.Parse(time.RFC3339, timeToConvert)
}

func convertTimeToEpochMilliseconds(timeToConvert time.Time) string {
	return strconv.FormatInt(timeToConvert.UnixMilli(), 10)
}
