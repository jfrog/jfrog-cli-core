package transferfiles

import (
	"fmt"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/state"
	"github.com/jfrog/jfrog-cli-core/v2/utils/reposnapshot"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"path"
	"time"
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
	return true, m.skipPhase()
}

func (m *fullTransferPhase) skipPhase() error {
	// Init progress bar as "done" with 0 tasks.
	if m.progressBar != nil {
		m.progressBar.AddPhase1(true)
	}
	return nil
}

func (m *fullTransferPhase) run() error {
	m.transferManager = newTransferManager(m.phaseBase, getDelayUploadComparisonFunctions(m.repoSummary.PackageType))
	action := func(pcWrapper *producerConsumerWrapper, uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
		if ShouldStop(&m.phaseBase, &delayHelper, errorsChannelMng) {
			return nil
		}
		folderHandler := m.createFolderFullTransferHandlerFunc(*pcWrapper, uploadChunkChan, delayHelper, errorsChannelMng)
		_, err := pcWrapper.chunkBuilderProducerConsumer.AddTaskWithError(folderHandler(folderParams{relativePath: "."}), pcWrapper.errorsQueue.AddError)
		return err
	}
	delayAction := consumeDelayFilesIfNoErrors
	return m.transferManager.doTransferWithProducerConsumer(action, delayAction)
}

type folderFullTransferHandlerFunc func(params folderParams) parallel.TaskFunc

type folderParams struct {
	relativePath string
}

func (m *fullTransferPhase) createFolderFullTransferHandlerFunc(pcWrapper producerConsumerWrapper, uploadChunkChan chan UploadedChunk,
	delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) folderFullTransferHandlerFunc {
	return func(params folderParams) parallel.TaskFunc {
		return func(threadId int) error {
			logMsgPrefix := clientUtils.GetLogMsgPrefix(threadId, false)
			return m.transferFolder(params, logMsgPrefix, pcWrapper, uploadChunkChan, delayHelper, errorsChannelMng)
		}
	}
}

func (m *fullTransferPhase) transferFolder(params folderParams, logMsgPrefix string, pcWrapper producerConsumerWrapper,
	uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) (err error) {
	log.Debug(logMsgPrefix+"Handling folder:", path.Join(m.repoKey, params.relativePath))

	// Get the directory's node from the snapshot manager, and use information from previous transfer attempts if such exist.
	node, done, previousChildrenMap, err := m.getAndHandleDirectoryNode(params, logMsgPrefix, pcWrapper, uploadChunkChan, delayHelper, errorsChannelMng)
	if err != nil || done {
		return err
	}

	curUploadChunk, err := m.searchAndHandleFolderContents(params, pcWrapper,
		uploadChunkChan, delayHelper, errorsChannelMng,
		node, previousChildrenMap)
	if err != nil {
		return
	}

	// Mark that no more results are expected for the current folder.
	err = node.MarkDoneExploring()
	if err != nil {
		return err
	}

	// Chunk didn't reach full size. Upload the remaining files.
	if len(curUploadChunk.UploadCandidates) > 0 {
		_, err = pcWrapper.chunkUploaderProducerConsumer.AddTaskWithError(uploadChunkWhenPossibleHandler(&m.phaseBase, curUploadChunk, uploadChunkChan, errorsChannelMng), pcWrapper.errorsQueue.AddError)
		if err != nil {
			return
		}
	}
	log.Debug(logMsgPrefix+"Done transferring folder:", path.Join(m.repoKey, params.relativePath))
	return
}

func (m *fullTransferPhase) searchAndHandleFolderContents(params folderParams, pcWrapper producerConsumerWrapper,
	uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng,
	node *reposnapshot.Node, previousChildrenMap map[string]*reposnapshot.Node) (curUploadChunk api.UploadChunk, err error) {
	curUploadChunk = api.UploadChunk{
		TargetAuth:                createTargetAuth(m.targetRtDetails, m.proxyKey),
		CheckExistenceInFilestore: m.checkExistenceInFilestore,
	}

	var result *servicesUtils.AqlSearchResult
	paginationI := 0
	for {
		if ShouldStop(&m.phaseBase, &delayHelper, errorsChannelMng) {
			return
		}
		result, err = m.getDirectoryContentsAql(params.relativePath, paginationI)
		if err != nil {
			return
		}

		// Empty folder. Add it as candidate.
		if paginationI == 0 && len(result.Results) == 0 {
			curUploadChunk.AppendUploadCandidateIfNeeded(api.FileRepresentation{Repo: m.repoKey, Path: params.relativePath}, m.buildInfoRepo)
			return
		}

		for _, item := range result.Results {
			if ShouldStop(&m.phaseBase, &delayHelper, errorsChannelMng) {
				return
			}
			if item.Name == "." {
				continue
			}
			switch item.Type {
			case "folder":
				err = m.handleFoundChildFolder(params, pcWrapper,
					uploadChunkChan, delayHelper, errorsChannelMng,
					node, previousChildrenMap, item)
			case "file":
				err = m.handleFoundFile(pcWrapper,
					uploadChunkChan, delayHelper, errorsChannelMng,
					node, item, &curUploadChunk)
			}
			if err != nil {
				return
			}
		}

		if len(result.Results) < AqlPaginationLimit {
			break
		}
		paginationI++
	}
	return
}

func (m *fullTransferPhase) handleFoundChildFolder(params folderParams, pcWrapper producerConsumerWrapper,
	uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng,
	node *reposnapshot.Node, previousChildrenMap map[string]*reposnapshot.Node, item servicesUtils.ResultItem) (err error) {
	newRelativePath := getFolderRelativePath(item.Name, params.relativePath)
	// Add a node for the found folder, as a child for the current folder in the snapshot manager.
	err = node.AddChildNode(item.Name, previousChildrenMap)
	if err != nil {
		return
	}
	folderHandler := m.createFolderFullTransferHandlerFunc(pcWrapper, uploadChunkChan, delayHelper, errorsChannelMng)
	_, err = pcWrapper.chunkBuilderProducerConsumer.AddTaskWithError(folderHandler(folderParams{relativePath: newRelativePath}), pcWrapper.errorsQueue.AddError)
	return
}

func (m *fullTransferPhase) handleFoundFile(pcWrapper producerConsumerWrapper,
	uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng,
	node *reposnapshot.Node, item servicesUtils.ResultItem, curUploadChunk *api.UploadChunk) (err error) {
	file := api.FileRepresentation{Repo: item.Repo, Path: item.Path, Name: item.Name, Size: item.Size}
	delayed, stopped := delayHelper.delayUploadIfNecessary(m.phaseBase, file)
	if stopped {
		return
	}
	// Add the file name to the directory's node in the snapshot manager, to track its progress.
	err = node.AddFile(item.Name, item.Size)
	if err != nil {
		return
	}
	if delayed {
		return
	}
	curUploadChunk.AppendUploadCandidateIfNeeded(file, m.buildInfoRepo)
	if len(curUploadChunk.UploadCandidates) == uploadChunkSize {
		_, err = pcWrapper.chunkUploaderProducerConsumer.AddTaskWithError(uploadChunkWhenPossibleHandler(&m.phaseBase, *curUploadChunk, uploadChunkChan, errorsChannelMng), pcWrapper.errorsQueue.AddError)
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

func (m *fullTransferPhase) getDirectoryContentsAql(relativePath string, paginationOffset int) (result *servicesUtils.AqlSearchResult, err error) {
	query := generateFolderContentsAqlQuery(m.repoKey, relativePath, paginationOffset)
	return runAql(m.context, m.srcRtDetails, query)
}

func generateFolderContentsAqlQuery(repoKey, relativePath string, paginationOffset int) string {
	query := fmt.Sprintf(`items.find({"type":"any","$or":[{"$and":[{"repo":"%s","path":{"$match":"%s"},"name":{"$match":"*"}}]}]})`, repoKey, relativePath)
	query += `.include("repo","path","name","type","size")`
	query += fmt.Sprintf(`.sort({"$asc":["name"]}).offset(%d).limit(%d)`, paginationOffset*AqlPaginationLimit, AqlPaginationLimit)
	return query
}

func convertRepoSnapshotToFileRepresentation(repoKey, relativePath string, snapshot *reposnapshot.Node) (files []api.FileRepresentation, err error) {
	filesMap, err := snapshot.GetFiles()
	if err != nil {
		return
	}
	for fileName, size := range filesMap {
		files = append(files, api.FileRepresentation{
			Repo: repoKey, Path: relativePath, Name: fileName, Size: size,
		})
	}
	return
}

// Decide how to continue handling the directory by the node's state in the repository snapshot (completed / done exploring / requires exploring)
// Outputs:
// node - A node in the repository snapshot tree, which represents the current directory.
// done - Whether the node directory should be explored or not. Exploring means searching for the directory contents. If all contents were previously found, there's no need to search again, just upload the known results.
// previousChildrenMap - If the directory requires exploring, previously known children will be added from this map in order to preserve their states and references.
func (m *fullTransferPhase) getAndHandleDirectoryNode(params folderParams, logMsgPrefix string, pcWrapper producerConsumerWrapper,
	uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) (node *reposnapshot.Node, done bool, previousChildrenMap map[string]*reposnapshot.Node, err error) {
	node, err = m.stateManager.LookUpNode(params.relativePath)
	if err != nil {
		return
	}
	// If data was not loaded from snapshot, we know that the node is visited for the first time and was not explored.
	loadedFromSnapshot, err := m.stateManager.WasSnapshotLoaded()
	if err != nil || !loadedFromSnapshot {
		return
	}

	completed, err := node.IsCompleted()
	if err != nil {
		return
	}
	if completed {
		log.Debug(logMsgPrefix+"Skipping completed folder: ", path.Join(m.repoKey, params.relativePath))
		return nil, true, nil, nil
	}
	doneExploring, err := node.IsDoneExploring()
	if err != nil {
		return
	}
	// All directory contents were already found, but not handled.
	if doneExploring {
		return nil, true, nil, m.handleNodeExplored(node, params, logMsgPrefix, pcWrapper, uploadChunkChan, delayHelper, errorsChannelMng)
	}
	previousChildrenMap, err = m.handleNodeRequiresExploring(node)
	return
}

// If a node had been fully explored before but not completed, complete it by adding all its children as tasks and upload all its files.
func (m *fullTransferPhase) handleNodeExplored(node *reposnapshot.Node, params folderParams, logMsgPrefix string, pcWrapper producerConsumerWrapper,
	uploadChunkChan chan UploadedChunk, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) (err error) {

	log.Debug(logMsgPrefix + "Folder '" + path.Join(m.repoKey, params.relativePath) + "' was already explored. Uploading remaining files...")
	children, err := node.GetChildren()
	if err != nil {
		return
	}
	for childName := range children {
		folderHandler := m.createFolderFullTransferHandlerFunc(pcWrapper, uploadChunkChan, delayHelper, errorsChannelMng)
		_, err = pcWrapper.chunkBuilderProducerConsumer.AddTaskWithError(folderHandler(folderParams{relativePath: getFolderRelativePath(childName, params.relativePath)}), pcWrapper.errorsQueue.AddError)
		if err != nil {
			return
		}
	}
	filesNames, err := node.GetFiles()
	if err != nil {
		return
	}
	if len(filesNames) > 0 {
		var files []api.FileRepresentation
		files, err = convertRepoSnapshotToFileRepresentation(m.repoKey, params.relativePath, node)
		if err != nil {
			return
		}
		_, err = uploadByChunks(files, uploadChunkChan, m.phaseBase, delayHelper, errorsChannelMng, &pcWrapper)
	}
	return
}

func (m *fullTransferPhase) handleNodeRequiresExploring(node *reposnapshot.Node) (previousChildrenMap map[string]*reposnapshot.Node, err error) {
	// Return old children map to add every found child with its previous data and references.
	previousChildrenMap, err = node.GetChildren()
	if err != nil {
		return
	}
	// Remove all files names because we will begin exploring from the beginning.
	// Clear children map to avoid handling directories that may have been deleted.
	err = node.RestartExploring()
	return
}
