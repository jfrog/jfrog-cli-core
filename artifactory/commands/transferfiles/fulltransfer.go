package transferfiles

import (
	"fmt"
	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/gofrog/parallel"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/repostate"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"path"
	"path/filepath"
	"time"
)

const saveRepositoryTreeStateMinutes = 10

// Manages the phase of performing a full transfer of the repository.
// This phase is only executed once per repository if its completed.
// Transfer is performed by treating every folder as a task, and searching for it's content in a flat AQL.
// New folders found are handled as a separate task, and files are uploaded in chunks and polled on for status.
type fullTransferPhase struct {
	phaseBase
}

func (m *fullTransferPhase) getSourceDetails() *coreConfig.ServerDetails {
	return m.srcRtDetails
}

func (m *fullTransferPhase) setProgressBar(progressbar *TransferProgressMng) {
	m.progressBar = progressbar
}

func (m *fullTransferPhase) initProgressBar() error {
	if m.progressBar == nil {
		return nil
	}
	tasks, err := m.repoSummary.FilesCount.Int64()
	if err != nil {
		return err
	}
	m.progressBar.AddPhase1(tasks)
	return nil
}

func (m *fullTransferPhase) getPhaseName() string {
	return "Full Transfer Phase"
}

func (m *fullTransferPhase) phaseStarted() error {
	m.startTime = time.Now()

	repoStateFile, err := GetRepoStateFilePath(m.repoKey)
	if err != nil {
		return err
	}
	// Loads repository state if exists, and updates started time accordingly.
	stateCreated := false
	m.stateManager, stateCreated, err = repostate.LoadOrCreateRepoStateManager(m.repoKey, repoStateFile)
	if err != nil {
		return err
	}

	err = setRepoFullTransferStarted(m.repoKey, m.startTime, !stateCreated)
	if err != nil {
		return err
	}

	if !isPropertiesPhaseDisabled() {
		return m.srcUpService.storeProperties(m.repoKey)
	}
	return nil
}

func (m *fullTransferPhase) phaseDone() error {
	err := setRepoFullTransferCompleted(m.repoKey)
	if err != nil {
		return err
	}
	// Deletes the repo state as it is no longer needed after full transfer is completed (state is also collapsed, so no info is provided there).
	err = m.removeRepoState()
	if err != nil {
		return err
	}

	if m.progressBar != nil {
		return m.progressBar.DonePhase(m.phaseId)
	}
	return nil
}

func (m *fullTransferPhase) removeRepoState() error {
	repoStateFile, err := GetRepoStateFilePath(m.repoKey)
	if err != nil {
		return err
	}
	exists, err := fileutils.IsFileExists(repoStateFile, false)
	if err != nil || !exists {
		return err
	}
	return os.Remove(repoStateFile)
}

func GetRepoStateFilePath(repoKey string) (string, error) {
	statesDir, err := coreutils.GetJfrogTransferRepositoriesStateDir()
	if err != nil {
		return "", err
	}
	err = utils.CreateDirIfNotExist(statesDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(statesDir, repoKey+".json"), nil
}

func (m *fullTransferPhase) shouldCheckExistenceInFilestore(shouldCheck bool) {
	m.checkExistenceInFilestore = shouldCheck
}

func (m *fullTransferPhase) shouldSkipPhase() (bool, error) {
	skip, err := isRepoTransferred(m.repoKey)
	if err != nil {
		return false, err
	}
	if skip {
		m.skipPhase()
	}
	return skip, nil
}

func (m *fullTransferPhase) skipPhase() {
	// Init progress bar as "done" with 0 tasks.
	if m.progressBar != nil {
		m.progressBar.AddPhase1(0)
	}
}

func (m *fullTransferPhase) setSrcUserPluginService(service *srcUserPluginService) {
	m.srcUpService = service
}

func (m *fullTransferPhase) setSourceDetails(details *coreConfig.ServerDetails) {
	m.srcRtDetails = details
}

func (m *fullTransferPhase) setTargetDetails(details *coreConfig.ServerDetails) {
	m.targetRtDetails = details
}

func (m *fullTransferPhase) setRepoSummary(repoSummary servicesUtils.RepositorySummary) {
	m.repoSummary = repoSummary
}

func (m *fullTransferPhase) run() error {
	manager := newTransferManager(m.phaseBase, getDelayUploadComparisonFunctions(m.repoSummary.PackageType))
	action := func(pcWrapper *producerConsumerWrapper, uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
		if ShouldStop(&m.phaseBase, &delayHelper, errorsChannelMng) {
			return nil
		}
		folderHandler := m.createFolderFullTransferHandlerFunc(*pcWrapper, uploadTokensChan, delayHelper, errorsChannelMng)
		_, err := pcWrapper.chunkBuilderProducerConsumer.AddTaskWithError(folderHandler(folderParams{repoKey: m.repoKey, relativePath: "."}), pcWrapper.errorsQueue.AddError)
		return err
	}
	return manager.doTransferWithProducerConsumer(action)
}

type folderFullTransferHandlerFunc func(params folderParams) parallel.TaskFunc

type folderParams struct {
	repoKey      string
	relativePath string
}

func (m *fullTransferPhase) createFolderFullTransferHandlerFunc(pcWrapper producerConsumerWrapper, uploadTokensChan chan string,
	delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) folderFullTransferHandlerFunc {
	return func(params folderParams) parallel.TaskFunc {
		return func(threadId int) error {
			logMsgPrefix := clientUtils.GetLogMsgPrefix(threadId, false)
			return m.transferFolder(params, logMsgPrefix, pcWrapper, uploadTokensChan, delayHelper, errorsChannelMng)
		}
	}
}

func (m *fullTransferPhase) transferFolder(params folderParams, logMsgPrefix string, pcWrapper producerConsumerWrapper,
	uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) (err error) {
	log.Debug(logMsgPrefix+"Handling folder:", path.Join(params.repoKey, params.relativePath))

	node, done, previousChildrenMap, err := m.getNodeAndHandleByState(params, logMsgPrefix, pcWrapper, uploadTokensChan, delayHelper, errorsChannelMng)
	if err != nil || done {
		return err
	}

	curUploadChunk := UploadChunk{
		TargetAuth:                createTargetAuth(m.targetRtDetails),
		CheckExistenceInFilestore: m.checkExistenceInFilestore,
	}

	var result *servicesUtils.AqlSearchResult
	paginationI := 0
	for {
		result, err = m.getDirectoryContentsAql(params.repoKey, params.relativePath, paginationI)
		if err != nil {
			return err
		}

		// Empty folder. Add it as candidate.
		if paginationI == 0 && len(result.Results) == 0 {
			curUploadChunk.appendUploadCandidate(FileRepresentation{Repo: params.repoKey, Path: params.relativePath})
			break
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
				newRelativePath := getFolderRelativePath(item.Name, params.relativePath)
				node.AddChildNode(item.Name, previousChildrenMap)
				folderHandler := m.createFolderFullTransferHandlerFunc(pcWrapper, uploadTokensChan, delayHelper, errorsChannelMng)
				_, err = pcWrapper.chunkBuilderProducerConsumer.AddTaskWithError(folderHandler(folderParams{repoKey: params.repoKey, relativePath: newRelativePath}), pcWrapper.errorsQueue.AddError)
				if err != nil {
					return err
				}
			case "file":
				file := FileRepresentation{Repo: item.Repo, Path: item.Path, Name: item.Name}
				delayed, stopped := delayHelper.delayUploadIfNecessary(m.phaseBase, file)
				if stopped {
					return
				}
				node.AddFileName(item.Name)
				if delayed {
					continue
				}
				curUploadChunk.appendUploadCandidate(file)
				if len(curUploadChunk.UploadCandidates) == uploadChunkSize {
					_, err = pcWrapper.chunkUploaderProducerConsumer.AddTaskWithError(uploadChunkWhenPossibleHandler(&m.phaseBase, curUploadChunk, uploadTokensChan, errorsChannelMng, m.stateManager), pcWrapper.errorsQueue.AddError)
					if err != nil {
						return
					}
					// Empty the uploaded chunk.
					curUploadChunk.UploadCandidates = []FileRepresentation{}
				}
			}
		}

		if len(result.Results) < AqlPaginationLimit {
			break
		}
		paginationI++
	}

	node.DoneExploring = true

	// Chunk didn't reach full size. Upload the remaining files.
	if len(curUploadChunk.UploadCandidates) > 0 {
		_, err = pcWrapper.chunkUploaderProducerConsumer.AddTaskWithError(uploadChunkWhenPossibleHandler(&m.phaseBase, curUploadChunk, uploadTokensChan, errorsChannelMng, m.stateManager), pcWrapper.errorsQueue.AddError)
		if err != nil {
			return
		}
	}
	log.Debug(logMsgPrefix+"Done transferring folder:", path.Join(params.repoKey, params.relativePath))
	return
}

func getFolderRelativePath(folderName, relativeLocation string) string {
	if relativeLocation == "." {
		return folderName
	}
	return path.Join(relativeLocation, folderName)
}

func (m *fullTransferPhase) getDirectoryContentsAql(repoKey, relativePath string, paginationOffset int) (result *servicesUtils.AqlSearchResult, err error) {
	query := generateFolderContentsAqlQuery(repoKey, relativePath, paginationOffset)
	return runAql(m.srcRtDetails, query)
}

func generateFolderContentsAqlQuery(repoKey, relativePath string, paginationOffset int) string {
	query := fmt.Sprintf(`items.find({"type":"any","$or":[{"$and":[{"repo":"%s","path":{"$match":"%s"},"name":{"$match":"*"}}]}]})`, repoKey, relativePath)
	query += `.include("repo","path","name","type")`
	query += fmt.Sprintf(`.sort({"$asc":["name"]}).offset(%d).limit(%d)`, paginationOffset*AqlPaginationLimit, AqlPaginationLimit)
	return query
}

func convertRepoStateToFileRepresentation(repoKey, relativePath string, state *repostate.Node) (files []FileRepresentation) {
	for file := range state.FilesNames {
		files = append(files, FileRepresentation{
			repoKey, relativePath, file,
		})
	}
	return
}

// If previous repo state exists, uses the known info to handle the current directory.
// Decides how to continue handling the directory by the node's state in the repository state (completed / done exploring / requires exploring)
// Outputs:
// node - node in repository state.
// done - whether the node directory is requires exploring or not.
// previousChildrenMap - if the directory requires exploring, previously known children will be added from this map in order to preserve their states and references.
func (m *fullTransferPhase) getNodeAndHandleByState(params folderParams, logMsgPrefix string, pcWrapper producerConsumerWrapper,
	uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) (node *repostate.Node, done bool, previousChildrenMap map[string]*repostate.Node, err error) {
	node, err = m.stateManager.LookUpNode(params.relativePath)
	if err != nil {
		return
	}
	if node.Completed {
		log.Debug(logMsgPrefix+"Skipping completed folder: ", path.Join(params.repoKey, params.relativePath))
		return nil, true, nil, nil
	}
	if node.DoneExploring {
		return nil, true, nil, m.handleNodeExplored(node, params, logMsgPrefix, pcWrapper, uploadTokensChan, delayHelper, errorsChannelMng)
	}
	return node, false, m.handleNodeRequiresExploring(node), nil
}

// If a node was fully explored before but not completed, complete it by adding all its children as tasks and upload all its files.
func (m *fullTransferPhase) handleNodeExplored(node *repostate.Node, params folderParams, logMsgPrefix string, pcWrapper producerConsumerWrapper,
	uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) (err error) {

	log.Debug(logMsgPrefix + "Folder '" + path.Join(params.repoKey, params.relativePath) + "' was already explored. Uploading remaining files...")
	for childName := range node.Children {
		folderHandler := m.createFolderFullTransferHandlerFunc(pcWrapper, uploadTokensChan, delayHelper, errorsChannelMng)
		_, err = pcWrapper.chunkBuilderProducerConsumer.AddTaskWithError(folderHandler(folderParams{repoKey: params.repoKey, relativePath: getFolderRelativePath(childName, params.relativePath)}), pcWrapper.errorsQueue.AddError)
		if err != nil {
			return
		}
	}
	if len(node.FilesNames) > 0 {
		_, err = uploadByChunks(convertRepoStateToFileRepresentation(params.repoKey, params.relativePath, node), uploadTokensChan, m.phaseBase, delayHelper, errorsChannelMng, &pcWrapper)
		if m.progressBar != nil {
			err = m.progressBar.IncrementPhaseBy(m.phaseId, len(node.FilesNames))
			if err != nil {
				return err
			}
		}
	}
	return
}

func (m *fullTransferPhase) handleNodeRequiresExploring(node *repostate.Node) (previousChildrenMap map[string]*repostate.Node) {
	// If not done exploring, remove all files names because we will begin exploring from the beginning.
	node.FilesNames = nil
	// Return old children map to add every found child with its previous data and references.
	previousChildrenMap = node.Children
	// Clear children map to avoid handling directories that may have been deleted.
	node.Children = nil
	return
}
