package transferfiles

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"math"
	"time"

	"github.com/jfrog/gofrog/parallel"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// When handling files diff, we split the whole time range being handled by searchTimeFramesMinutes in order to receive smaller results from the AQLs.
const searchTimeFramesMinutes = 15

// Manages the phase of fixing files diffs (files that were created/modified after they were transferred),
// and handling transfer failures that have been collected during previous runs and phases.
type filesDiffPhase struct {
	phaseBase
}

func (f *filesDiffPhase) initProgressBar() error {
	if f.progressBar == nil {
		return nil
	}
	diffRangeStart, diffRangeEnd, err := f.stateManager.GetDiffHandlingRange(f.repoKey)
	if err != nil {
		return err
	}

	// Init progress with the number of tasks.
	// Task is either an errors file handling (fixing previous upload failures),
	// or a time frame diff handling (a split of the time range on which this phase fixes files diffs).
	totalLength := diffRangeEnd.Sub(diffRangeStart)
	aqlNum := math.Ceil(totalLength.Minutes() / searchTimeFramesMinutes)
	f.progressBar.AddPhase2(int64(aqlNum))
	return nil
}

func (f *filesDiffPhase) getPhaseName() string {
	return "Files Diff Handling Phase"
}

func (f *filesDiffPhase) phaseStarted() error {
	f.startTime = time.Now()
	return f.stateManager.AddNewDiffToState(f.repoKey, f.startTime)
}

func (f *filesDiffPhase) phaseDone() error {
	// If the phase stopped gracefully, don't mark the phase as completed
	if !f.ShouldStop() {
		if err := f.stateManager.SetFilesDiffHandlingCompleted(f.repoKey); err != nil {
			return err
		}
	}

	if f.progressBar != nil {
		return f.progressBar.DonePhase(f.phaseId)
	}
	return nil
}

func (f *filesDiffPhase) shouldSkipPhase() (bool, error) {
	return false, nil
}

func (f *filesDiffPhase) run() error {
	return f.handleDiffTimeFrames()
}

// Split the time range of fixing files diffs into smaller time frames and handle them separately with smaller AQLs.
// Diffs found (files created/modifies) are uploaded in chunks, then polled on for status.
func (f *filesDiffPhase) handleDiffTimeFrames() error {
	log.Info("Starting to handle files diffs...")
	diffRangeStart, diffRangeEnd, err := f.stateManager.GetDiffHandlingRange(f.repoKey)
	if err != nil {
		return err
	}

	f.transferManager = newTransferManager(f.phaseBase, getDelayUploadComparisonFunctions(f.repoSummary.PackageType))
	action := func(pcWrapper *producerConsumerWrapper, uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
		// Create tasks to handle files diffs in time frames of searchTimeFramesMinutes.
		// In case an error occurred while handling errors/delayed artifacts files - stop transferring.
		curDiffTimeFrame := diffRangeStart
		for diffRangeEnd.Sub(curDiffTimeFrame) > 0 && !ShouldStop(&f.phaseBase, &delayHelper, errorsChannelMng) {
			diffTimeFrameHandler := f.createDiffTimeFrameHandlerFunc(pcWrapper, uploadChunkChan, delayHelper, errorsChannelMng)
			_, err = pcWrapper.chunkBuilderProducerConsumer.AddTaskWithError(diffTimeFrameHandler(timeFrameParams{repoKey: f.repoKey, fromTime: curDiffTimeFrame}), pcWrapper.errorsQueue.AddError)
			if err != nil {
				return err
			}
			curDiffTimeFrame = curDiffTimeFrame.Add(searchTimeFramesMinutes * time.Minute)
		}
		return nil
	}
	delayAction := consumeDelayFilesIfNoErrors
	err = f.transferManager.doTransferWithProducerConsumer(action, delayAction)
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

func (f *filesDiffPhase) createDiffTimeFrameHandlerFunc(pcWrapper *producerConsumerWrapper, uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) diffTimeFrameHandlerFunc {
	return func(params timeFrameParams) parallel.TaskFunc {
		return func(threadId int) error {
			logMsgPrefix := clientUtils.GetLogMsgPrefix(threadId, false)
			return f.handleTimeFrameFilesDiff(pcWrapper, params, logMsgPrefix, uploadChunkChan, delayHelper, errorsChannelMng)
		}
	}
}

func (f *filesDiffPhase) handleTimeFrameFilesDiff(pcWrapper *producerConsumerWrapper, params timeFrameParams, logMsgPrefix string, uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
	fromTimestamp := params.fromTime.Format(time.RFC3339)
	toTimestamp := params.fromTime.Add(searchTimeFramesMinutes * time.Minute).Format(time.RFC3339)
	log.Debug(logMsgPrefix + "Searching time frame: '" + fromTimestamp + "' to '" + toTimestamp + "'")

	paginationI := 0
	for {
		result, err := f.getTimeFrameFilesDiff(params.repoKey, fromTimestamp, toTimestamp, paginationI)
		if err != nil {
			return err
		}

		if len(result.Results) == 0 {
			if paginationI == 0 {
				log.Debug("No diffs were found in time frame: '" + fromTimestamp + "' to '" + toTimestamp + "'")
			}
			break
		}

		files := convertResultsToFileRepresentation(result.Results)
		shouldStop, err := uploadByChunks(files, uploadChunkChan, f.phaseBase, delayHelper, errorsChannelMng, pcWrapper)
		if err != nil || shouldStop {
			return err
		}

		if len(result.Results) < AqlPaginationLimit {
			break
		}
		paginationI++
	}

	if f.progressBar != nil {
		err := f.progressBar.IncrementPhase(f.phaseId)
		if err != nil {
			return err
		}
	}
	log.Debug(logMsgPrefix + "Done handling time frame: '" + fromTimestamp + "' to '" + toTimestamp + "'")
	return nil
}

func convertResultsToFileRepresentation(results []servicesUtils.ResultItem) (files []api.FileRepresentation) {
	for _, result := range results {
		files = append(files, api.FileRepresentation{
			Repo: result.Repo,
			Path: result.Path,
			Name: result.Name,
		})
	}
	return
}

func (f *filesDiffPhase) getTimeFrameFilesDiff(repoKey, fromTimestamp, toTimestamp string, paginationOffset int) (result *servicesUtils.AqlSearchResult, err error) {
	// Handle Docker repositories.
	if f.packageType == docker {
		return f.getDockerTimeFrameFilesDiff(repoKey, fromTimestamp, toTimestamp, paginationOffset)
	}
	// Handle all other (non docker) repository types.
	return f.getNonDockerTimeFrameFilesDiff(repoKey, fromTimestamp, toTimestamp, paginationOffset)
}

func (f *filesDiffPhase) getNonDockerTimeFrameFilesDiff(repoKey, fromTimestamp, toTimestamp string, paginationOffset int) (aqlResult *servicesUtils.AqlSearchResult, err error) {
	query := generateDiffAqlQuery(repoKey, fromTimestamp, toTimestamp, paginationOffset)
	return runAql(f.context, f.srcRtDetails, query)
}

// We handle docker repositories differently from other repositories.
// The reason is as follows. If a docker layer already exists in Artifactory, and we try to upload it again to a different repository or a different path,
// its creation time will be the time of the initial upload, and not the latest one. This means that the layer will not be picked up and transferred as part of Phase 2.
// To avoid this situation, we look for all "manifest.json" and "list.manifest.json" files, and for each "manifest.json", we will run a search AQL in Artifactory
// to get all artifacts in its path (that includes the "manifest.json" file itself and all its layouts).
func (f *filesDiffPhase) getDockerTimeFrameFilesDiff(repoKey, fromTimestamp, toTimestamp string, paginationOffset int) (aqlResult *servicesUtils.AqlSearchResult, err error) {
	// Get all newly created or modified manifest files ("manifest.json" and "list.manifest.json" files)
	query := generateDockerManifestAqlQuery(repoKey, fromTimestamp, toTimestamp, paginationOffset)
	manifestFilesResult, err := runAql(f.context, f.srcRtDetails, query)
	if err != nil {
		return
	}
	var result []servicesUtils.ResultItem
	if len(manifestFilesResult.Results) > 0 {
		var manifestPaths []string
		// Add the "list.manifest.json" files to the result, skip "manifest.json" files and save their paths separately.
		for _, file := range manifestFilesResult.Results {
			if file.Name == "manifest.json" {
				manifestPaths = append(manifestPaths, file.Path)
			} else if file.Name == "list.manifest.json" {
				result = append(result, file)
			} else {
				err = errorutils.CheckErrorf("unexpected file name returned from AQL query. Expecting either 'manifest.json' or 'list.manifest.json'. Received '%s'.", file.Name)
				return
			}
		}
		if manifestPaths != nil {
			// Get all content of Artifactory folders containing a "manifest.json" file.
			query = generateGetDirContentAqlQuery(repoKey, manifestPaths)
			var pathsResult *servicesUtils.AqlSearchResult
			pathsResult, err = runAql(f.context, f.srcRtDetails, query)
			if err != nil {
				return
			}
			// Merge "list.manifest.json" files with all other files.
			result = append(result, pathsResult.Results...)
		}
	}
	aqlResult = &servicesUtils.AqlSearchResult{}
	aqlResult.Results = result
	return
}

func generateDiffAqlQuery(repoKey, fromTimestamp, toTimestamp string, paginationOffset int) string {
	query := fmt.Sprintf(`items.find({"$and":[{"modified":{"$gte":"%s"}},{"modified":{"$lt":"%s"}},{"repo":"%s","path":{"$match":"*"},"name":{"$match":"*"}}]})`, fromTimestamp, toTimestamp, repoKey)
	query += `.include("repo","path","name","modified")`
	query += fmt.Sprintf(`.sort({"$asc":["modified"]}).offset(%d).limit(%d)`, paginationOffset*AqlPaginationLimit, AqlPaginationLimit)
	return query
}

// This function generates an AQL that searches for all the content in the list of provided Artifactory paths.
func generateGetDirContentAqlQuery(repoKey string, paths []string) string {
	query := `items.find({"$or":[`
	for i, path := range paths {
		query += fmt.Sprintf(`{"$and":[{"repo":"%s","path":{"$match":"%s"},"name":{"$match":"*"}}]}`, repoKey, path)
		// Add comma for all paths except for the last one.
		if i != len(paths)-1 {
			query += ","
		}
	}
	query += `]}).include("name","repo","path","actual_md5","actual_sha1","sha256","size","type","modified","created","property")`
	return query
}

// This function generates an AQL that searches for all files named "manifest.json" and "list.manifest.json" in a specific repository.
func generateDockerManifestAqlQuery(repoKey, fromTimestamp, toTimestamp string, paginationOffset int) string {
	query := `items.find({"$and":`
	query += fmt.Sprintf(`[{"$and":[{"repo":"%s"}]},{"modified":{"$gte":"%s"}},{"modified":{"$lt":"%s"}},{"$or":[{"name":{"$match":"manifest.json"}},{"name":{"$match":"list.manifest.json"}}]}`, repoKey, fromTimestamp, toTimestamp)
	query += `]}).include("repo","path","name","modified")`
	query += fmt.Sprintf(`.sort({"$asc":["modified"]}).offset(%d).limit(%d)`, paginationOffset*AqlPaginationLimit, AqlPaginationLimit)
	return query
}
