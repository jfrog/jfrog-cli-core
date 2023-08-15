package transferfiles

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	cmdutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"time"
)

// Max errors that will be written in a file
var maxErrorsInFile = 50000

// TransferErrorsMng manages multi threads writing errors.
// We want to create a file which contains all upload error statuses for each repository and phase.
// Those files will serve us in 2 cases:
// 1. Whenever we re-run 'transfer-files' command, we want to attempt to upload failed files again.
// 2. As part of the transfer process, we generate a csv file that contains all upload errors.
// In case an error occurs when creating those upload errors files, we would like to stop the transfer right away.
type TransferErrorsMng struct {
	// All go routines will write errors to the same channel
	errorsChannelMng *ErrorsChannelMng
	// Current repository that is being transferred
	repoKey string
	// Transfer current phase
	phaseId        int
	phaseStartTime string
	errorWriterMng errorWriterMng
	// Update state when changes occur
	stateManager *state.TransferStateManager
	// Update progressBar when changes occur
	progressBar *TransferProgressMng
}

type errorWriter struct {
	writer     *content.ContentWriter
	errorCount int
	filePath   string
}

type errorWriterMng struct {
	retryable errorWriter
	skipped   errorWriter
}

// newTransferErrorsToFile creates a manager for the files transferring process.
// localPath - Path to the dir which error files will be written to.
// repoKey - the repo that is being transferred
// phase - the phase number
// errorsChannelMng - all go routines will write to the same channel
func newTransferErrorsToFile(repoKey string, phaseId int, phaseStartTime string, errorsChannelMng *ErrorsChannelMng, progressBar *TransferProgressMng, stateManager *state.TransferStateManager) (*TransferErrorsMng, error) {
	err := initTransferErrorsDir(repoKey)
	if err != nil {
		return nil, err
	}
	mng := TransferErrorsMng{errorsChannelMng: errorsChannelMng, repoKey: repoKey, phaseId: phaseId, phaseStartTime: phaseStartTime, progressBar: progressBar, stateManager: stateManager}
	return &mng, nil
}

// Create transfer errors directory inside the JFrog CLI home directory.
// Inside the errors' directory creates directory for retryable errors and skipped errors.
// Return the root errors' directory path.
func initTransferErrorsDir(repoKey string) error {
	// Create errors directory
	errorsDirPath, err := getJfrogTransferRepoErrorsDir(repoKey)
	if err != nil {
		return err
	}
	err = makeDirIfDoesNotExists(errorsDirPath)
	if err != nil {
		return err
	}
	// Create retryable directory inside errors directory
	retryable, err := getJfrogTransferRepoRetryableDir(repoKey)
	if err != nil {
		return err
	}
	err = makeDirIfDoesNotExists(retryable)
	if err != nil {
		return err
	}
	// Create skipped directory inside errors directory
	skipped, err := getJfrogTransferRepoSkippedDir(repoKey)
	if err != nil {
		return err
	}
	return makeDirIfDoesNotExists(skipped)
}

func makeDirIfDoesNotExists(path string) error {
	exists, err := fileutils.IsDirExists(path, false)
	if err != nil {
		return err
	}
	if !exists {
		err = os.Mkdir(path, 0777)
	}
	return err
}

func (mng *TransferErrorsMng) start() (err error) {
	// Init content writers manager
	writerMng := errorWriterMng{}
	// Init the content writer which is responsible for writing 'retryable errors' into files.
	// In the next run we would like to retry and upload those files again.
	retryablePath, err := getJfrogTransferRepoRetryableDir(mng.repoKey)
	if err != nil {
		return err
	}
	writerRetry, retryFilePath, err := mng.newUniqueContentWriter(retryablePath)
	if err != nil {
		return err
	}
	defer func() {
		e := mng.errorWriterMng.retryable.closeWriter()
		if err == nil {
			err = e
		}
	}()
	writerMng.retryable = errorWriter{writer: writerRetry, filePath: retryFilePath}
	// Init the content writer which is responsible for writing 'skipped errors' into files.
	// In the next run we won't retry and upload those files.
	skippedPath, err := getJfrogTransferRepoSkippedDir(mng.repoKey)
	if err != nil {
		return err
	}
	writerSkip, skipFilePath, err := mng.newUniqueContentWriter(skippedPath)
	if err != nil {
		return err
	}
	defer func() {
		e := mng.errorWriterMng.skipped.closeWriter()
		if err == nil {
			err = e
		}
	}()
	writerMng.skipped = errorWriter{writer: writerSkip, filePath: skipFilePath}
	mng.errorWriterMng = writerMng

	// Read errors from channel and write them to files.
	for e := range mng.errorsChannelMng.channel {
		err = mng.writeErrorContent(e)
		if err != nil {
			return
		}
	}
	return
}

func (mng *TransferErrorsMng) newUniqueContentWriter(dirPath string) (*content.ContentWriter, string, error) {
	writer, err := content.NewContentWriter("errors", true, false)
	if err != nil {
		return nil, "", err
	}
	errorsFilePath, err := getUniqueErrorOrDelayFilePath(dirPath, func() string {
		return getErrorsFileNamePrefix(mng.repoKey, mng.phaseId, mng.phaseStartTime)
	})
	if err != nil {
		return nil, "", err
	}
	return writer, errorsFilePath, nil
}

func getErrorsFileNamePrefix(repoKey string, phaseId int, phaseStartTime string) string {
	return fmt.Sprintf("%s-%d-%s", repoKey, phaseId, phaseStartTime)
}

func (mng *TransferErrorsMng) writeErrorContent(e ExtendedFileUploadStatusResponse) error {
	var err error
	switch e.Status {
	case api.SkippedLargeProps:
		err = mng.writeSkippedErrorContent(e)
	default:
		err = mng.writeRetryableErrorContent(e)
		if err == nil && mng.progressBar != nil {
			// Increment the failures counter view by 1, following the addition
			// of the file to errors file.
			mng.progressBar.changeNumberOfFailuresBy(1)
			err = mng.stateManager.ChangeTransferFailureCountBy(1, true)
		}
	}
	return err
}

func (mng *TransferErrorsMng) writeSkippedErrorContent(e ExtendedFileUploadStatusResponse) error {
	log.Debug(fmt.Sprintf("write %s to file %s", e.Reason, mng.errorWriterMng.skipped.filePath))
	mng.errorWriterMng.skipped.writer.Write(e)
	mng.errorWriterMng.skipped.errorCount++
	// If file contains maximum number of errors - create and write to a new errors file
	if mng.errorWriterMng.skipped.errorCount == maxErrorsInFile {
		err := mng.errorWriterMng.skipped.closeWriter()
		if err != nil {
			return err
		}
		// Initialize variables for new errors file
		dirPath, err := getJfrogTransferRepoSkippedDir(mng.repoKey)
		if err != nil {
			return err
		}
		mng.errorWriterMng.skipped.writer, mng.errorWriterMng.skipped.filePath, err = mng.newUniqueContentWriter(dirPath)
		if err != nil {
			return err
		}
		mng.errorWriterMng.skipped.errorCount = 0
	}
	return nil
}

func (mng *TransferErrorsMng) writeRetryableErrorContent(e ExtendedFileUploadStatusResponse) error {
	log.Debug(fmt.Sprintf("write %s to file %s", e.Reason, mng.errorWriterMng.retryable.filePath))
	mng.errorWriterMng.retryable.writer.Write(e)
	mng.errorWriterMng.retryable.errorCount++
	// If file contains maximum number of errors - create and write to a new errors file
	if mng.errorWriterMng.retryable.errorCount == maxErrorsInFile {
		err := mng.errorWriterMng.retryable.closeWriter()
		if err != nil {
			return err
		}
		// Initialize variables for new errors file
		dirPath, err := getJfrogTransferRepoRetryableDir(mng.repoKey)
		if err != nil {
			return err
		}
		mng.errorWriterMng.retryable.writer, mng.errorWriterMng.retryable.filePath, err = mng.newUniqueContentWriter(dirPath)
		if err != nil {
			return err
		}
		mng.errorWriterMng.retryable.errorCount = 0
	}
	return nil
}

func (writerMng *errorWriter) closeWriter() error {
	// Close content writer and move output file to our working directory
	if writerMng.writer == nil {
		return nil
	}
	err := writerMng.writer.Close()
	if err != nil {
		return err
	}

	if writerMng.writer.GetFilePath() != "" {
		log.Debug(fmt.Sprintf("Saving errors outpt to: %s.", writerMng.filePath))
		err = fileutils.MoveFile(writerMng.writer.GetFilePath(), writerMng.filePath)
		if err != nil {
			err = fmt.Errorf("saving error file failed! failed moving %s to %s: %w", writerMng.writer.GetFilePath(), writerMng.filePath, err)
		}
	}
	return err
}

// Creates the csv errors files - contains the retryable and skipped errors.
// In case no errors were written returns empty string
func createErrorsCsvSummary(sourceRepos []string, timeStarted time.Time) (string, error) {
	errorsFiles, err := getErrorsFiles(sourceRepos, true)
	if err != nil {
		return "", err
	}

	skippedErrorsFiles, err := getErrorsFiles(sourceRepos, false)
	if err != nil {
		return "", err
	}

	errorsFiles = append(errorsFiles, skippedErrorsFiles...)
	if len(errorsFiles) == 0 {
		return "", nil
	}
	// Collect all errors from the given log files
	allErrors, err := parseErrorsFromLogFiles(errorsFiles)
	if err != nil {
		return "", err
	}
	return cmdutils.CreateCSVFile("transfer-files-logs", allErrors.Errors, timeStarted)
}

// Gets a list of all errors files from the CLI's cache.
// Errors-files contain files that were failed to upload or actions that were skipped because of known limitations.
func getErrorsFiles(repoKeys []string, isRetry bool) (filesPaths []string, err error) {
	if isRetry {
		return getErrorOrDelayFiles(repoKeys, getJfrogTransferRepoRetryableDir)
	}
	return getErrorOrDelayFiles(repoKeys, getJfrogTransferRepoSkippedDir)
}

// Count the number of transfer failures of a given subset of repositories
func getRetryErrorCount(repoKeys []string) (int, error) {
	files, err := getErrorsFiles(repoKeys, true)
	if err != nil {
		return -1, err
	}

	count := 0
	for _, file := range files {
		failedFiles, err := readErrorFile(file)
		if err != nil {
			return -1, err
		}
		count += len(failedFiles.Errors)
	}
	return count, nil
}

// Reads an error file from a given path, parses and populate a given FilesErrors instance with the file information
func readErrorFile(path string) (FilesErrors, error) {
	// Stores the errors read from the errors file.
	var failedFiles FilesErrors

	fContent, err := os.ReadFile(path)
	if err != nil {
		return failedFiles, errorutils.CheckError(err)
	}

	err = json.Unmarshal(fContent, &failedFiles)
	if err != nil {
		return failedFiles, errorutils.CheckError(err)
	}
	return failedFiles, nil
}

// ErrorsChannelMng handles the uploading errors and adds them to a common channel.
// Stops adding elements to the channel if an error occurs while handling the files.
type ErrorsChannelMng struct {
	channel chan ExtendedFileUploadStatusResponse
	err     error
}

type FilesErrors struct {
	Errors []ExtendedFileUploadStatusResponse `json:"errors,omitempty"`
}

type ExtendedFileUploadStatusResponse struct {
	api.FileUploadStatusResponse
	Time string `json:"time,omitempty"`
}

func (mng ErrorsChannelMng) add(element api.FileUploadStatusResponse) (stopped bool) {
	if mng.shouldStop() {
		return true
	}
	extendedElement := ExtendedFileUploadStatusResponse{FileUploadStatusResponse: element, Time: time.Now().Format(time.RFC3339)}
	mng.channel <- extendedElement
	return false
}

// Close channel
func (mng ErrorsChannelMng) close() {
	close(mng.channel)
}

func (mng ErrorsChannelMng) shouldStop() bool {
	// Stop adding elements to the channel if a 'blocking' error occurred in a different go routine.
	return mng.err != nil
}

func createErrorsChannelMng() ErrorsChannelMng {
	errorChannel := make(chan ExtendedFileUploadStatusResponse, fileWritersChannelSize)
	return ErrorsChannelMng{channel: errorChannel}
}
