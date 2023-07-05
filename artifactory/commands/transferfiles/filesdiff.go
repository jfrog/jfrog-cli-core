package transferfiles

import (
	"fmt"
	"path"
	"time"

	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
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
	if f.progressBar != nil {
		f.progressBar.AddPhase2()
	}
	return nil
}

func (f *filesDiffPhase) getPhaseName() string {
	return "Files Diff Handling Phase"
}

func (f *filesDiffPhase) phaseStarted() error {
	f.startTime = time.Now()
	return f.stateManager.AddNewDiffToState(f.startTime)
}

func (f *filesDiffPhase) phaseDone() error {
	// If the phase stopped gracefully, don't mark the phase as completed
	if !f.ShouldStop() {
		if err := f.stateManager.SetFilesDiffHandlingCompleted(); err != nil {
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
	diffRangeStart, diffRangeEnd, err := f.stateManager.GetDiffHandlingRange()
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
			_, err = pcWrapper.chunkBuilderProducerConsumer.AddTaskWithError(diffTimeFrameHandler(timeFrameParams{fromTime: curDiffTimeFrame}), pcWrapper.errorsQueue.AddError)
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
		result, lastPage, err := f.getTimeFrameFilesDiff(fromTimestamp, toTimestamp, paginationI)
		if err != nil {
			return err
		}
		if len(result) == 0 {
			if paginationI == 0 {
				log.Debug("No diffs were found in time frame: '" + fromTimestamp + "' to '" + toTimestamp + "'")
			}
			break
		}
		files := convertResultsToFileRepresentation(result)
		totalSize := 0
		for _, r := range files {
			totalSize += int(r.Size)
		}

		err = f.transferManager.stateManager.IncTotalSizeAndFilesPhase2(int64(len(files)), int64(totalSize))
		if err != nil {
			return err
		}
		storage, _, _, _, err := f.transferManager.stateManager.GetStorageAndFilesRepoPointers(f.phaseId)
		if err != nil {
			return err
		}
		if f.progressBar != nil {
			f.progressBar.phases[f.phaseId].GetTasksProgressBar().SetGeneralProgressTotal(*storage)
		}
		shouldStop, err := uploadByChunks(files, uploadChunkChan, f.phaseBase, delayHelper, errorsChannelMng, pcWrapper)
		if err != nil || shouldStop {
			return err
		}

		if lastPage {
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
		switch result.Type {
		case "folder":
			var pathInRepo string
			if result.Path == "." {
				pathInRepo = result.Name
			} else {
				pathInRepo = path.Join(result.Path, result.Name)
			}
			files = append(files, api.FileRepresentation{
				Repo: result.Repo,
				Path: pathInRepo,
			})
		default:
			files = append(files, api.FileRepresentation{
				Repo: result.Repo,
				Path: result.Path,
				Name: result.Name,
				Size: result.Size,
			})
		}
	}
	return
}

// Get a list of changed files and folders between the input timestamps.
// fromTimestamp - Time in RFC3339 represents the start time
// toTimestamp - Time in RFC3339 represents the end time
// paginationOffset - Requested page
// Return values:
// result - The list of changed files and folders between the input timestamps
// lastPage - True if we are in the last AQL page and it is not needed to run another AQL requests
// err - The error, if any occurred
func (f *filesDiffPhase) getTimeFrameFilesDiff(fromTimestamp, toTimestamp string, paginationOffset int) (result []servicesUtils.ResultItem, lastPage bool, err error) {
	var timeFrameFilesDiff *servicesUtils.AqlSearchResult
	if f.packageType == docker {
		// Handle Docker repositories.
		timeFrameFilesDiff, err = f.getDockerTimeFrameFilesDiff(fromTimestamp, toTimestamp, paginationOffset)
	} else {
		// Handle all other (non docker) repository types.
		timeFrameFilesDiff, err = f.getNonDockerTimeFrameFilesDiff(fromTimestamp, toTimestamp, paginationOffset)
	}
	if err != nil {
		return []servicesUtils.ResultItem{}, true, err
	}
	lastPage = len(timeFrameFilesDiff.Results) < AqlPaginationLimit
	result, err = f.locallyGeneratedFilter.FilterLocallyGenerated(timeFrameFilesDiff.Results)
	return
}

func (f *filesDiffPhase) getNonDockerTimeFrameFilesDiff(fromTimestamp, toTimestamp string, paginationOffset int) (aqlResult *servicesUtils.AqlSearchResult, err error) {
	query := generateDiffAqlQuery(f.repoKey, fromTimestamp, toTimestamp, paginationOffset)
	return runAql(f.context, f.srcRtDetails, query)
}

// We handle docker repositories differently from other repositories.
// The reason is as follows. If a docker layer already exists in Artifactory, and we try to upload it again to a different repository or a different path,
// its creation time will be the time of the initial upload, and not the latest one. This means that the layer will not be picked up and transferred as part of Phase 2.
// To avoid this situation, we look for all "manifest.json" and "list.manifest.json" files, and for each "manifest.json", we will run a search AQL in Artifactory
// to get all artifacts in its path (that includes the "manifest.json" file itself and all its layouts).
func (f *filesDiffPhase) getDockerTimeFrameFilesDiff(fromTimestamp, toTimestamp string, paginationOffset int) (aqlResult *servicesUtils.AqlSearchResult, err error) {
	// Get all newly created or modified manifest files ("manifest.json" and "list.manifest.json" files)
	query := generateDockerManifestAqlQuery(f.repoKey, fromTimestamp, toTimestamp, paginationOffset)
	manifestFilesResult, err := runAql(f.context, f.srcRtDetails, query)
	if err != nil {
		return
	}
	var result []servicesUtils.ResultItem
	if len(manifestFilesResult.Results) > 0 {
		var manifestPaths []string
		// Add the "list.manifest.json" files to the result, skip "manifest.json" files and save their paths separately.
		for _, file := range manifestFilesResult.Results {
			switch file.Name {
			case "manifest.json":
				manifestPaths = append(manifestPaths, file.Path)
			case "list.manifest.json":
			default:
				err = errorutils.CheckErrorf("unexpected file name returned from AQL query. Expecting either 'manifest.json' or 'list.manifest.json'. Received '%s'.", file.Name)
				return
			}
		}
		if manifestPaths != nil {
			// Get all content of Artifactory folders containing a "manifest.json" file.
			query = generateGetDirContentAqlQuery(f.repoKey, manifestPaths)
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
	query := fmt.Sprintf(`items.find({"$and":[{"modified":{"$gte":"%s"}},{"modified":{"$lt":"%s"}},{"repo":"%s","type":"any"}]})`, fromTimestamp, toTimestamp, repoKey)
	query += `.include("repo","path","name","type","modified","size")`
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
	query += `]}).include("name","repo","path","sha256","size","type","modified","created")`
	return query
}

// This function generates an AQL that searches for all files named "manifest.json" and "list.manifest.json" in a specific repository.
func generateDockerManifestAqlQuery(repoKey, fromTimestamp, toTimestamp string, paginationOffset int) string {
	query := `items.find({"$and":`
	query += fmt.Sprintf(`[{"repo":"%s"},{"modified":{"$gte":"%s"}},{"modified":{"$lt":"%s"}},{"$or":[{"name":"manifest.json"},{"name":"list.manifest.json"}]}`, repoKey, fromTimestamp, toTimestamp)
	query += `]}).include("repo","path","name","type","modified")`
	query += fmt.Sprintf(`.sort({"$asc":["modified"]}).offset(%d).limit(%d)`, paginationOffset*AqlPaginationLimit, AqlPaginationLimit)
	return query
}
