package transferfiles

import (
	"fmt"
	"github.com/jfrog/gofrog/parallel"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	servicesUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
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
}

func (m *fullTransferPhase) getSourceDetails() *coreConfig.ServerDetails {
	return m.srcRtDetails
}

func (m *fullTransferPhase) setProgressBar(progressbar *progressbar.TransferProgressMng) {
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
	err := setRepoFullTransferStarted(m.repoKey, m.startTime)
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
	if m.progressBar != nil {
		return m.progressBar.DonePhase(m.phaseId)
	}
	return nil
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
	action := func(pcDetails *producerConsumerDetails, uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) error {
		// In case an error occurred while handling errors/delayed artifacts files - stop transferring.
		if delayHelper.delayedArtifactsChannelMng.shouldStop() || errorsChannelMng.shouldStop() {
			log.Debug("Stop transferring data - error occurred while handling transfer's errors/delayed artifacts files.")
			return nil
		}
		folderHandler := m.createFolderFullTransferHandlerFunc(*pcDetails, uploadTokensChan, delayHelper, errorsChannelMng)
		_, err := pcDetails.producerConsumer.AddTaskWithError(folderHandler(folderParams{repoKey: m.repoKey, relativePath: "."}), pcDetails.errorsQueue.AddError)
		return err
	}
	return manager.doTransferWithProducerConsumer(action)
}

type folderFullTransferHandlerFunc func(params folderParams) parallel.TaskFunc

type folderParams struct {
	repoKey      string
	relativePath string
}

func (m *fullTransferPhase) createFolderFullTransferHandlerFunc(pcDetails producerConsumerDetails, uploadTokensChan chan string,
	delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) folderFullTransferHandlerFunc {
	return func(params folderParams) parallel.TaskFunc {
		return func(threadId int) error {
			logMsgPrefix := clientUtils.GetLogMsgPrefix(threadId, false)
			return m.transferFolder(params, logMsgPrefix, pcDetails, uploadTokensChan, delayHelper, errorsChannelMng)
		}
	}
}

func (m *fullTransferPhase) transferFolder(params folderParams, logMsgPrefix string, pcDetails producerConsumerDetails,
	uploadTokensChan chan string, delayHelper delayUploadHelper, errorsChannelMng *ErrorsChannelMng) (err error) {
	log.Debug(logMsgPrefix+"Visited folder:", path.Join(params.repoKey, params.relativePath))

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
			curUploadChunk.appendUploadCandidate(FileRepresentation{
				Repo: params.repoKey,
				Path: path.Dir(params.relativePath),
				Name: path.Base(params.relativePath),
			})
			break
		}

		for _, item := range result.Results {
			// In case an error occurred while handling errors/delayed artifacts files - stop transferring.
			if delayHelper.delayedArtifactsChannelMng.shouldStop() || errorsChannelMng.shouldStop() {
				log.Debug("Stop transferring data - error occurred while handling transfer's errors/delayed artifacts files.")
				return
			}
			if item.Name == "." {
				continue
			}
			switch item.Type {
			case "folder":
				newRelativePath := item.Name
				if params.relativePath != "." {
					newRelativePath = path.Join(params.relativePath, newRelativePath)
				}
				folderHandler := m.createFolderFullTransferHandlerFunc(pcDetails, uploadTokensChan, delayHelper, errorsChannelMng)
				_, err = pcDetails.producerConsumer.AddTaskWithError(folderHandler(folderParams{repoKey: params.repoKey, relativePath: newRelativePath}), pcDetails.errorsQueue.AddError)
				if err != nil {
					return err
				}
			case "file":
				file := FileRepresentation{Repo: item.Repo, Path: item.Path, Name: item.Name}
				delayed, stopped := delayHelper.delayUploadIfNecessary(file)
				if stopped {
					return
				}
				if delayed {
					continue
				}
				curUploadChunk.appendUploadCandidate(file)
				if len(curUploadChunk.UploadCandidates) == uploadChunkSize {
					stopped = uploadChunkWhenPossible(m.srcUpService, curUploadChunk, uploadTokensChan, errorsChannelMng)
					if stopped {
						return
					}
					// Increase phase1 progress bar with the uploaded number of files.
					if m.progressBar != nil {
						err = m.progressBar.IncrementPhaseBy(m.phaseId, len(curUploadChunk.UploadCandidates))
						if err != nil {
							return err
						}
					}
					// Empty the uploaded chunk.
					curUploadChunk.UploadCandidates = []FileRepresentation{}
				}
			}
		}

		if len(result.Results) < aqlPaginationLimit {
			break
		}
		paginationI++
	}

	// Chunk didn't reach full size. Upload the remaining files.
	if len(curUploadChunk.UploadCandidates) > 0 {
		if uploadChunkWhenPossible(m.srcUpService, curUploadChunk, uploadTokensChan, errorsChannelMng) {
			return
		}
		// Increase phase1 progress bar with the uploaded number of files.
		if m.progressBar != nil {
			err = m.progressBar.IncrementPhaseBy(m.phaseId, len(curUploadChunk.UploadCandidates))
			if err != nil {
				return
			}
		}
	}
	log.Debug(logMsgPrefix+"Done transferring folder:", path.Join(params.repoKey, params.relativePath))
	return
}

func (m *fullTransferPhase) getDirectoryContentsAql(repoKey, relativePath string, paginationOffset int) (result *servicesUtils.AqlSearchResult, err error) {
	query := generateFolderContentsAqlQuery(repoKey, relativePath, paginationOffset)
	return runAql(m.srcRtDetails, query)
}

func generateFolderContentsAqlQuery(repoKey, relativePath string, paginationOffset int) string {
	query := fmt.Sprintf(`items.find({"type":"any","$or":[{"$and":[{"repo":"%s","path":{"$match":"%s"},"name":{"$match":"*"}}]}]})`, repoKey, relativePath)
	query += `.include("repo","path","name","type")`
	query += fmt.Sprintf(`.sort({"$asc":["name"]}).offset(%d).limit(%d)`, paginationOffset*aqlPaginationLimit, aqlPaginationLimit)
	return query
}
