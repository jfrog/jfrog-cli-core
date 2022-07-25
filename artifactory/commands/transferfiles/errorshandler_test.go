package transferfiles

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"math"
	"sync"
	"testing"
	"time"
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
	transferErrorsMng, err := newTransferErrorsToFile(testRepoKey, 0, convertTimeToEpochMilliseconds(time.Now()), &errorsChannelMng)
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
	errorsFiles, err := getErrorsFiles(testRepoKey, isRetryable)
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
