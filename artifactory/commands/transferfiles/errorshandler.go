package transferfiles

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path"
	"time"
)

const (
	// Max errors that will be written in a file
	maxErrorsInFile = 50000
)

// TransferErrorsMng managing multi threads writing errors.
type TransferErrorsMng struct {
	// All thread will write errors to this channel
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
	// Retryable writer
	retryablePath, err := coreutils.GetJfrogTransferRetryableDir()
	if err != nil {
		return err
	}
	writerRetry, retryFilePath, err := mng.newContentWriter(retryablePath, 0)
	if err != nil {
		return err
	}
	writerMng.retryable = errorWriter{writer: writerRetry, fileIndex: 0, filePath: retryFilePath}

	// Skipped writer
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

// WriteTransferErrorsToFile creates manager for the files transferring process.
// localPath- Path to the dir which error files will be written to.
// repoName- the repo that being transferred
// phase-the phase number
// bufferSize- how many errorsEntity to write in buffer before flushing to disk
func WriteTransferErrorsToFile(repoName string, phaseId int, phaseStartTime string, errorsChannel chan FileUploadStatusResponse) error {
	err := initTransferErrorsDir()
	if err != nil {
		return err
	}

	mng := TransferErrorsMng{errorsChannel: errorsChannel, repoName: repoName, phaseId: phaseId, phaseStartTime: phaseStartTime}
	err = mng.initErrorWriterMng()
	if err != nil {
		return err
	}
	mng.Start()
	return nil
}

// Create errors directory inside '.jfrog/transfer' directory.
// Inside the error directory creates directory for retryable errors and skip errors.
// Return the root errors' directory path.
func initTransferErrorsDir() error {
	errorsDirPath, err := coreutils.GetJfrogTransferErrorsDir()
	if err != nil {
		return err
	}
	exists, err := fileutils.IsDirExists(errorsDirPath, false)
	if !exists {
		err = os.Mkdir(errorsDirPath, 0777)
		if err != nil {
			return err
		}
		// Create retryable and skip errors directories
		retryable, err := coreutils.GetJfrogTransferRetryableDir()
		if err != nil {
			return err
		}
		err = os.Mkdir(retryable, 0777)
		if err != nil {
			return err
		}
		skipped, err := coreutils.GetJfrogTransferSkippedDir()
		if err != nil {
			return err
		}
		err = os.Mkdir(skipped, 0777)
		if err != nil {
			return err
		}
	}
	return nil
}
func (mng *TransferErrorsMng) Start() error {
	for e := range mng.errorsChannel {
		log.Info(fmt.Sprintf("Status code:  %s.", e.StatusCode))
		mng.writeErrorContent(e)
	}
	time.Sleep(2 * time.Second)
	mng.errorWriterMng.retryable.closeWriter()
	mng.errorWriterMng.skipped.closeWriter()
	// TODO debug
	log.Info(fmt.Sprintf("Finish writing repository's retryable errors to files %s.", mng.errorWriterMng.retryable.filePath))
	log.Info(fmt.Sprintf("Finish writing repository's skipped errors to files %s.", mng.errorWriterMng.skipped.filePath))

	return nil
}

func (mng *TransferErrorsMng) newContentWriter(dirPath string, index int) (*content.ContentWriter, string, error) {
	writer, err := content.NewContentWriter("errors", true, false)
	if err != nil {
		return nil, "", err
	}
	errorsFilePath := path.Join(dirPath, fmt.Sprintf("%s-%d-%s-%d.json", mng.repoName, mng.phaseId, mng.phaseStartTime, index))
	return writer, errorsFilePath, nil
}

func (mng *TransferErrorsMng) writeErrorContent(e FileUploadStatusResponse) error {
	switch e.Status {
	case SkippedLargeProps:
		log.Info(fmt.Sprintf("write %s to file %s", e.Reason, mng.errorWriterMng.skipped.filePath))
		mng.errorWriterMng.skipped.writer.Write(e)
		mng.errorWriterMng.skipped.errorCount++
		// If file contains maximum number off errors - create and write to a new errors file
		if mng.errorWriterMng.skipped.errorCount == maxErrorsInFile {
			mng.errorWriterMng.skipped.closeWriter()
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
			return nil

		}
	default:
		log.Info(fmt.Sprintf("write %s to file %s", e.Reason, mng.errorWriterMng.retryable.filePath))
		mng.errorWriterMng.retryable.writer.Write(e)
		mng.errorWriterMng.retryable.errorCount++
		// If file contains maximum number off errors - create and write to a new errors file
		if mng.errorWriterMng.retryable.errorCount == maxErrorsInFile {
			mng.errorWriterMng.retryable.closeWriter()
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
			return nil
		}
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
		log.Error(err)
		return err
	}

	if writerMng.writer.GetFilePath() != "" {
		log.Debug(fmt.Sprintf("Move %s to %s.", writerMng.writer.GetFilePath(), writerMng.filePath))
		err = fileutils.MoveFile(writerMng.writer.GetFilePath(), writerMng.filePath)
	}
	return err
}
