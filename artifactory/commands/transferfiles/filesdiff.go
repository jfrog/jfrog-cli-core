package transferfiles

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/gofrog/parallel"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"math"
	"os"
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
	if f.progressBar != nil {
		err = f.progressBar.DonePhase(f.phaseId)
	}
	return err
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

func (f *filesDiffPhase) setRepoSummary(repoSummary servicesUtils.RepositorySummary) {
	f.repoSummary = repoSummary
}

func (f *filesDiffPhase) run() error {
	err := f.handlePreviousUploadFailures()
	if err != nil {
		return err
	}
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

	manager := newTransferManager(f.phaseBase, getDelayUploadComparisonFunctions(f.repoSummary.PackageType))
	action := func(pcDetails producerConsumerDetails, uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng ErrorsChannelMng) error {
		// Create tasks to handle files diffs in time frames of searchTimeFramesMinutes.
		// In case an error occurred while handling delayed artifacts - stop transferring.
		curDiffTimeFrame := diffRangeStart
		for diffRangeEnd.Sub(curDiffTimeFrame) > 0 && !delayHelper.delayedArtifactsChannelMng.shouldStop() {
			diffTimeFrameHandler := f.createDiffTimeFrameHandlerFunc(uploadTokensChan, delayHelper, errorsChannelMng)
			_, err = pcDetails.producerConsumer.AddTaskWithError(diffTimeFrameHandler(timeFrameParams{repoKey: f.repoKey, fromTime: curDiffTimeFrame}), pcDetails.errorsQueue.AddError)
			if err != nil {
				return err
			}
			curDiffTimeFrame = curDiffTimeFrame.Add(searchTimeFramesMinutes * time.Minute)
		}
		return nil
	}
	err = manager.doTransfer(true, action)
	if err == nil {
		log.Info("Done handling files diffs.")
	}
	return err
}

type diffTimeFrameHandlerFunc func(params timeFrameParams) parallel.TaskFunc

type timeFrameParams struct {
	repoKey  string
	fromTime time.Time
}

func (f *filesDiffPhase) createDiffTimeFrameHandlerFunc(uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng ErrorsChannelMng) diffTimeFrameHandlerFunc {
	return func(params timeFrameParams) parallel.TaskFunc {
		return func(threadId int) error {
			logMsgPrefix := clientUtils.GetLogMsgPrefix(threadId, false)
			return f.handleTimeFrameFilesDiff(params, logMsgPrefix, uploadTokensChan, delayHelper, errorsChannelMng)
		}
	}
}

func (f *filesDiffPhase) handleTimeFrameFilesDiff(params timeFrameParams, logMsgPrefix string, uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng ErrorsChannelMng) error {
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
	shouldStop, err := uploadByChunks(files, uploadTokensChan, f.phaseBase, delayHelper, errorsChannelMng)
	if err != nil || shouldStop {
		return err
	}
	if f.progressBar != nil {
		err = f.progressBar.IncrementPhase(f.phaseId)
		if err != nil {
			return err
		}
	}
	log.Debug(logMsgPrefix + "Done handling time frame: '" + fromTimestamp + "' to '" + toTimestamp + "'")
	return nil
}

func convertResultsToFileRepresentation(results []servicesUtils.ResultItem) (files []FileRepresentation) {
	for _, result := range results {
		files = append(files, FileRepresentation{
			Repo: result.Repo,
			Path: result.Path,
			Name: result.Name,
		})
	}
	return
}

func (f *filesDiffPhase) getTimeFrameFilesDiff(repoKey, fromTimestamp, toTimestamp string) (result *servicesUtils.AqlSearchResult, err error) {
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
func (f *filesDiffPhase) handlePreviousUploadFailures() error {
	log.Info("Starting to handle previous upload failures...")
	manager := newTransferManager(f.phaseBase, getDelayUploadComparisonFunctions(f.repoSummary.PackageType))
	action := func(optionalPcDetails producerConsumerDetails, uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng ErrorsChannelMng) error {
		// In case an error occurred while handling delayed artifacts - stop transferring.
		if delayHelper.delayedArtifactsChannelMng.shouldStop() {
			return nil
		}
		return f.handleErrorsFiles(uploadTokensChan, delayHelper, errorsChannelMng)
	}
	err := manager.doTransfer(false, action)
	if err == nil {
		log.Info("Done handling previous upload failures.")
	}
	return err
}

func (f *filesDiffPhase) handleErrorsFiles(uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng ErrorsChannelMng) error {
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

		shouldStop, err := uploadByChunks(convertUploadStatusToFileRepresentation(failedFiles.Errors), uploadTokensChan, f.phaseBase, delayHelper, errorsChannelMng)
		if err != nil || shouldStop {
			return err
		}

		// Remove the file, so it won't be consumed again.
		err = os.Remove(path)
		if err != nil {
			return errorutils.CheckError(err)
		}

		if f.progressBar != nil {
			err = f.progressBar.IncrementPhase(f.phaseId)
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
