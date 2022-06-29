package transferfiles

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/gofrog/parallel"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	artifactoryUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"math"
	"os"
	"sync"
	"time"
)

// When handling files diff, we split the whole time range being handled by searchTimeFramesMinutes in order to receive smaller results from the AQLs.
const searchTimeFramesMinutes = 15

// Manages the phase of fixing files diffs (files that were created/modified after they were transferred),
// and handling upload failures that were collected during previous runs and phases.
type filesDiffPhase struct {
	phaseBase
	errorsFilesToHandle []string
}

func (f *filesDiffPhase) getSourceDetails() *coreConfig.ServerDetails {
	return f.srcRtDetails
}

func (f *filesDiffPhase) setProgressBar(progressbar *progressbar.TransferProgressMng) {
	f.progressBar = progressbar
}

func (f *filesDiffPhase) initProgressBar() error {
	if f.progressBar == nil {
		return nil
	}
	diffRangeStart, diffRangeEnd, err := getDiffHandlingRange(f.repoKey)
	if err != nil {
		return err
	}

	// Init progress with the number of tasks.
	// Task is either an errors file handling (fixing previous upload failures),
	// or a time frame diff handling (a split of the time range on which this phase fixes files diffs).
	totalLength := diffRangeEnd.Sub(diffRangeStart)
	aqlNum := math.Ceil(totalLength.Minutes() / searchTimeFramesMinutes)
	f.progressBar.AddPhase2(int64(len(f.errorsFilesToHandle)) + int64(aqlNum))
	return nil
}

func (f *filesDiffPhase) getPhaseName() string {
	return "Files Diff Handling Phase"
}

func (f *filesDiffPhase) getPhaseId() int {
	return 1
}

func (f *filesDiffPhase) phaseStarted() error {
	f.startTime = time.Now()
	err := addNewDiffToState(f.repoKey, f.startTime)
	if err != nil {
		return err
	}
	err = setFilesDiffHandlingStarted(f.repoKey, f.startTime)
	if err != nil {
		return err
	}
	// Find all errors files the phase will handle.
	f.errorsFilesToHandle, err = getErrorsFiles(f.repoKey, true)
	return err
}

func (f *filesDiffPhase) phaseDone() error {
	err := setFilesDiffHandlingCompleted(f.repoKey)
	if err != nil {
		return err
	}
	return f.progressBar.DonePhase(f.getPhaseId())
}

func (f *filesDiffPhase) shouldSkipPhase() (bool, error) {
	return false, nil
}

func (f *filesDiffPhase) shouldCheckExistenceInFilestore(shouldCheck bool) {
	f.checkExistenceInFilestore = shouldCheck
}

func (f *filesDiffPhase) setSrcUserPluginService(service *srcUserPluginService) {
	f.srcUpService = service
}

func (f *filesDiffPhase) setSourceDetails(details *coreConfig.ServerDetails) {
	f.srcRtDetails = details
}

func (f *filesDiffPhase) setTargetDetails(details *coreConfig.ServerDetails) {
	f.targetRtDetails = details
}

func (f *filesDiffPhase) run() error {
	f.handlePreviousUploadFailures()
	return f.handleDiffTimeFrames()
}

// Split the time range of fixing files diffs into smaller time frames and handle them separately with smaller AQLs.
// Diffs found (files created/modifies) are uploaded in chunks, then polled on for status.
func (f *filesDiffPhase) handleDiffTimeFrames() error {
	log.Info("Starting to handle files diffs...")
	diffRangeStart, diffRangeEnd, err := getDiffHandlingRange(f.repoKey)
	if err != nil {
		return err
	}

	producerConsumer := parallel.NewRunner(getThreads(), tasksMaxCapacity, false)
	expectedChan := make(chan int, 1)
	errorsQueue := clientUtils.NewErrorsQueue(1)
	uploadTokensChan := make(chan string, tasksMaxCapacity)
	var runWaitGroup sync.WaitGroup
	// Done channel notifies the polling go routines that no more tasks are expected.
	doneChan := make(chan bool, 2)

	runWaitGroup.Add(1)
	go func() {
		defer runWaitGroup.Done()
		periodicallyUpdateThreads(producerConsumer, doneChan)
	}()

	runWaitGroup.Add(1)
	go func() {
		defer runWaitGroup.Done()
		pcDetails := producerConsumerDetails{
			producerConsumer: producerConsumer,
			expectedChan:     expectedChan,
			errorsQueue:      errorsQueue,
			uploadTokensChan: uploadTokensChan,
		}

		// Create tasks to handle files diffs in time frames of searchTimeFramesMinutes.
		curDiffTimeFrame := diffRangeStart
		for diffRangeEnd.Sub(curDiffTimeFrame) > 0 {
			diffTimeFrameHandler := f.createDiffTimeFrameHandlerFunc(pcDetails)
			_, err = producerConsumer.AddTaskWithError(diffTimeFrameHandler(timeFrameParams{repoKey: f.repoKey, fromTime: curDiffTimeFrame}), errorsQueue.AddError)
			if err != nil {
				log.Error("error occurred when adding new task to producer consumer:" + err.Error())
			}
			curDiffTimeFrame = curDiffTimeFrame.Add(searchTimeFramesMinutes * time.Minute)
		}
	}()

	runWaitGroup.Add(1)
	var pollingError error
	go func() {
		defer runWaitGroup.Done()
		pollingError = pollUploads(f.srcUpService, uploadTokensChan, doneChan)
	}()

	runWaitGroup.Add(1)
	var runnerErr error
	go func() {
		defer runWaitGroup.Done()
		// When the producer consumer is idle for assumeProducerConsumerDoneWhenIdleForSeconds (not tasks are being handled)
		// the work is assumed to be done.
		runnerErr = producerConsumer.DoneWhenAllIdle(assumeProducerConsumerDoneWhenIdleForSeconds)
		// Notify the other go routines that work is done.
		doneChan <- true
		doneChan <- true
	}()
	// Blocked until finish consuming
	producerConsumer.Run()
	runWaitGroup.Wait()

	var returnedError error
	for _, err := range []error{runnerErr, pollingError, errorsQueue.GetError()} {
		if err != nil {
			log.Error(err)
			returnedError = err
		}
	}
	log.Info("Done handling files diffs.")
	return returnedError
}

type diffTimeFrameHandlerFunc func(params timeFrameParams) parallel.TaskFunc

type timeFrameParams struct {
	repoKey  string
	fromTime time.Time
}

func (f *filesDiffPhase) createDiffTimeFrameHandlerFunc(pcDetails producerConsumerDetails) diffTimeFrameHandlerFunc {
	return func(params timeFrameParams) parallel.TaskFunc {
		return func(threadId int) error {
			logMsgPrefix := clientUtils.GetLogMsgPrefix(threadId, false)
			return f.handleTimeFrameFilesDiff(params, logMsgPrefix, pcDetails)
		}
	}
}

func (f *filesDiffPhase) handleTimeFrameFilesDiff(params timeFrameParams, logMsgPrefix string, pcDetails producerConsumerDetails) error {
	fromTimestamp := params.fromTime.Format(time.RFC3339)
	toTimestamp := params.fromTime.Add(searchTimeFramesMinutes * time.Minute).Format(time.RFC3339)
	log.Debug(logMsgPrefix + "Searching time frame: '" + fromTimestamp + "' to '" + toTimestamp + "'")

	result, err := f.getTimeFrameFilesDiff(params.repoKey, fromTimestamp, toTimestamp)
	if err != nil {
		return err
	}

	if len(result.Results) == 0 {
		log.Debug("No diffs were found in time frame: '" + fromTimestamp + "' to '" + toTimestamp + "'")
		return nil
	}

	files := convertResultsToFileRepresentation(result.Results)
	err = f.uploadByChunks(files, pcDetails.uploadTokensChan)
	if err != nil {
		return err
	}
	if f.progressBar != nil {
		err = f.progressBar.IncrementPhase(f.getPhaseId())
		if err != nil {
			return err
		}
	}
	log.Debug(logMsgPrefix + "Done handling time frame: '" + fromTimestamp + "' to '" + toTimestamp + "'")
	return nil
}

func convertResultsToFileRepresentation(results []artifactoryUtils.ResultItem) (files []FileRepresentation) {
	for _, result := range results {
		files = append(files, FileRepresentation{
			Repo: result.Repo,
			Path: result.Path,
			Name: result.Name,
		})
	}
	return
}

// Collects files in chunks of size uploadChunkSize and uploads them whenever possible (the amount of chunks uploaded is limited by the number of threads).
// If not all files were checksum deployed, an uuid token is returned and is being polled on for status.
func (f *filesDiffPhase) uploadByChunks(files []FileRepresentation, uploadTokensChan chan string) (err error) {
	curUploadChunk := UploadChunk{
		TargetAuth:                createTargetAuth(f.targetRtDetails),
		CheckExistenceInFilestore: f.checkExistenceInFilestore,
	}

	for _, item := range files {
		curUploadChunk.appendUploadCandidate(item.Repo, item.Path, item.Name)
		if len(curUploadChunk.UploadCandidates) == uploadChunkSize {
			err = uploadChunkWhenPossible(f.srcUpService, curUploadChunk, uploadTokensChan)
			if err != nil {
				return err
			}
			// Empty the uploaded chunk.
			curUploadChunk.UploadCandidates = []FileRepresentation{}
		}
	}
	// Chunk didn't reach full size. Upload the remaining files.
	if len(curUploadChunk.UploadCandidates) > 0 {
		return uploadChunkWhenPossible(f.srcUpService, curUploadChunk, uploadTokensChan)
	}
	return nil
}

func (f *filesDiffPhase) getTimeFrameFilesDiff(repoKey, fromTimestamp, toTimestamp string) (result *artifactoryUtils.AqlSearchResult, err error) {
	query := generateDiffAqlQuery(repoKey, fromTimestamp, toTimestamp)
	return runAql(f.srcRtDetails, query)
}

func generateDiffAqlQuery(repoKey, fromTimestamp, toTimestamp string) string {
	items := fmt.Sprintf(`items.find({"type":"file","modified":{"$gte":"%s"},"modified":{"$lt":"%s"},"$or":[{"$and":[{"repo":"%s","path":{"$match":"*"},"name":{"$match":"*"}}]}]})`, fromTimestamp, toTimestamp, repoKey)
	items += `.include("repo","path","name")`
	return items
}

// Consumes errors files with upload failures from cache and tries to upload these files again.
// Does so by creating and uploading by chunks, and polling on status.
// Consumed errors files are deleted, new failures are written to new files.
func (f *filesDiffPhase) handlePreviousUploadFailures() {
	log.Info("Starting to handle previous upload failures...")
	uploadTokensChan := make(chan string, tasksMaxCapacity)
	var runWaitGroup sync.WaitGroup
	// Done channel notifies the polling go routines that no more tasks are expected.
	doneChan := make(chan bool, 2)

	runWaitGroup.Add(1)
	go func() {
		defer runWaitGroup.Done()
		periodicallyUpdateThreads(nil, doneChan)
	}()

	runWaitGroup.Add(1)
	var pollingError error
	go func() {
		defer runWaitGroup.Done()
		pollingError = pollUploads(f.srcUpService, uploadTokensChan, doneChan)
	}()

	runWaitGroup.Add(1)
	var runnerErr error
	go func() {
		defer runWaitGroup.Done()
		runnerErr = f.handleErrorsFiles(uploadTokensChan)
		doneChan <- true
		doneChan <- true
	}()

	runWaitGroup.Wait()

	for _, err := range []error{runnerErr, pollingError} {
		if err != nil {
			log.Error(err)
		}
	}
	log.Info("Done handling previous upload failures.")
}

func (f *filesDiffPhase) handleErrorsFiles(uploadTokensChan chan string) error {
	for _, path := range f.errorsFilesToHandle {
		log.Debug("Handling errors file: '" + path + "'")
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var failedFiles FilesErrors
		err = json.Unmarshal(content, &failedFiles)
		if err != nil {
			return errorutils.CheckError(err)
		}

		err = f.uploadByChunks(convertUploadStatusToFileRepresentation(failedFiles.Errors), uploadTokensChan)
		if err != nil {
			return err
		}

		// Remove the file, so it won't be consumed again.
		err = os.Remove(path)
		if err != nil {
			return errorutils.CheckError(err)
		}

		if f.progressBar != nil {
			err = f.progressBar.IncrementPhase(f.getPhaseId())
			if err != nil {
				return err
			}
		}
		log.Debug("Done handling errors file: '" + path + "'. Deleting it...")
	}
	return nil
}

func convertUploadStatusToFileRepresentation(statuses []FileUploadStatusResponse) (files []FileRepresentation) {
	for _, status := range statuses {
		files = append(files, status.FileRepresentation)
	}
	return
}
