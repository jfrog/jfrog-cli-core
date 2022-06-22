package transferdata

import (
	"fmt"
	"github.com/jfrog/gofrog/parallel"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	artifactoryUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"time"
)

const searchTimeFramesMinutes = 15

type filesDiffPhase struct {
	repoKey                   string
	checkExistenceInFilestore bool
	startTime                 time.Time
	srcUpService              *srcUserPluginService
	srcRtDetails              *coreConfig.ServerDetails
	targetRtDetails           *coreConfig.ServerDetails
}

func (f *filesDiffPhase) getPhaseName() string {
	return "Files Diff Handling Phase"
}

func (f *filesDiffPhase) phaseStarted() error {
	// TODO notify progress
	f.startTime = time.Now()
	err := addNewDiffToState(f.repoKey, f.startTime)
	if err != nil {
		return err
	}
	return setFilesDiffHandlingStarted(f.repoKey, f.startTime)
}

func (f *filesDiffPhase) phaseDone() error {
	// TODO notify progress
	return setFilesDiffHandlingCompleted(f.repoKey)
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
	diffRangeStart, diffRangeEnd, err := getDiffHandlingRange(f.repoKey)
	if err != nil {
		return err
	}

	producerConsumer := parallel.NewRunner(getThreads(), tasksMaxCapacity, false)
	expectedChan := make(chan int, 1)
	errorsQueue := clientUtils.NewErrorsQueue(1)
	uploadTokensChan := make(chan string, tasksMaxCapacity)

	go func() {
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
			_, _ = producerConsumer.AddTaskWithError(diffTimeFrameHandler(timeFrameParams{repoKey: f.repoKey, fromTime: curDiffTimeFrame}), errorsQueue.AddError)
			curDiffTimeFrame = curDiffTimeFrame.Add(searchTimeFramesMinutes * time.Minute)
		}
	}()

	doneChan := make(chan bool, 1)

	var pollingError error
	go func() {
		pollingError = pollUploads(f.srcUpService, uploadTokensChan, doneChan)
	}()

	var runnerErr error
	go func() {
		runnerErr = producerConsumer.DoneWhenAllIdle(15)
		doneChan <- true
	}()
	// Blocked until finish consuming
	producerConsumer.Run()

	if runnerErr != nil {
		return runnerErr
	}
	if pollingError != nil {
		return pollingError
	}

	return errorsQueue.GetError()
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

	curUploadChunk := UploadChunk{
		TargetAuth:                createTargetAuth(f.targetRtDetails),
		CheckExistenceInFilestore: f.checkExistenceInFilestore,
	}

	for _, item := range result.Results {
		if item.Name == "." {
			continue
		}
		curUploadChunk.appendUploadCandidate(item.Repo, item.Path, item.Name)
		if len(curUploadChunk.UploadCandidates) == uploadChunkSize {
			err := uploadChunkWhenPossible(f.srcUpService, curUploadChunk, pcDetails.uploadTokensChan)
			if err != nil {
				return err
			}
			// Empty the uploaded chunk.
			curUploadChunk.UploadCandidates = []FileRepresentation{}
		}
	}
	// Chunk didn't reach full size. Upload the remaining files.
	if len(curUploadChunk.UploadCandidates) > 0 {
		return uploadChunkWhenPossible(f.srcUpService, curUploadChunk, pcDetails.uploadTokensChan)
	}
	return nil
}

func (f *filesDiffPhase) getTimeFrameFilesDiff(repoKey, fromTimestamp, toTimestamp string) (result *artifactoryUtils.AqlSearchResult, err error) {
	query := generateDiffAqlQuery(repoKey, fromTimestamp, toTimestamp)
	return runAql(f.srcRtDetails, query)
}

func generateDiffAqlQuery(repoKey, fromTimestamp, toTimestamp string) string {
	items := fmt.Sprintf(`items.find({"type":"file","modified":{"$gte":"%s"},"modified":{"$lt":"%s"},"$or":[{"$and":[{"repo":"%s","path":{"$match":"*"},"name":{"$match":"*"}}]}]})`, repoKey, fromTimestamp, toTimestamp)
	items += `.include("repo","path","name")`
	return items
}
