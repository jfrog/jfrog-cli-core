package transferfiles

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"io/ioutil"
	"strconv"
	"time"
)

const requestsNumForNodeDetection = 50

// This struct holds the current state of the whole transfer of the source Artifactory instance.
// It is saved to a file in JFrog CLI's home.
// The command determines actions based on the state, such as if full transfer need or was completed before,
// on what time range files diffs should be fixed, etc.
type TransferState struct {
	Repositories []Repository `json:"repositories,omitempty"`
	NodeIds      []string     `json:"nodes,omitempty"`
}

type Repository struct {
	Name         string            `json:"name,omitempty"`
	FullTransfer PhaseDetails      `json:"full_transfer,omitempty"`
	Diffs        []FullDiffDetails `json:"diffs,omitempty"`
}

type PhaseDetails struct {
	Started string `json:"started,omitempty"`
	Ended   string `json:"ended,omitempty"`
}

type FullDiffDetails struct {
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

// Checks whether this is the first run of the command, based on information saved in the command's state.
// Currently, only needed for the properties phase.
func isCleanStart() (bool, error) {
	state, err := getTransferState()
	if err != nil {
		return false, err
	}
	return len(state.NodeIds) == 0, nil
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
		return nil, errorutils.CheckErrorf("Could not find repository '" + repoKey + "' in state file. Aborting.")
	}
	repo := Repository{Name: repoKey}
	ts.Repositories = append(ts.Repositories, repo)
	return &repo, nil
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

func addNewDiffToState(repoKey string, startTime time.Time) error {
	action := func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		newDiff := FullDiffDetails{}

		// Determines the range on which files diffs should be fixed.
		// The beginning of the range should be the start time of the last phase completed for the same repo.
		// For example, if the repo was just fully migrated, the start of the range would be the start time of the migration, so that we will handle files
		// that may have been created / modified during the transfer.
		// If the repo has already previously completed a files diff phase, we will take the start time of that phase.
		for i := len(repo.Diffs) - 1; i >= 0; i-- {
			if repo.Diffs[i].Completed {
				newDiff.HandledRange.Started = repo.Diffs[i].HandledRange.Started
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

func setFilesDiffHandlingStarted(repoKey string, startTime time.Time) error {
	action := func(state *TransferState) error {
		repo, err := state.getRepository(repoKey, false)
		if err != nil {
			return err
		}
		repo.Diffs[len(repo.Diffs)-1].FilesDiffRunTime.Started = convertTimeToRFC3339(startTime)
		return nil
	}
	return doAndSaveState(action)
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

func convertTimeToRFC3339(timeToConvert time.Time) string {
	return timeToConvert.Format(time.RFC3339)
}

func convertRFC3339ToTime(timeToConvert string) (time.Time, error) {
	return time.Parse(time.RFC3339, timeToConvert)
}

func convertTimeToEpochMilliseconds(timeToConvert time.Time) string {
	return strconv.FormatInt(timeToConvert.UnixMilli(), 10)
}

// Sends rapid requests to the user plugin and finds all existing nodes in Artifactory.
// Writes all node ids to the state file.
// Also notifies all nodes of a clean start, i.e. older caches (if exists) are not needed, and the nodes should start
// tracking properties modifications (if phase 3 is enabled).
// Nodes are expected not to change during the whole transfer process.
func nodeDetection(srcUpService *srcUserPluginService) error {
	var nodeIds []string
requestsLoop:
	for i := 0; i < requestsNumForNodeDetection; i++ {
		curNodeId, err := srcUpService.ping()
		if err != nil {
			return err
		}
		for _, existingNode := range nodeIds {
			if curNodeId == existingNode {
				continue requestsLoop
			}
		}
		nodeIds = append(nodeIds, curNodeId)
	}

	return saveTransferState(&TransferState{NodeIds: nodeIds})
}

func getNodesList() ([]string, error) {
	state, err := getTransferState()
	if err != nil {
		return nil, err
	}

	return state.NodeIds, nil
}
