package transferfiles

import (
	"encoding/json"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"time"
)

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
	manager := newTransferManager(e.phaseBase, getDelayUploadComparisonFunctions(e.repoSummary.PackageType))
	action := func(pcWrapper *producerConsumerWrapper, uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
		return e.handleErrorsFiles(pcWrapper, uploadTokensChan, delayHelper, errorsChannelMng)
	}
	err := manager.doTransferWithSingleProducer(action)
	if err == nil {
		log.Info("Done handling previous upload failures.")
	}
	return err
}

func convertUploadStatusToFileRepresentation(statuses []ExtendedFileUploadStatusResponse) (files []FileRepresentation) {
	for _, status := range statuses {
		files = append(files, status.FileRepresentation)
	}
	return
}

func (e *errorsRetryPhase) handleErrorsFiles(pcWrapper *producerConsumerWrapper, uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
	for _, path := range e.errorsFilesToHandle {
		if ShouldStop(&e.phaseBase, &delayHelper, errorsChannelMng) {
			return nil
		}
		log.Debug("Handling errors file: '" + path + "'")

		// read and parse file
		failedFiles, err := e.readErrorFile(path)
		if err != nil {
			return err
		}

		// upload
		shouldStop, err := uploadByChunks(convertUploadStatusToFileRepresentation(failedFiles.Errors), uploadTokensChan, e.phaseBase, delayHelper, errorsChannelMng, pcWrapper)
		if err != nil || shouldStop {
			return err
		}

		// Remove the file, so it won't be consumed again.
		err = os.Remove(path)
		if err != nil {
			return errorutils.CheckError(err)
		}

		log.Debug("Done handling errors file: '" + path + "'. Deleting it...")
	}
	return nil
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

// Reads an error file from a given path, parses and populate a given FilesErrors instance with the file information
func (e *errorsRetryPhase) readErrorFile(path string) (FilesErrors, error) {
	// Stores the errors read from the errors file.
	var failedFiles FilesErrors

	fContent, err := os.ReadFile(path)
	if err != nil {
		return failedFiles, errorutils.CheckError(err)
	}
	// parse to struct
	err = json.Unmarshal(fContent, &failedFiles)
	if err != nil {
		return failedFiles, errorutils.CheckError(err)
	}
	return failedFiles, nil
}

func (e *errorsRetryPhase) initProgressBar() error {
	if e.progressBar == nil {
		return nil
	}

	// Init progress with the number of tasks of errors file handling (fixing previous upload failures)
	filesCount := 0
	for _, path := range e.errorsFilesToHandle {

		failedFiles, err := e.readErrorFile(path)
		if err != nil {
			return err
		}
		filesCount += len(failedFiles.Errors)
	}

	e.progressBar.AddPhase3(int64(filesCount))

	return nil
}

func (e *errorsRetryPhase) phaseDone() error {
	if e.progressBar != nil {
		return e.progressBar.DonePhase(e.phaseId)
	}
	return nil
}
