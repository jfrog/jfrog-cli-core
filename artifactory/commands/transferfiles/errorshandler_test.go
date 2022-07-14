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

	errorsNumber := 40
	maxErrorsInFile = 20
	errorsChannelMng := createErrorsChannelMng()
	transferErrorsMng, err := newTransferErrorsToFile(testRepoKey, 0, convertTimeToEpochMilliseconds(time.Now()), &errorsChannelMng)
	assert.NoError(t, err)

	var writeWaitGroup sync.WaitGroup
	var readWaitGroup sync.WaitGroup

	// Error returned from the "writing transfer errors to file" mechanism
	var writingErrorsErr error
	// Start reading from the errors channel, and write errors into the relevant files.
	readWaitGroup.Add(1)
	go func() {
		defer readWaitGroup.Done()
		writingErrorsErr = transferErrorsMng.start()
	}()

	// Add 'retryable errors' to the common errors channel.
	// These errors will be written into files located in the "retryable" directory under the Jfrog CLI home directory.
	writeWaitGroup.Add(1)
	go func() {
		defer writeWaitGroup.Done()
		for i := 0; i < errorsNumber; i++ {
			errorsChannelMng.channel <- FileUploadStatusResponse{FileRepresentation: FileRepresentation{Repo: testRepoKey, Path: "path", Name: "name"}, Status: Fail, StatusCode: i, Reason: "reason"}
		}
	}()

	// Add 'skipped errors' to the common errors channel.
	// These errors will be written into files located in the "skipped" directory under the Jfrog CLI home directory.
	writeWaitGroup.Add(1)
	go func() {
		defer writeWaitGroup.Done()
		for i := 0; i < errorsNumber; i++ {
			errorsChannelMng.channel <- FileUploadStatusResponse{FileRepresentation: FileRepresentation{Repo: testRepoKey, Path: "path", Name: "name"}, Status: SkippedLargeProps, StatusCode: i, Reason: "reason"}
		}
	}()

	writeWaitGroup.Wait()
	errorsChannelMng.close()
	readWaitGroup.Wait()
	assert.NoError(t, writingErrorsErr)
	numOfFiles := int(math.Ceil(float64(errorsNumber) / float64(maxErrorsInFile)))
	validateErrorsFiles(t, numOfFiles, maxErrorsInFile, true)
	validateErrorsFiles(t, numOfFiles, maxErrorsInFile, false)
}

func validateErrorsFiles(t *testing.T, filesNum, errorsNum int, isRetryable bool) {
	errorsFiles, err := getErrorsFiles(testRepoKey, isRetryable)
	status := getStatusType(isRetryable)
	assert.NoError(t, err)
	assert.Equal(t, filesNum, len(errorsFiles), "unexpected number of error files.")
	for i := 0; i < filesNum; i++ {
		entitiesNum := validateErrorsFileContent(t, errorsFiles[i], status)
		assert.Equal(t, errorsNum, entitiesNum)
	}
}

func getStatusType(isRetryable bool) ChunkFileStatusType {
	if isRetryable {
		return testRetryableStatus
	}
	return testSkippedStatus
}

func validateErrorsFileContent(t *testing.T, path string, status ChunkFileStatusType) (entitiesNum int) {
	exists, err := fileutils.IsFileExists(path, false)
	assert.NoError(t, err)
	assert.True(t, exists, fmt.Sprintf("file: %s does not exist", path))

	var content []byte
	content, err = fileutils.ReadFile(path)
	assert.NoError(t, err)

	fileErrors := new(FilesErrors)
	assert.NoError(t, json.Unmarshal(content, &fileErrors))

	// Verify all unique errors were written with the correct status
	statusCodeMap := make(map[int]bool)
	for _, entity := range fileErrors.Errors {
		if entity.Status != status {
			assert.Fail(t, fmt.Sprintf("expecting error status to be: %s but got %s", status, entity.Status))
			return
		}
		// Verify error's unique status code
		if statusCodeMap[entity.StatusCode] == true {
			assert.Fail(t, fmt.Sprintf("an error with a uniqe status code %d was written more than one", entity.StatusCode))
			return
		}
		statusCodeMap[entity.StatusCode] = true
	}
	return len(fileErrors.Errors)
}
