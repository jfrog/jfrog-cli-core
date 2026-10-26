package transferfiles

import (
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/jfrog/gofrog/safeconvert"

	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	coreutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/reposnapshot"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Manages the phase of performing a full transfer of the repository.
// This phase is only executed once per repository if its completed.
// Transfer is performed by treating every folder as a task, and searching for it's content in a flat AQL.
// New folders found are handled as a separate task, and files are transferred directly per file.
type fullTransferPhase struct {
	phaseBase
	transferManager              *transferManager
	completedFolderFilterWarning sync.Once
}

func (m *fullTransferPhase) initProgressBar() error {
	if m.progressBar == nil {
		return nil
	}
	skip, err := m.shouldSkipPhase()
	if err != nil {
		return err
	}
	m.progressBar.AddPhase1(skip)
	return nil
}

func (m *fullTransferPhase) getPhaseName() string {
	return "Full Transfer Phase"
}

func (m *fullTransferPhase) phaseStarted() error {
	m.startTime = time.Now()
	return m.stateManager.SetRepoFullTransferStarted(m.startTime)
}

func (m *fullTransferPhase) phaseDone() error {
	// If the phase stopped gracefully, don't mark the phase as completed or delete snapshots.
	if !m.ShouldStop() {
		if err := m.handleSuccessfulTransfer(); err != nil {
			return err
		}
	}

	if m.progressBar != nil {
		return m.progressBar.DonePhase(m.phaseId)
	}
	return nil
}

// Marks the phase as completed and deletes snapshots.
// Should ONLY be called if phase ended SUCCESSFULLY (not interrupted / stopped).
func (m *fullTransferPhase) handleSuccessfulTransfer() error {
	if err := m.stateManager.SetRepoFullTransferCompleted(); err != nil {
		return err
	}
	// Disable repo transfer snapshot since it is not tracked by the following phases we are not handling a full transfer.
	m.stateManager.DisableRepoTransferSnapshot()
	snapshotDir, err := state.GetJfrogTransferRepoSnapshotDir(m.repoKey)
	if err != nil {
		return err
	}
	return fileutils.RemoveDirContents(snapshotDir)
}

func (m *fullTransferPhase) shouldSkipPhase() (bool, error) {
	repoTransferred, err := m.stateManager.IsRepoTransferred()
	if err != nil || !repoTransferred {
		return false, err
	}
	m.skipPhase()
	return true, nil
}

func (m *fullTransferPhase) skipPhase() {
	// Init progress bar as "done" with 0 tasks.
	if m.progressBar != nil {
		m.progressBar.AddPhase1(true)
	}
}

func (m *fullTransferPhase) run() error {
	m.transferManager = newTransferManager(m.phaseBase, getDelayUploadComparisonFunctions(m.repoSummary.PackageType))

	// If include patterns are provided, use AQL-based direct query instead of folder traversal
	if len(m.includeFilesPatterns) > 0 {
		return m.runWithAqlPatternFiltering()
	}

	// Default: use folder traversal
	return m.runWithFolderTraversal()
}

// runWithFolderTraversal uses the traditional folder-by-folder traversal approach.
// This is the default behavior when no include patterns are specified.
func (m *fullTransferPhase) runWithFolderTraversal() error {
	action := func(pcWrapper *producerConsumerWrapper, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
		if ShouldStop(&m.phaseBase, &delayHelper, errorsChannelMng) {
			return nil
		}

		// Get the directory's node from the snapshot manager, and use information from previous transfer attempts if such exists.
		node, done, err := m.getAndHandleDirectoryNode(".")
		if err != nil || done {
			return err
		}

		folderHandler := m.createFolderFullTransferHandlerFunc(node, pcWrapper, delayHelper, errorsChannelMng)
		_, err = pcWrapper.chunkBuilderProducerConsumer.AddTaskWithError(folderHandler(folderParams{relativePath: "."}), pcWrapper.errorsQueue.AddError)
		return err
	}
	delayAction := func(phase phaseBase, addedDelayFiles []string) error {
		// Disable repo transfer snapshot as it is not used for delayed files.
		if err := m.stateManager.SaveStateAndSnapshots(); err != nil {
			return err
		}
		m.stateManager.DisableRepoTransferSnapshot()
		return consumeDelayFilesIfNoErrors(phase, addedDelayFiles)
	}
	return m.transferManager.doTransferWithProducerConsumer(action, delayAction)
}

// runWithAqlPatternFiltering uses a direct AQL query to fetch all files matching the include patterns.
// This is more efficient than folder traversal when filtering is needed.
func (m *fullTransferPhase) runWithAqlPatternFiltering() error {
	log.Info("Using AQL-based pattern filtering for include patterns:", m.includeFilesPatterns)

	action := func(pcWrapper *producerConsumerWrapper, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
		if ShouldStop(&m.phaseBase, &delayHelper, errorsChannelMng) {
			return nil
		}

		// Fetch all matching files using AQL with pattern filtering
		paginationOffset := 0
		for {
			if ShouldStop(&m.phaseBase, &delayHelper, errorsChannelMng) {
				return nil
			}

			result, lastPage, err := m.getPatternMatchingFiles(paginationOffset)
			if err != nil {
				return err
			}

			if len(result) == 0 {
				if paginationOffset == 0 {
					log.Info("No files found matching the include patterns")
					m.warnIfRepoAppearsBlackedOut("the include-pattern AQL query")
				}
				break
			}

			// Convert results to file representations and upload
			files := convertResultsToFileRepresentation(result)
			shouldStop, err := transferFiles(files, m.phaseBase, delayHelper, errorsChannelMng, pcWrapper)
			if err != nil || shouldStop {
				return err
			}

			if lastPage {
				break
			}
			paginationOffset++
		}
		return nil
	}

	delayAction := func(phase phaseBase, addedDelayFiles []string) error {
		// Disable repo transfer snapshot as it is not used for delayed files.
		if err := m.stateManager.SaveStateAndSnapshots(); err != nil {
			return err
		}
		m.stateManager.DisableRepoTransferSnapshot()
		return consumeDelayFilesIfNoErrors(phase, addedDelayFiles)
	}

	return m.transferManager.doTransferWithProducerConsumer(action, delayAction)
}

// getPatternMatchingFiles fetches files from source Artifactory using AQL with pattern filtering.
func (m *fullTransferPhase) getPatternMatchingFiles(paginationOffset int) (result []servicesUtils.ResultItem, lastPage bool, err error) {
	query := generatePatternBasedAqlQuery(m.repoKey, m.includeFilesPatterns, paginationOffset, m.disabledDistinctiveAql, m.timestampFilter)
	aqlResults, err := runAql(m.context, m.srcRtDetails, query)
	if err != nil {
		return []servicesUtils.ResultItem{}, false, err
	}

	lastPage = len(aqlResults.Results) < AqlPaginationLimit
	result, err = m.locallyGeneratedFilter.FilterLocallyGenerated(aqlResults.Results, m.packageType)
	return
}

// warnIfRepoAppearsBlackedOut logs a warning when the repository's own summary (fetched once,
// upfront, via GetRepoSummary at repo-transfer start) reports it holds files, but a query that
// should have covered the whole repository (queryDescription) returned zero results. Artifactory's
// AQL endpoint silently filters out items the querying identity can't read, returning an empty
// result set (HTTP 200) rather than an error - which looks identical to a genuinely empty
// repository. This does not fail or retry the transfer (an empty repo, or a repo whose files were
// all legitimately deleted since the summary was fetched, are both real possibilities too); it
// only surfaces the ambiguity so operators can audit a suspicious zero-file result instead of
// silently trusting it.
func (m *fullTransferPhase) warnIfRepoAppearsBlackedOut(queryDescription string) {
	filesCount, err := coreutils.GetFilesCountFromRepositorySummary(&m.repoSummary)
	if err != nil || filesCount <= 0 {
		return
	}
	log.Warn(fmt.Sprintf(
		"Repository '%s' reports %d file(s) in its summary, but %s returned zero results. "+
			"Artifactory's AQL endpoint returns an empty result set (not an error) when the transferring "+
			"identity lacks read permission on a repository or path, which is indistinguishable from a "+
			"genuinely empty repository. If this repository unexpectedly transfers 0 files, verify the "+
			"transferring user's read permissions on '%s' before trusting this run's result.",
		m.repoKey, filesCount, queryDescription, m.repoKey))
}

type folderFullTransferHandlerFunc func(params folderParams) parallel.TaskFunc

type folderParams struct {
	relativePath string
}

func (m *fullTransferPhase) createFolderFullTransferHandlerFunc(node *reposnapshot.Node, pcWrapper *producerConsumerWrapper,
	delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) folderFullTransferHandlerFunc {
	return func(params folderParams) parallel.TaskFunc {
		return func(threadId int) error {
			logMsgPrefix := clientUtils.GetLogMsgPrefix(threadId, false)
			return m.transferFolder(node, params, logMsgPrefix, pcWrapper, delayHelper, errorsChannelMng)
		}
	}
}

func (m *fullTransferPhase) transferFolder(node *reposnapshot.Node, params folderParams, logMsgPrefix string, pcWrapper *producerConsumerWrapper,
	delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) (err error) {
	log.Debug(logMsgPrefix+"Handling folder:", path.Join(m.repoKey, params.relativePath))

	// Increment progress number of folders
	if m.progressBar != nil {
		m.progressBar.incNumberOfVisitedFolders()
	}
	if err = m.stateManager.IncVisitedFolders(); err != nil {
		return
	}

	err = m.searchAndHandleFolderContents(params, pcWrapper, delayHelper, errorsChannelMng, node)
	if err != nil {
		return
	}

	// Mark that no more results are expected for the current folder.
	if err = node.MarkDoneExploring(); err != nil {
		return err
	}

	log.Debug(logMsgPrefix+"Done transferring folder:", path.Join(m.repoKey, params.relativePath))
	return
}

func (m *fullTransferPhase) searchAndHandleFolderContents(params folderParams, pcWrapper *producerConsumerWrapper,
	delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng,
	node *reposnapshot.Node) (err error) {
	var result []servicesUtils.ResultItem
	var lastPage bool
	paginationI := 0
	for !lastPage && !ShouldStop(&m.phaseBase, &delayHelper, errorsChannelMng) {
		result, lastPage, err = m.getDirectoryContentAql(params.relativePath, paginationI)
		if err != nil {
			return
		}

		// Transfer folders with properties or empty folders once per directory scan.
		if params.relativePath != "." && paginationI == 0 {
			folder := api.FileRepresentation{Repo: m.repoKey, Path: params.relativePath, NonEmptyDir: len(result) > 0}
			var shouldStop bool
			shouldStop, err = m.transferFolderCandidateIfNeeded(folder, delayHelper, errorsChannelMng, pcWrapper)
			if err != nil || shouldStop {
				return err
			}
		}

		// Empty folder
		if paginationI == 0 && len(result) == 0 {
			if params.relativePath == "." {
				m.warnIfRepoAppearsBlackedOut("the repository root folder listing")
			}
			return
		}

		for _, item := range result {
			if ShouldStop(&m.phaseBase, &delayHelper, errorsChannelMng) {
				return
			}
			if item.Name == "." {
				continue
			}
			switch item.Type {
			case "folder":
				err = m.handleFoundChildFolder(params, pcWrapper, delayHelper, errorsChannelMng, item)
			case "file":
				err = m.handleFoundFile(pcWrapper, delayHelper, errorsChannelMng, node, item)
			}
			if err != nil {
				return
			}
		}

		paginationI++
	}
	return
}

func (m *fullTransferPhase) transferFolderCandidateIfNeeded(folder api.FileRepresentation, delayHelper delayUploadHelper,
	errorsChannelMng *ErrorsChannelMng, pcWrapper *producerConsumerWrapper) (shouldStop bool, err error) {
	if m.buildInfoRepo && folder.Name == "" {
		log.Debug(fmt.Sprintf("Skipping unneeded empty dir '%s' in the build-info repository '%s'", folder.Path, folder.Repo))
		return false, nil
	}
	return transferFiles([]api.FileRepresentation{folder}, m.phaseBase, delayHelper, errorsChannelMng, pcWrapper)
}

func (m *fullTransferPhase) handleFoundChildFolder(params folderParams, pcWrapper *producerConsumerWrapper,
	delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng,
	item servicesUtils.ResultItem) (err error) {
	newRelativePath := getFolderRelativePath(item.Name, params.relativePath)

	// Get the directory's node from the snapshot manager, and use information from previous transfer attempts if such exists.
	node, done, err := m.getAndHandleDirectoryNode(newRelativePath)
	if err != nil || done {
		return err
	}

	folderHandler := m.createFolderFullTransferHandlerFunc(node, pcWrapper, delayHelper, errorsChannelMng)
	_, err = pcWrapper.chunkBuilderProducerConsumer.AddTaskWithError(folderHandler(folderParams{relativePath: newRelativePath}), pcWrapper.errorsQueue.AddError)
	return
}

// Note: Pattern filtering is handled at AQL level when --include-files is provided.
// This function is only called during folder traversal (when no patterns are specified).
func (m *fullTransferPhase) handleFoundFile(pcWrapper *producerConsumerWrapper,
	delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng,
	node *reposnapshot.Node, item servicesUtils.ResultItem) (err error) {
	file := api.FileRepresentation{Repo: item.Repo, Path: item.Path, Name: item.Name, Size: item.Size}
	delayed, stopped := delayHelper.delayUploadIfNecessary(m.phaseBase, file)
	if delayed || stopped {
		// If delayed, do not increment files count to allow tree collapsing during this phase.
		return
	}
	// Increment the files count in the directory's node in the snapshot manager, to track its progress.
	unsignedFileSize, err := safeconvert.Int64ToUint64(file.Size)
	if err != nil {
		return fmt.Errorf("failed to convert file size to uint64: %w", err)
	}
	err = node.IncrementFilesCount(unsignedFileSize)
	if err != nil {
		return
	}
	_, err = pcWrapper.chunkUploaderProducerConsumer.AddTaskWithError(transferFileHandler(&m.phaseBase, file, errorsChannelMng), pcWrapper.errorsQueue.AddError)
	return
}

func getFolderRelativePath(folderName, relativeLocation string) string {
	if relativeLocation == "." {
		return folderName
	}
	return path.Join(relativeLocation, folderName)
}

func (m *fullTransferPhase) getDirectoryContentAql(relativePath string, paginationOffset int) (result []servicesUtils.ResultItem, lastPage bool, err error) {
	query := generateFolderContentAqlQuery(m.repoKey, relativePath, paginationOffset, m.disabledDistinctiveAql, m.timestampFilter)
	aqlResults, err := runAql(m.context, m.srcRtDetails, query)
	if err != nil {
		return []servicesUtils.ResultItem{}, false, err
	}

	lastPage = len(aqlResults.Results) < AqlPaginationLimit
	result, err = m.locallyGeneratedFilter.FilterLocallyGenerated(aqlResults.Results, m.packageType)
	return
}

func generateFolderContentAqlQuery(repoKey, relativePath string, paginationOffset int, disabledDistinctiveAql bool, filter *timestampFilter) string {
	var query string
	if filter == nil {
		query = fmt.Sprintf(`items.find({"type":"any","$or":[{"$and":[{"repo":"%s","path":{"$match":"%s"},"name":{"$match":"*"}}]}]})`, repoKey, relativePath)
	} else {
		// Keep folders unfiltered so old parents do not hide newer files; require files to match the timestamp.
		query = fmt.Sprintf(`items.find({"$and":[{"repo":"%s","path":{"$match":"%s"},"name":{"$match":"*"}},{"$or":[{"type":"folder"},{"$and":[{"type":"file"},%s]}]}]})`,
			repoKey, relativePath, aqlInclusiveTimestampCondition(filter))
	}
	query += `.include("repo","path","name","type","size")`
	query += fmt.Sprintf(`.sort({"$asc":["name"]}).offset(%d).limit(%d)`, paginationOffset*AqlPaginationLimit, AqlPaginationLimit)
	query += appendDistinctIfNeeded(disabledDistinctiveAql)
	return query
}

// Decide how to continue handling the directory by the node's state in the repository snapshot (completed or not)
// Outputs:
// node - A node in the repository snapshot tree, which represents the current directory.
// completed - Whether handling the node directory was completed. If it wasn't fully transferred, we start exploring and transferring it from scratch.
// previousChildren - If the directory requires exploring, previously known children will be added from this map in order to preserve their states and references.
func (m *fullTransferPhase) getAndHandleDirectoryNode(relativePath string) (node *reposnapshot.Node, completed bool, err error) {
	node, err = m.stateManager.LookUpNode(relativePath)
	if err != nil {
		return
	}

	// If data was not loaded from snapshot, we know that the node is visited for the first time and was not explored.
	if !m.stateManager.WasSnapshotLoaded() {
		return
	}

	completed, err = node.IsCompleted()
	if err != nil {
		return
	}
	if completed {
		m.maybeWarnCompletedFolderSkippedWithFilter()
		log.Debug("Skipping completed folder:", path.Join(m.repoKey, relativePath))
		return
	}
	// If the node was not completed, we will start exploring it from the beginning.
	// Remove all files names because we will begin exploring from the beginning.
	err = node.RestartExploring()
	return
}

// maybeWarnCompletedFolderSkippedWithFilter emits one informational warning when a timestamp filter is
// active and Phase 1 skips folders already marked completed by a previous run.
func (m *fullTransferPhase) maybeWarnCompletedFolderSkippedWithFilter() {
	if m.timestampFilter == nil {
		return
	}
	m.completedFolderFilterWarning.Do(func() {
		log.Warn("A timestamp filter is active, but some folders were already marked completed by a previous transfer. " +
			"Those folders will not be re-scanned under the current filter. Use --ignore-state for a full re-scan.")
	})
}
