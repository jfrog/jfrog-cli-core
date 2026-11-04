package transferfiles

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	testRetryableStatus = api.Fail
	testSkippedStatus   = api.SkippedLargeProps
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
	addErrorsToChannel(&writeWaitGroup, errorsNumber, errorsChannelMng, api.Fail)
	// Add 'skipped errors' to the common errors channel.
	// These errors will be written into files located in the "skipped" directory under the Jfrog CLI home directory.
	addErrorsToChannel(&writeWaitGroup, errorsNumber, errorsChannelMng, api.SkippedLargeProps)

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

// TestTransferErrorsMng_sourceItemGoneGoesToSkippedNotRetryable verifies that items skipped as
// "source item gone" are auditable in the errors CSV via the skipped errors files, without being
// counted as failures or queued for retry.
func TestTransferErrorsMng_sourceItemGoneGoesToSkippedNotRetryable(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	errorsChannelMng := createErrorsChannelMng()
	transferErrorsMng, err := newTransferErrorsToFile(testRepoKey, 0, state.ConvertTimeToEpochMilliseconds(time.Now()), &errorsChannelMng, nil, nil)
	assert.NoError(t, err)

	var readWaitGroup sync.WaitGroup
	var writingErrorsErr error
	readWaitGroup.Add(1)
	go func() {
		defer readWaitGroup.Done()
		writingErrorsErr = transferErrorsMng.start()
	}()

	const goneNumber = 3
	for i := 0; i < goneNumber; i++ {
		errorsChannelMng.add(api.FileUploadStatusResponse{
			FileRepresentation: api.FileRepresentation{Repo: testRepoKey, Path: "path", Name: fmt.Sprintf("gone%d", i)},
			Status:             api.SkippedSourceItemGone,
			Reason:             ErrSourceItemGone.Error(),
		})
	}
	errorsChannelMng.close()
	readWaitGroup.Wait()
	assert.NoError(t, writingErrorsErr)

	skippedFiles, err := getErrorsFiles([]string{testRepoKey}, false)
	assert.NoError(t, err)
	var skippedEntities int
	for _, file := range skippedFiles {
		errs, readErr := readErrorFile(file)
		assert.NoError(t, readErr)
		for _, e := range errs.Errors {
			assert.Equal(t, api.SkippedSourceItemGone, e.Status)
			assert.Equal(t, ErrSourceItemGone.Error(), e.Reason)
		}
		skippedEntities += len(errs.Errors)
	}
	assert.Equal(t, goneNumber, skippedEntities)

	retryCount, err := getRetryErrorCount([]string{testRepoKey})
	assert.NoError(t, err)
	assert.Zero(t, retryCount, "gone items are not retryable errors")
}

// writeErrorRows writes the given rows through a TransferErrorsMng of the given phase, the way a
// transfer phase does (retryable and skipped statuses land in their own directories).
func writeErrorRows(t *testing.T, repoKey string, phaseId int, rows ...api.FileUploadStatusResponse) {
	errorsChannelMng := createErrorsChannelMng()
	mng, err := newTransferErrorsToFile(repoKey, phaseId, state.ConvertTimeToEpochMilliseconds(time.Now()), &errorsChannelMng, nil, nil)
	assert.NoError(t, err)
	var wg sync.WaitGroup
	var writeErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		writeErr = mng.start()
	}()
	for _, row := range rows {
		errorsChannelMng.add(row)
	}
	errorsChannelMng.close()
	wg.Wait()
	assert.NoError(t, writeErr)
}

func goneRow(repoKey, name string) api.FileUploadStatusResponse {
	return api.FileUploadStatusResponse{
		FileRepresentation: api.FileRepresentation{Repo: repoKey, Path: "path", Name: name},
		Status:             api.SkippedSourceItemGone,
		Reason:             ErrSourceItemGone.Error(),
	}
}

// TestCreateErrorsCsvSummary_dedupesSkippedRowsByPath: the same path skipped in two phases (or runs)
// must appear once in the errors CSV.
func TestCreateErrorsCsvSummary_dedupesSkippedRowsByPath(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	writeErrorRows(t, testRepoKey, 1, goneRow(testRepoKey, "a.bin"), goneRow(testRepoKey, "b.bin"))
	writeErrorRows(t, testRepoKey, 2, goneRow(testRepoKey, "a.bin"))

	csvPath, err := createErrorsCsvSummary([]string{testRepoKey}, time.Now())
	assert.NoError(t, err)
	assert.NotEmpty(t, csvPath)
	csvContent, err := os.ReadFile(csvPath)
	assert.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(csvContent), "a.bin"), "duplicate skipped rows for one path must be collapsed")
	assert.Equal(t, 1, strings.Count(string(csvContent), "b.bin"))
}

// TestErrorsRetryPhase_goneRowsAreNotRetried documents that items skipped as "source item gone"
// are audit-only: the retry phase reads the retryable directory only, so a rerun does not retry them.
func TestErrorsRetryPhase_goneRowsAreNotRetried(t *testing.T) {
	cleanUpJfrogHome, err := tests.SetJfrogHome()
	assert.NoError(t, err)
	defer cleanUpJfrogHome()

	writeErrorRows(t, testRepoKey, 1, goneRow(testRepoKey, "a.bin"))
	retryPhase := &errorsRetryPhase{phaseBase: phaseBase{repoKey: testRepoKey}}
	skip, err := retryPhase.shouldSkipPhase()
	assert.NoError(t, err)
	assert.True(t, skip, "only gone rows exist: nothing to retry")
	assert.Empty(t, retryPhase.errorsFilesToHandle)

	writeErrorRows(t, testRepoKey, 2, api.FileUploadStatusResponse{
		FileRepresentation: api.FileRepresentation{Repo: testRepoKey, Path: "path", Name: "failed.bin"}, Status: api.Fail, Reason: "boom"})
	skip, err = retryPhase.shouldSkipPhase()
	assert.NoError(t, err)
	assert.False(t, skip, "a real failure is still retried")
	assert.Len(t, retryPhase.errorsFilesToHandle, 1)
}

func addErrorsToChannel(writeWaitGroup *sync.WaitGroup, errorsNumber int, errorsChannelMng ErrorsChannelMng, status api.ChunkFileStatusType) {
	writeWaitGroup.Add(1)
	go func() {
		defer writeWaitGroup.Done()
		for i := 0; i < errorsNumber; i++ {
			errorsChannelMng.add(api.FileUploadStatusResponse{FileRepresentation: api.FileRepresentation{Repo: testRepoKey, Path: "path", Name: fmt.Sprintf("name%d", i)}, Status: status, StatusCode: 404, Reason: "reason"})
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

func getStatusType(isRetryable bool) api.ChunkFileStatusType {
	if isRetryable {
		return testRetryableStatus
	}
	return testSkippedStatus
}

// Check the number of errors, their status and their uniqueness by reading the file's content.
func validateErrorsFileContent(t *testing.T, path string, status api.ChunkFileStatusType) (entitiesNum int) {
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

	// Create 3 retryable and 1 skipped errors files that belong to repo1.
	writeEmptyErrorsFile(t, repo1Key, true, 0, 0)
	writeEmptyErrorsFile(t, repo1Key, true, 0, 1)
	writeEmptyErrorsFile(t, repo1Key, true, 1, 123)
	writeEmptyErrorsFile(t, repo1Key, false, 0, 1)

	// Create 2 retryable and 2 skipped errors files that belong to repo2.
	writeEmptyErrorsFile(t, repo2Key, true, 0, 0)
	writeEmptyErrorsFile(t, repo2Key, true, 2, 1)
	writeEmptyErrorsFile(t, repo2Key, false, 1, 0)
	writeEmptyErrorsFile(t, repo2Key, false, 0, 1)

	paths, err := getErrorsFiles([]string{repo1Key}, true)
	assert.NoError(t, err)
	assert.Len(t, paths, 3)
	paths, err = getErrorsFiles([]string{repo1Key}, false)
	assert.NoError(t, err)
	assert.Len(t, paths, 1)
	paths, err = getErrorsFiles([]string{repo1Key, repo2Key}, true)
	assert.NoError(t, err)
	assert.Len(t, paths, 5)
}

func writeEmptyErrorsFile(t *testing.T, repoKey string, retryable bool, phase, counter int) {
	var errorsDirPath string
	var err error
	if retryable {
		errorsDirPath, err = getJfrogTransferRepoRetryableDir(repoKey)
	} else {
		errorsDirPath, err = getJfrogTransferRepoSkippedDir(repoKey)
	}
	assert.NoError(t, err)
	assert.NoError(t, fileutils.CreateDirIfNotExist(errorsDirPath))

	fileName := fmt.Sprintf("%s-%d.json", getErrorsFileNamePrefix(repoKey, phase, state.ConvertTimeToEpochMilliseconds(time.Now())), counter)
	assert.NoError(t, os.WriteFile(filepath.Join(errorsDirPath, fileName), nil, 0644))
}

func TestDedupeSkippedRows_keyIncludesRepoPathNameAndStatus(t *testing.T) {
	row := func(repo, dir, name string, status api.ChunkFileStatusType) ExtendedFileUploadStatusResponse {
		return ExtendedFileUploadStatusResponse{FileUploadStatusResponse: api.FileUploadStatusResponse{
			FileRepresentation: api.FileRepresentation{Repo: repo, Path: dir, Name: name},
			Status:             status,
		}}
	}
	rows := []ExtendedFileUploadStatusResponse{
		row("repo-a", "dir1", "same.txt", api.SkippedSourceItemGone),
		row("repo-a", "dir1", "same.txt", api.SkippedSourceItemGone),  // exact duplicate: collapses
		row("repo-a", "dir2", "same.txt", api.SkippedSourceItemGone),  // other directory: kept
		row("repo-b", "dir1", "same.txt", api.SkippedSourceItemGone),  // other repo: kept
		row("repo-a", "dir1", "other.txt", api.SkippedSourceItemGone), // other name: kept
		row("repo-a", "dir1", "same.txt", api.SkippedLargeProps),      // other status: kept
	}
	deduped := dedupeSkippedRows(rows)
	if !assert.Len(t, deduped, 5) {
		return
	}
	assert.Equal(t, rows[0], deduped[0])
	assert.Equal(t, rows[2], deduped[1])
	assert.Equal(t, rows[3], deduped[2])
}
