package transferfiles

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path/filepath"
)

const (
	// Max errors that will be written in a file
	maxErrorsInFile = 50000
)

// TransferErrorsMng managing multi threads writing errors.
type TransferErrorsMng struct {
	// All go routines will write errors to this channel
	errorsChannel chan FileUploadStatusResponse
	// Current repository that is being transferred
	repoName string
	// Transfer current phase
	phaseId        int
	phaseStartTime string

	errorWriterMng errorWriterMng
}

type errorWriter struct {
	writer     *content.ContentWriter
	errorCount int
	// In case we have multiple errors files - we index them
	fileIndex int
	filePath  string
}

type errorWriterMng struct {
	retryable errorWriter
	skipped   errorWriter
}

func (mng *TransferErrorsMng) initErrorWriterMng() error {
	writerMng := errorWriterMng{}
	// Init the content writer which responsible for writing retryable errors - That means errors which related to files that we should try to transfer again in the next run
	retryablePath, err := coreutils.GetJfrogTransferRetryableDir()
	if err != nil {
		return err
	}
	writerRetry, retryFilePath, err := mng.newContentWriter(retryablePath, 0)
	if err != nil {
		return err
	}
	writerMng.retryable = errorWriter{writer: writerRetry, fileIndex: 0, filePath: retryFilePath}

	// Init the content writer which responsible for writing skipped errors - That means errors which related to files that we skipped on during transfer, and we shouldn't try transferring them again in the next run
	skippedPath, err := coreutils.GetJfrogTransferSkippedDir()
	if err != nil {
		return err
	}
	writerSkip, skipFilePath, err := mng.newContentWriter(skippedPath, 0)
	if err != nil {
		return err
	}
	writerMng.skipped = errorWriter{writer: writerSkip, fileIndex: 0, filePath: skipFilePath}
	mng.errorWriterMng = writerMng
	return nil
}

// newTransferErrorsToFile creates a manager for the files transferring process.
// localPath - Path to the dir which error files will be written to.
// repoName - the repo that is being transferred
// phase - the phase number
// bufferSize - the total of errorsEntity instances to write in buffer before flushing to disk
func newTransferErrorsToFile(repoName string, phaseId int, phaseStartTime string, errorsChannel chan FileUploadStatusResponse) (*TransferErrorsMng, error) {
	err := initTransferErrorsDir()
	if err != nil {
		return nil, err
	}

	mng := TransferErrorsMng{errorsChannel: errorsChannel, repoName: repoName, phaseId: phaseId, phaseStartTime: phaseStartTime}
	err = mng.initErrorWriterMng()
	return &mng, err
}

// Create transfer errors directory inside the JFrog CLI home directory.
// Inside the errors directory creates directory for retryable errors and skipped errors.
// Return the root errors' directory path.
func initTransferErrorsDir() error {
	// Create errors directory
	errorsDirPath, err := coreutils.GetJfrogTransferErrorsDir()
	if err != nil {
		return err
	}
	err = makeDirIfDoesNotExists(errorsDirPath)
	if err != nil {
		return err
	}
	// Create retryable directory inside errors directory
	retryable, err := coreutils.GetJfrogTransferRetryableDir()
	if err != nil {
		return err
	}
	err = makeDirIfDoesNotExists(retryable)
	if err != nil {
		return err
	}
	// Create skipped directory inside errors directory
	skipped, err := coreutils.GetJfrogTransferSkippedDir()
	if err != nil {
		return err
	}
	err = makeDirIfDoesNotExists(skipped)
	if err != nil {
		return err
	}
	return nil
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

func (mng *TransferErrorsMng) start() error {
	for e := range mng.errorsChannel {
		err := mng.writeErrorContent(e)
		if err != nil {
			return err
		}
	}
	err := mng.errorWriterMng.retryable.closeWriter()
	if err != nil {
		return err
	}
	err = mng.errorWriterMng.skipped.closeWriter()
	if err != nil {
		return err
	}

	return nil
}

func (mng *TransferErrorsMng) newContentWriter(dirPath string, index int) (*content.ContentWriter, string, error) {
	writer, err := content.NewContentWriter("errors", true, false)
	if err != nil {
		return nil, "", err
	}
	errorsFilePath := filepath.Join(dirPath, fmt.Sprintf("%s-%d-%s-%d.json", mng.repoName, mng.phaseId, mng.phaseStartTime, index))
	return writer, errorsFilePath, nil
}

func (mng *TransferErrorsMng) writeErrorContent(e FileUploadStatusResponse) error {
	var err error
	switch e.Status {
	case SkippedLargeProps:
		err = mng.writeSkippedErrorContent(e)
	default:
		err = mng.writeRetryableErrorContent(e)
	}
	return err
}

func (mng *TransferErrorsMng) writeSkippedErrorContent(e FileUploadStatusResponse) error {
	log.Info(fmt.Sprintf("write %s to file %s", e.Reason, mng.errorWriterMng.skipped.filePath))
	mng.errorWriterMng.skipped.writer.Write(e)
	mng.errorWriterMng.skipped.errorCount++
	// If file contains maximum number of errors - create and write to a new errors file
	if mng.errorWriterMng.skipped.errorCount == maxErrorsInFile {
		err := mng.errorWriterMng.skipped.closeWriter()
		if err != nil {
			return err
		}
		// Initialize variables for new errors file
		mng.errorWriterMng.skipped.fileIndex++
		dirPath, err := coreutils.GetJfrogTransferSkippedDir()
		if err != nil {
			return err
		}
		mng.errorWriterMng.skipped.writer, mng.errorWriterMng.skipped.filePath, err = mng.newContentWriter(dirPath, mng.errorWriterMng.skipped.fileIndex)
		if err != nil {
			return err
		}
		mng.errorWriterMng.skipped.errorCount = 0
	}
	return nil
}

func (mng *TransferErrorsMng) writeRetryableErrorContent(e FileUploadStatusResponse) error {
	log.Info(fmt.Sprintf("write %s to file %s", e.Reason, mng.errorWriterMng.retryable.filePath))
	mng.errorWriterMng.retryable.writer.Write(e)
	mng.errorWriterMng.retryable.errorCount++
	// If file contains maximum number of errors - create and write to a new errors file
	if mng.errorWriterMng.retryable.errorCount == maxErrorsInFile {
		err := mng.errorWriterMng.retryable.closeWriter()
		if err != nil {
			return err
		}
		// Initialize variables for new errors file
		mng.errorWriterMng.retryable.fileIndex++
		dirPath, err := coreutils.GetJfrogTransferRetryableDir()
		if err != nil {
			return err
		}
		mng.errorWriterMng.retryable.writer, mng.errorWriterMng.retryable.filePath, err = mng.newContentWriter(dirPath, mng.errorWriterMng.retryable.fileIndex)
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
			err = fmt.Errorf(fmt.Sprintf("Saving error file failed! failed moving %s to %s", writerMng.writer.GetFilePath(), writerMng.filePath), err)
		}
	}
	return err
}

// Creates the csv errors files - contains the retryable and skipped errors.
// In case no errors were written returns empty string
func createErrorsCsvSummary() (string, error) {
	// Create csv errors file
	csvTempDIr, err := fileutils.CreateTempDir()
	if err != nil {
		return "", err
	}

	// Get a list of retryable errors files from the errors directory
	retryable, err := coreutils.GetJfrogTransferRetryableDir()
	if err != nil {
		return "", err
	}
	errorsFiles, err := fileutils.ListFiles(retryable, false)
	if err != nil {
		return "", err
	}

	// Get a list of skipped errors files from the errors directory
	skipped, err := coreutils.GetJfrogTransferSkippedDir()
	if err != nil {
		return "", err
	}
	skippedFilesList, err := fileutils.ListFiles(skipped, false)
	if err != nil {
		return "", err
	}

	errorsFiles = append(errorsFiles, skippedFilesList...)
	if len(errorsFiles) == 0 {
		return "", nil
	}
	return CreateErrorsSummaryCsvFile(errorsFiles, csvTempDIr)
}
