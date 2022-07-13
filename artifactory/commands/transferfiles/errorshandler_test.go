package transferfiles

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func TestTransferErrorsMng(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()
	repoKey := "repo"
	errorsNumber := 105
	//maxErrorsInFile = 20
	errorsChannelMng := createErrorsChannelMng()
	transferErrorsMng, err := newTransferErrorsToFile(repoKey, 0, convertTimeToEpochMilliseconds(time.Now()), &errorsChannelMng)
	assert.NoError(t, err)
	// Error returned from the "writing transfer errors to file" mechanism
	var writingErrorsErr error
	var writeWaitGroup sync.WaitGroup
	var readWaitGroup sync.WaitGroup
	writeWaitGroup.Add(1)
	go func() {
		defer writeWaitGroup.Done()
		for i := 0; i < errorsNumber; i++ {
			fmt.Println(i)
			errorsChannelMng.channel <- FileUploadStatusResponse{FileRepresentation: FileRepresentation{Repo: "repo", Path: "path", Name: "name"}, Status: Fail, StatusCode: i, Reason: "reason"}
		}
	}()

	writeWaitGroup.Add(1)
	go func() {
		defer writeWaitGroup.Done()
		for i := 0; i < errorsNumber; i++ {
			fmt.Println(i)
			errorsChannelMng.channel <- FileUploadStatusResponse{FileRepresentation: FileRepresentation{Repo: "repo", Path: "path", Name: "name"}, Status: SkippedLargeProps, StatusCode: i, Reason: "reason"}
		}
	}()

	readWaitGroup.Add(1)
	go func() {
		defer readWaitGroup.Done()
		writingErrorsErr = transferErrorsMng.start()
	}()
	writeWaitGroup.Wait()
	errorsChannelMng.close()
	readWaitGroup.Wait()
	assert.NoError(t, writingErrorsErr)

	retryableErrorsDirPath, err := getErrorsFiles("repo", true)
	assert.Equal(t, 1, len(retryableErrorsDirPath), "got more than 1 retryable error file")
	entitiesNum, err := validateErrorsFiles(retryableErrorsDirPath[0], Fail)
	assert.NoError(t, err)
	assert.Equal(t, errorsNumber, entitiesNum)
}

func validateErrorsFiles(path string, status ChunkFileStatusType) (entitiesNum int, err error) {
	exists, err := fileutils.IsFileExists(path, false)
	if err != nil {
		return
	}
	if !exists {
		err = fmt.Errorf("log file: %s does not exist", path)
		return
	}
	var content []byte
	content, err = fileutils.ReadFile(path)
	if err != nil {
		return
	}
	fileErrors := new(FilesErrors)
	err = errorutils.CheckError(json.Unmarshal(content, &fileErrors))
	if err != nil {
		return
	}
	// Verify all unique errors were written with the correct status
	for i, entity := range fileErrors.Errors {
		if entity.Status != status {
			err = fmt.Errorf("expecting entity status to be: %s but got %s", status, entity.Status)
			return
		}
		if entity.StatusCode != i {
			err = fmt.Errorf("missing entity number %d", i)
			return
		}
	}
	entitiesNum = len(fileErrors.Errors)
	return
}
