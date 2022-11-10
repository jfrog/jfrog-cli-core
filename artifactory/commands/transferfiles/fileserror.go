package transferfiles

import (
	"errors"
	"fmt"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"strings"
	"time"
)

type errorFileHandlerFunc func() parallel.TaskFunc

// Manages the phase of handling upload failures that were collected during previous runs and phases.
type errorsRetryPhase struct {
	phaseBase
	errorsFilesToHandle []string
}

func (e *errorsRetryPhase) getPhaseName() string {
	return "Retry Transfer Errors Phase"
}

func (e *errorsRetryPhase) run() error {
	return e.handlePreviousUploadFailures()
}

// Consumes errors files with upload failures from cache and tries to transfer these files again.
// Does so by creating and uploading by chunks, and polling on status.
// Consumed errors files are deleted, new failures are written to new files.
func (e *errorsRetryPhase) handlePreviousUploadFailures() error {
	if len(e.errorsFilesToHandle) == 0 {
		return nil
	}
	log.Info("Starting to handle previous upload failures...")
	e.transferManager = newTransferManager(e.phaseBase, getDelayUploadComparisonFunctions(e.repoSummary.PackageType))
	action := func(pcWrapper *producerConsumerWrapper, uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
		errFileHandler := e.createErrorFilesHandleFunc(pcWrapper, uploadChunkChan, delayHelper, errorsChannelMng)
		_, err := pcWrapper.chunkBuilderProducerConsumer.AddTaskWithError(errFileHandler(), pcWrapper.errorsQueue.AddError)
		return err
	}
	delayAction := consumeAllDelayFiles
	err := e.transferManager.doTransferWithProducerConsumer(action, delayAction)
	if err == nil {
		log.Info("Done handling previous upload failures.")
	}
	return err
}

func convertUploadStatusToFileRepresentation(statuses []ExtendedFileUploadStatusResponse) (files []api.FileRepresentation) {
	for _, status := range statuses {
		files = append(files, status.FileRepresentation)
	}
	return
}

func (e *errorsRetryPhase) handleErrorsFile(errFilePath string, pcWrapper *producerConsumerWrapper, uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
	if ShouldStop(&e.phaseBase, &delayHelper, errorsChannelMng) {
		return nil
	}
	log.Debug("Handling errors file: '", errFilePath, "'")

	// Read and parse the file
	failedFiles, err := readErrorFile(errFilePath)
	if err != nil {
		return err
	}

	if e.progressBar != nil {
		// Since we're about to handle the transfer retry of the failed files,
		// we should now decrement the failures counter view.
		e.progressBar.changeNumberOfFailuresBy(-1 * len(failedFiles.Errors))
		err = e.stateManager.ChangeTransferFailureCountBy(uint(len(failedFiles.Errors)), false)
		if err != nil {
			return err
		}
	}

	// Upload
	shouldStop, err := uploadByChunks(convertUploadStatusToFileRepresentation(failedFiles.Errors), uploadChunkChan, e.phaseBase, delayHelper, errorsChannelMng, pcWrapper)
	if err != nil || shouldStop {
		return err
	}

	// Remove the file, so it won't be consumed again.
	err = os.Remove(errFilePath)
	if err != nil {
		return errorutils.CheckError(err)
	}

	log.Debug("Done handling errors file: '", errFilePath, "'. Deleting it...")
	return nil
}

func (e *errorsRetryPhase) createErrorFilesHandleFunc(pcWrapper *producerConsumerWrapper, uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) errorFileHandlerFunc {
	return func() parallel.TaskFunc {
		return func(threadId int) error {
			var errList []string
			var err error
			for _, errFile := range e.errorsFilesToHandle {
				err = e.handleErrorsFile(errFile, pcWrapper, uploadChunkChan, delayHelper, errorsChannelMng)
				if err != nil {
					errList = append(errList, fmt.Sprintf("handleErrorsFile for %s failed with error: \n%s", errFile, err.Error()))
				}
			}
			if len(errList) > 0 {
				err = errors.New(strings.Join(errList, "\n"))
			}
			return err
		}
	}
}

func (e *errorsRetryPhase) shouldSkipPhase() (bool, error) {
	var err error
	// check if error file exist for this repo
	e.errorsFilesToHandle, err = getErrorsFiles([]string{e.repoKey}, true)
	if err != nil {
		return true, err
	}
	return len(e.errorsFilesToHandle) < 1, nil
}

func (e *errorsRetryPhase) phaseStarted() error {
	e.startTime = time.Now()
	return nil
}

func (e *errorsRetryPhase) initProgressBar() error {
	if e.progressBar == nil {
		return nil
	}

	// Init progress with the number of tasks of errors file handling (fixing previous upload failures)
	filesCount := 0
	for _, path := range e.errorsFilesToHandle {

		failedFiles, err := readErrorFile(path)
		if err != nil {
			return err
		}
		filesCount += len(failedFiles.Errors)
	}

	// The progress bar will also be responsible to display the number of delayed items for this repository.
	// Those delayed artifacts will be handled at the end of this phase in case they exist.
	delayFiles, err := getDelayFiles([]string{e.repoKey})
	if err != nil {
		return err
	}
	delayCount, err := countDelayFilesContent(delayFiles)
	if err != nil {
		return err
	}
	e.progressBar.AddPhase3(int64(filesCount) + int64(delayCount))

	return nil
}

func (e *errorsRetryPhase) phaseDone() error {
	if e.progressBar != nil {
		return e.progressBar.DonePhase(e.phaseId)
	}
	return nil
}
