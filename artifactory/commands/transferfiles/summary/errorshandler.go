package summary

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"
)

const (
	FINISH = -1
	// Max errors that will be written in a file
	//maxErrorsInFile = 5000
	maxErrorsInFile = 2
	errorsDirName   = "errors"
)

type TransferFilesSummary struct {
	success int64
	fail    int64
	errors  *[]ErrorEntity
}

// ErrorEntity represents entity in the transfer files errors list.
type ErrorEntity struct {
	RepoName   string `csv:"repositoryName"`
	Path       string `csv:"pathInArtifactory"`
	FileName   string `csv:"fileName"`
	Status     string `csv:"statusCode"`
	StatusCode int    `csv:"statusCode"`
	Reason     string `csv:"reason"`
}

func newErrorEntity(repoName, path, fileName, status string, statusCode int, reason string) ErrorEntity {
	return ErrorEntity{RepoName: repoName, Path: path, FileName: fileName, Status: status, StatusCode: statusCode, Reason: reason}
}

// TransferErrorsMng managing multi threads writing errors.
type TransferErrorsMng struct {
	// Max errors saved locally in buffer before flushing to disk
	bufferSize int
	// All thread will write errors to this channel
	errorsChannel chan ErrorEntity
	// Path to the dir which error files will be written to.
	errorsDirPath string
	// Current repository that is being transferred
	repoName string
	// Transfer current phase
	phase int
}

// WriteTransferErrorsToFile creates manager for the files transferring process.
// localPath- Path to the dir which error files will be written to.
// repoName- the repo that being transferred
// phase-the phase number
// bufferSize- how many errorsEntity to write in buffer before flushing to disk
func WriteTransferErrorsToFile(repoName string, phase, bufferSize int, errorsChannel chan ErrorEntity) error {
	errorsDirPath, err := getTransferErrorsDir()
	if err != nil {
		return err
	}

	mng := TransferErrorsMng{bufferSize: bufferSize, errorsChannel: errorsChannel, errorsDirPath: errorsDirPath, repoName: repoName, phase: phase}
	mng.Start()
	return nil
}

func getTransferErrorsDir() (string, error) {
	transferDirPath, err := coreutils.GetJfrogTransferDir()
	if err != nil {
		return "", err
	}
	errorsDirPath := filepath.Join(transferDirPath, errorsDirName)
	exists, err := fileutils.IsDirExists(errorsDirPath, false)
	if !exists {
		err = os.Mkdir(errorsDirPath, 0777)
		if err != nil {
			return "", err
		}
	}
	return errorsDirPath, nil
}
func (mng *TransferErrorsMng) Start() error {
	// Counts error written to a specific file
	errorsCount := 0
	// TODO: remove this
	filesCounter := 0
	writer, errorsFilePath, err := mng.newContentWriter(filesCounter)
	if err != nil {
		return err
	}
	for e := range mng.errorsChannel {
		log.Info(fmt.Sprintf("Status code:  %d.", e.StatusCode))
		if e.StatusCode != FINISH {
			writer.Write(e)
			errorsCount++
			// If file contains maximum number off errors - create and write to a new errors file
			if errorsCount == maxErrorsInFile {
				closeWriter(writer, errorsFilePath)
				log.Info(fmt.Sprintf("Closing writer and file %s.", errorsFilePath))
				// Initialize variables for new errors file
				filesCounter++
				writer, errorsFilePath, err = mng.newContentWriter(filesCounter)
				if err != nil {
					return err
				}
				errorsCount = 0
				continue
			}
		} else {
			closeWriter(writer, errorsFilePath)
			log.Info(fmt.Sprintf("Finish writing repository's errors to file %s.", errorsFilePath))
			continue
		}
	}

	close(mng.errorsChannel)
	return nil
}

func (mng *TransferErrorsMng) newContentWriter(counter int) (*content.ContentWriter, string, error) {
	writer, err := content.NewContentWriter("errors", true, false)
	if err != nil {
		return nil, "", err
	}
	errorsFilePath := path.Join(mng.errorsDirPath, fmt.Sprintf("%s-%d-%s-%d.json", mng.repoName, mng.phase, strconv.FormatInt(time.Now().Unix(), 10), counter))
	return writer, errorsFilePath, nil
}

func closeWriter(writer *content.ContentWriter, errorsFilePath string) error {
	// Close content writer and move output file to our working directory
	err := writer.Close()
	if err != nil {
		log.Error(err)
		return err
	}
	log.Info(fmt.Sprintf("Move %s to %s.", writer.GetFilePath(), errorsFilePath))
	err = fileutils.MoveFile(writer.GetFilePath(), errorsFilePath)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}
