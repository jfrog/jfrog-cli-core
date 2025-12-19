package transferfiles

import (
	"fmt"
	"path"
	"time"

	"github.com/jfrog/gofrog/safeconvert"

	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/utils/reposnapshot"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Manages the phase of performing a full transfer of the repository.
// This phase is only executed once per repository if its completed.
// Transfer is performed by treating every folder as a task, and searching for it's content in a flat AQL.
// New folders found are handled as a separate task, and files are uploaded in chunks and polled on for status.
type fullTransferPhase struct {
	phaseBase
	transferManager *transferManager
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
	action := func(pcWrapper *producerConsumerWrapper, uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
		if ShouldStop(&m.phaseBase, &delayHelper, errorsChannelMng) {
			return nil
		}

		// Get the directory's node from the snapshot manager, and use information from previous transfer attempts if such exists.
		node, done, err := m.getAndHandleDirectoryNode(".")
		if err != nil || done {
			return err
		}

		folderHandler := m.createFolderFullTransferHandlerFunc(node, pcWrapper, uploadChunkChan, delayHelper, errorsChannelMng)
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

	action := func(pcWrapper *producerConsumerWrapper, uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
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
				}
				break
			}

			// Convert results to file representations and upload
			files := convertResultsToFileRepresentation(result)
			shouldStop, err := uploadByChunks(files, uploadChunkChan, m.phaseBase, delayHelper, errorsChannelMng, pcWrapper)
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
	query := generatePatternBasedAqlQuery(m.repoKey, m.includeFilesPatterns, paginationOffset, m.disabledDistinctiveAql)
	aqlResults, err := runAql(m.context, m.srcRtDetails, query)
	if err != nil {
		return []servicesUtils.ResultItem{}, false, err
	}

	lastPage = len(aqlResults.Results) < AqlPaginationLimit
	result, err = m.locallyGeneratedFilter.FilterLocallyGenerated(aqlResults.Results)
	return
}

type folderFullTransferHandlerFunc func(params folderParams) parallel.TaskFunc

type folderParams struct {
	relativePath string
}

func (m *fullTransferPhase) createFolderFullTransferHandlerFunc(node *reposnapshot.Node, pcWrapper *producerConsumerWrapper, uploadChunkChan chan UploadedChunk,
	delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) folderFullTransferHandlerFunc {
	return func(params folderParams) parallel.TaskFunc {
		return func(threadId int) error {
			logMsgPrefix := clientUtils.GetLogMsgPrefix(threadId, false)
			return m.transferFolder(node, params, logMsgPrefix, pcWrapper, uploadChunkChan, delayHelper, errorsChannelMng)
		}
	}
}

func (m *fullTransferPhase) transferFolder(node *reposnapshot.Node, params folderParams, logMsgPrefix string, pcWrapper *producerConsumerWrapper,
	uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) (err error) {
	log.Debug(logMsgPrefix+"Handling folder:", path.Join(m.repoKey, params.relativePath))

	// Increment progress number of folders
	if m.progressBar != nil {
		m.progressBar.incNumberOfVisitedFolders()
	}
	if err = m.stateManager.IncVisitedFolders(); err != nil {
		return
	}

	curUploadChunk, err := m.searchAndHandleFolderContents(params, pcWrapper,
		uploadChunkChan, delayHelper, errorsChannelMng, node)
	if err != nil {
		return
	}

	// Mark that no more results are expected for the current folder.
	if err = node.MarkDoneExploring(); err != nil {
		return err
	}

	// Chunk didn't reach full size. Upload the remaining files.
	if len(curUploadChunk.UploadCandidates) > 0 {
		if _, err = pcWrapper.chunkUploaderProducerConsumer.AddTaskWithError(uploadChunkWhenPossibleHandler(pcWrapper, &m.phaseBase, curUploadChunk, uploadChunkChan, errorsChannelMng), pcWrapper.errorsQueue.AddError); err != nil {
			return
		}
	}
	log.Debug(logMsgPrefix+"Done transferring folder:", path.Join(m.repoKey, params.relativePath))
	return
}

func (m *fullTransferPhase) searchAndHandleFolderContents(params folderParams, pcWrapper *producerConsumerWrapper,
	uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng,
	node *reposnapshot.Node) (curUploadChunk api.UploadChunk, err error) {
	curUploadChunk = api.UploadChunk{
		TargetAuth:                createTargetAuth(m.targetRtDetails, m.proxyKey),
		CheckExistenceInFilestore: m.checkExistenceInFilestore,
		// Skip file filtering in the Data Transfer plugin if it is already enabled in the JFrog CLI.
		// The local generated filter is enabled in the JFrog CLI for target Artifactory servers >= 7.55.
		SkipFileFiltering:     m.locallyGeneratedFilter.IsEnabled(),
		MinCheckSumDeploySize: m.minCheckSumDeploySize,
	}

	var result []servicesUtils.ResultItem
	var lastPage bool
	paginationI := 0
	for !lastPage && !ShouldStop(&m.phaseBase, &delayHelper, errorsChannelMng) {
		result, lastPage, err = m.getDirectoryContentAql(params.relativePath, paginationI)
		if err != nil {
			return
		}

		// Add the folder as a candidate to transfer. The reason is that we'd like to transfer only folders with properties or empty folders.
		if params.relativePath != "." {
			curUploadChunk.AppendUploadCandidateIfNeeded(api.FileRepresentation{Repo: m.repoKey, Path: params.relativePath, NonEmptyDir: len(result) > 0}, m.buildInfoRepo)
		}

		// Empty folder
		if paginationI == 0 && len(result) == 0 {
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
				err = m.handleFoundChildFolder(params, pcWrapper,
					uploadChunkChan, delayHelper, errorsChannelMng, item)
			case "file":
				err = m.handleFoundFile(pcWrapper,
					uploadChunkChan, delayHelper, errorsChannelMng,
					node, item, &curUploadChunk)
			}
			if err != nil {
				return
			}
		}

		paginationI++
	}
	return
}

func (m *fullTransferPhase) handleFoundChildFolder(params folderParams, pcWrapper *producerConsumerWrapper,
	uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng,
	item servicesUtils.ResultItem) (err error) {
	newRelativePath := getFolderRelativePath(item.Name, params.relativePath)

	// Get the directory's node from the snapshot manager, and use information from previous transfer attempts if such exists.
	node, done, err := m.getAndHandleDirectoryNode(newRelativePath)
	if err != nil || done {
		return err
	}

	folderHandler := m.createFolderFullTransferHandlerFunc(node, pcWrapper, uploadChunkChan, delayHelper, errorsChannelMng)
	_, err = pcWrapper.chunkBuilderProducerConsumer.AddTaskWithError(folderHandler(folderParams{relativePath: newRelativePath}), pcWrapper.errorsQueue.AddError)
	return
}

// Note: Pattern filtering is handled at AQL level when --include-files is provided.
// This function is only called during folder traversal (when no patterns are specified).
func (m *fullTransferPhase) handleFoundFile(pcWrapper *producerConsumerWrapper,
	uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng,
	node *reposnapshot.Node, item servicesUtils.ResultItem, curUploadChunk *api.UploadChunk) (err error) {
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
	curUploadChunk.AppendUploadCandidateIfNeeded(file, m.buildInfoRepo)
	if curUploadChunk.IsChunkFull() {
		_, err = pcWrapper.chunkUploaderProducerConsumer.AddTaskWithError(uploadChunkWhenPossibleHandler(pcWrapper, &m.phaseBase, *curUploadChunk, uploadChunkChan, errorsChannelMng), pcWrapper.errorsQueue.AddError)
		if err != nil {
			return
		}
		// Empty the uploaded chunk.
		curUploadChunk.UploadCandidates = []api.FileRepresentation{}
	}
	return
}

func getFolderRelativePath(folderName, relativeLocation string) string {
	if relativeLocation == "." {
		return folderName
	}
	return path.Join(relativeLocation, folderName)
}

func (m *fullTransferPhase) getDirectoryContentAql(relativePath string, paginationOffset int) (result []servicesUtils.ResultItem, lastPage bool, err error) {
	query := generateFolderContentAqlQuery(m.repoKey, relativePath, paginationOffset, m.disabledDistinctiveAql)
	aqlResults, err := runAql(m.context, m.srcRtDetails, query)
	if err != nil {
		return []servicesUtils.ResultItem{}, false, err
	}

	lastPage = len(aqlResults.Results) < AqlPaginationLimit
	result, err = m.locallyGeneratedFilter.FilterLocallyGenerated(aqlResults.Results)
	return
}

func generateFolderContentAqlQuery(repoKey, relativePath string, paginationOffset int, disabledDistinctiveAql bool) string {
	query := fmt.Sprintf(`items.find({"type":"any","$or":[{"$and":[{"repo":"%s","path":{"$match":"%s"},"name":{"$match":"*"}}]}]})`, repoKey, relativePath)
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
		log.Debug("Skipping completed folder:", path.Join(m.repoKey, relativePath))
		return
	}
	// If the node was not completed, we will start exploring it from the beginning.
	// Remove all files names because we will begin exploring from the beginning.
	err = node.RestartExploring()
	return
}
