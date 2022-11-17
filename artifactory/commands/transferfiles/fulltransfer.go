package transferfiles

import (
	"fmt"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
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
	transferManager *transferManager
}

func (m *fullTransferPhase) initProgressBar() error {
	if m.progressBar == nil {
		return nil
	}
	storage, err := m.repoSummary.UsedSpaceInBytes.Int64()
	if err != nil {
		return err
	}
	skip, err := m.shouldSkipPhase()
	if err != nil {
		return err
	}
	return m.progressBar.AddPhase1(storage, skip)
}

func (m *fullTransferPhase) getPhaseName() string {
	return "Full Transfer Phase"
}

func (m *fullTransferPhase) phaseStarted() error {
	m.startTime = time.Now()
	return m.stateManager.SetRepoFullTransferStarted(m.startTime)
}

func (m *fullTransferPhase) phaseDone() error {
	// If the phase stopped gracefully, don't mark the phase as completed
	if !m.ShouldStop() {
		if err := m.stateManager.SetRepoFullTransferCompleted(); err != nil {
			return err
		}
	}

	if m.progressBar != nil {
		return m.progressBar.DonePhase(m.phaseId)
	}
	return nil
}

func (m *fullTransferPhase) shouldSkipPhase() (bool, error) {
	repoTransferred, err := m.stateManager.IsRepoTransferred()
	if err != nil {
		return false, err
	}
	if repoTransferred {
		return repoTransferred, m.skipPhase()
	}
	return repoTransferred, nil
}

func (m *fullTransferPhase) skipPhase() error {
	// Init progress bar as "done" with 0 tasks.
	if m.progressBar != nil {
		return m.progressBar.AddPhase1(0, true)
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
	log.Debug(logMsgPrefix+"Visited folder:", path.Join(m.repoKey, params.relativePath))

	curUploadChunk := api.UploadChunk{
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
			return err
		}

		// Empty folder. Add it as candidate.
		if paginationI == 0 && len(result.Results) == 0 {
			curUploadChunk.AppendUploadCandidateIfNeeded(api.FileRepresentation{Repo: m.repoKey, Path: params.relativePath}, m.buildInfoRepo)
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
				newRelativePath := item.Name
				if params.relativePath != "." {
					newRelativePath = path.Join(params.relativePath, newRelativePath)
				}
				folderHandler := m.createFolderFullTransferHandlerFunc(pcWrapper, uploadChunkChan, delayHelper, errorsChannelMng)
				_, err = pcWrapper.chunkBuilderProducerConsumer.AddTaskWithError(folderHandler(folderParams{relativePath: newRelativePath}), pcWrapper.errorsQueue.AddError)
				if err != nil {
					return err
				}
			case "file":
				file := api.FileRepresentation{Repo: item.Repo, Path: item.Path, Name: item.Name}
				delayed, stopped := delayHelper.delayUploadIfNecessary(m.phaseBase, file)
				if stopped {
					return
				}
				if delayed {
					continue
				}
				curUploadChunk.AppendUploadCandidateIfNeeded(file, m.buildInfoRepo)
				if len(curUploadChunk.UploadCandidates) == uploadChunkSize {
					_, err = pcWrapper.chunkUploaderProducerConsumer.AddTaskWithError(uploadChunkWhenPossibleHandler(&m.phaseBase, curUploadChunk, uploadChunkChan, errorsChannelMng), pcWrapper.errorsQueue.AddError)
					if err != nil {
						return
					}
					// Empty the uploaded chunk.
					curUploadChunk.UploadCandidates = []api.FileRepresentation{}
				}
			}
		}

		if len(result.Results) < AqlPaginationLimit {
			break
		}
		paginationI++
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

func (m *fullTransferPhase) getDirectoryContentsAql(relativePath string, paginationOffset int) (result *servicesUtils.AqlSearchResult, err error) {
	query := generateFolderContentsAqlQuery(m.repoKey, relativePath, paginationOffset)
	return runAql(m.context, m.srcRtDetails, query)
}

func generateFolderContentsAqlQuery(repoKey, relativePath string, paginationOffset int) string {
	query := fmt.Sprintf(`items.find({"type":"any","$or":[{"$and":[{"repo":"%s","path":{"$match":"%s"},"name":{"$match":"*"}}]}]})`, repoKey, relativePath)
	query += `.include("repo","path","name","type")`
	query += fmt.Sprintf(`.sort({"$asc":["name"]}).offset(%d).limit(%d)`, paginationOffset*AqlPaginationLimit, AqlPaginationLimit)
	return query
}
