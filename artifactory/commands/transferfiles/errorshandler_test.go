package transferfiles

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
)

var (
	testRetryableStatus = Fail
	testSkippedStatus   = SkippedLargeProps
	testRepoKey         = "repo"
)

func TestTransferErrorsMng(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	errorsNumber := 50
	// We reduce the maximum number of entities per file to test the creation of multiple errors files.
	originalMaxErrorsInFile := maxErrorsInFile
	maxErrorsInFile = 20
	defer func() { maxErrorsInFile = originalMaxErrorsInFile }()
	errorsChannelMng := createErrorsChannelMng()
	transferErrorsMng, err := newTransferErrorsToFile(testRepoKey, 0, state.ConvertTimeToEpochMilliseconds(time.Now()), &errorsChannelMng, nil, nil)
	assert.NoError(t, err)

	var writeWaitGroup sync.WaitGroup
	var readWaitGroup sync.WaitGroup

	// "Writing transfer's errors to files" mechanism returned error
	var writingErrorsErr error
	// Start reading from the errors channel, and write errors into the relevant files.
	readWaitGroup.Add(1)
	go func() {
		defer readWaitGroup.Done()
		writingErrorsErr = transferErrorsMng.start()
	}()

	// Add 'retryable errors' to the common errors channel.
	// These errors will be written into files located in the "retryable" directory under the Jfrog CLI home directory.
	addErrorsToChannel(&writeWaitGroup, errorsNumber, errorsChannelMng, Fail)
	// Add 'skipped errors' to the common errors channel.
	// These errors will be written into files located in the "skipped" directory under the Jfrog CLI home directory.
	addErrorsToChannel(&writeWaitGroup, errorsNumber, errorsChannelMng, SkippedLargeProps)

	writeWaitGroup.Wait()
	errorsChannelMng.close()
	readWaitGroup.Wait()
	assert.NoError(t, writingErrorsErr)
	expectedNumberOfFiles := int(math.Ceil(float64(errorsNumber) / float64(maxErrorsInFile)))
	validateErrorsFiles(t, expectedNumberOfFiles, errorsNumber, true)
	validateErrorsFiles(t, expectedNumberOfFiles, errorsNumber, false)

	retryEntityCount, err := getRetryErrorCount([]string{testRepoKey})
	assert.NoError(t, err)
	assert.Equal(t, errorsNumber, retryEntityCount)
}

func addErrorsToChannel(writeWaitGroup *sync.WaitGroup, errorsNumber int, errorsChannelMng ErrorsChannelMng, status ChunkFileStatusType) {
	writeWaitGroup.Add(1)
	go func() {
		defer writeWaitGroup.Done()
		for i := 0; i < errorsNumber; i++ {
			errorsChannelMng.add(FileUploadStatusResponse{FileRepresentation: FileRepresentation{Repo: testRepoKey, Path: "path", Name: fmt.Sprintf("name%d", i)}, Status: status, StatusCode: 404, Reason: "reason"})
		}
	}()
}

// Ensure that all retryable/skipped errors files have been created and that they contain the expected content
func validateErrorsFiles(t *testing.T, filesNum, errorsNum int, isRetryable bool) {
	errorsFiles, err := getErrorsFiles([]string{testRepoKey}, isRetryable)
	status := getStatusType(isRetryable)
	assert.NoError(t, err)
	assert.Lenf(t, errorsFiles, filesNum, "unexpected number of error files.")
	var entitiesNum int
	for i := 0; i < filesNum; i++ {
		entitiesNum += validateErrorsFileContent(t, errorsFiles[i], status)
	}
	assert.Equal(t, errorsNum, entitiesNum)
}

func getStatusType(isRetryable bool) ChunkFileStatusType {
	if isRetryable {
		return testRetryableStatus
	}
	return testSkippedStatus
}

// Check the number of errors, their status and their uniqueness by reading the file's content.
func validateErrorsFileContent(t *testing.T, path string, status ChunkFileStatusType) (entitiesNum int) {
	exists, err := fileutils.IsFileExists(path, false)
	assert.NoError(t, err)
	assert.True(t, exists, fmt.Sprintf("file: %s does not exist", path))

	var content []byte
	content, err = fileutils.ReadFile(path)
	assert.NoError(t, err)

	filesErrors := new(FilesErrors)
	assert.NoError(t, json.Unmarshal(content, &filesErrors))

	// Verify all unique errors were written with the correct status
	errorsNamesMap := make(map[string]bool)
	for _, entity := range filesErrors.Errors {
		// Verify error's status
		assert.Equal(t, status, entity.Status, fmt.Sprintf("expecting error status to be: %s but got %s", status, entity.Status))
		// Verify error's unique name
		assert.False(t, errorsNamesMap[entity.Name], fmt.Sprintf("an error with the uniqe name \"%s\" was written more than once", entity.Name))
		errorsNamesMap[entity.Name] = true
		// Verify time
		assert.NotEmptyf(t, entity.Time, "expecting error's time stamp, but got empty string")
	}
	return len(filesErrors.Errors)
}

func TestGetErrorsFiles(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	retryableErrorsDirPath, err := coreutils.GetJfrogTransferRetryableDir()
	assert.NoError(t, err)
	assert.NoError(t, fileutils.CreateDirIfNotExist(retryableErrorsDirPath))

	skippedErrorsDirPath, err := coreutils.GetJfrogTransferSkippedDir()
	assert.NoError(t, err)
	assert.NoError(t, fileutils.CreateDirIfNotExist(skippedErrorsDirPath))

	repoKey := "my-repo-local"
	// Create 3 retryable errors files that belong to the repo.
	writeEmptyErrorsFile(t, retryableErrorsDirPath, repoKey, 0, 0)
	writeEmptyErrorsFile(t, retryableErrorsDirPath, repoKey, 0, 1)
	writeEmptyErrorsFile(t, retryableErrorsDirPath, repoKey, 1, 123)
	// Create a few retryable errors files that are distractions.
	writeEmptyErrorsFile(t, retryableErrorsDirPath, "wrong"+repoKey, 0, 0)
	writeEmptyErrorsFile(t, retryableErrorsDirPath, repoKey+"wrong", 0, 1)
	writeEmptyErrorsFile(t, retryableErrorsDirPath, "wrong-"+repoKey+"-wrong", 1, 0)
	writeEmptyErrorsFile(t, retryableErrorsDirPath, repoKey+"-0", 1, 0)
	writeEmptyErrorsFile(t, retryableErrorsDirPath, repoKey+"-0-1", 1, 0)

	// Create 1 skipped errors file that belongs to the repo.
	writeEmptyErrorsFile(t, skippedErrorsDirPath, repoKey, 0, 1)

	paths, err := getErrorsFiles([]string{repoKey}, true)
	assert.NoError(t, err)
	assert.Len(t, paths, 3)
	paths, err = getErrorsFiles([]string{repoKey}, false)
	assert.NoError(t, err)
	assert.Len(t, paths, 1)
}

func writeEmptyErrorsFile(t *testing.T, path, repoKey string, phase, counter int) {
	fileName := getErrorsFileName(repoKey, phase, state.ConvertTimeToEpochMilliseconds(time.Now()), counter)
	assert.NoError(t, ioutil.WriteFile(filepath.Join(path, fileName), nil, 0644))
}
