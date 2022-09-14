package transferfiles

import (
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

// Consumes errors files with upload failures from cache and tries to upload these files again.
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

func (e *errorsRetryPhase) handleErrorsFiles(pcWrapper *producerConsumerWrapper, uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
	for _, path := range e.errorsFilesToHandle {
		if ShouldStop(&e.phaseBase, &delayHelper, errorsChannelMng) {
			return nil
		}
		log.Debug("Handling errors file: '" + path + "'")

		// read and parse file
		failedFiles, err := readErrorFile(path)
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

		if e.progressBar != nil {
			err = e.progressBar.IncrementPhase(e.phaseId)
			if err != nil {
				return err
			}
		}
		log.Debug("Done handling errors file: '" + path + "'. Deleting it...")
	}
	return nil
}

// phase interface

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
		//var failedFiles FilesErrors
		failedFiles, err := readErrorFile(path)
		if err != nil {
			return err
		}
		filesCount += len(failedFiles.Errors)
	}

	log.Debug("Starting Error-Retry Phase (3) for '", filesCount, "' files")
	e.progressBar.AddPhase3(int64(filesCount))

	return nil
}

func (e *errorsRetryPhase) phaseDone() error {
	if e.progressBar != nil {
		return e.progressBar.DonePhase(e.phaseId)
	}
	return nil
}
