package transferfiles

import (
	"fmt"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	artifactoryUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"path"
	"sync"
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
	serviceManager, err := utils.CreateServiceManager(m.getSourceDetails(), -1, 0, false)
	if err != nil {
		return err
	}
	repoSummaryList, err := serviceManager.StorageInfo(true)
	if err != nil {
		return err
	}
	for _, repo := range repoSummaryList.RepositoriesSummaryList {
		if m.repoKey == repo.RepoKey {
			tasks, err := repo.FilesCount.Int64()
			if err != nil {
				return err
			}
			m.progressBar.AddPhase1(tasks)
			return nil
		}
	}
	// Case that the repo was exists in the beginning of the transfer run and was deleted before handled.
	log.Error(fmt.Sprintf("repository: \"%s\" doesn't exists in Artifactory", m.repoKey))
	return nil
}

func (m *fullTransferPhase) getPhaseName() string {
	return "Full Transfer Phase"
}

func (m *fullTransferPhase) getPhaseId() int {
	return 0
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
		return m.progressBar.DonePhase(m.getPhaseId())
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

func (m *fullTransferPhase) run() error {
	producerConsumer := parallel.NewRunner(getThreads(), tasksMaxCapacity, false)
	expectedChan := make(chan int, 1)
	errorsQueue := clientUtils.NewErrorsQueue(1)
	uploadTokensChan := make(chan string, tasksMaxCapacity)
	var runWaitGroup sync.WaitGroup
	// Done channel notifies the polling go routines that no more tasks are expected.
	doneChan := make(chan bool, 2)

	errorChannel := make(chan FileUploadStatusResponse, errorChannelSize)
	transferErrorsMng, err := newTransferErrorsToFile(m.repoKey, m.getPhaseId(), convertTimeToEpochMilliseconds(m.startTime), errorChannel)
	if err != nil {
		return err
	}
	// Error returned from the "writing transfer errors to file" mechanism
	var writingErrorsErr error
	go func() {
		writingErrorsErr = transferErrorsMng.start()
	}()

	runWaitGroup.Add(1)
	go func() {
		defer runWaitGroup.Done()
		periodicallyUpdateThreads(producerConsumer, doneChan)
	}()

	runWaitGroup.Add(1)
	go func() {
		defer runWaitGroup.Done()
		pcDetails := producerConsumerDetails{
			producerConsumer: producerConsumer,
			expectedChan:     expectedChan,
			errorsQueue:      errorsQueue,
			uploadTokensChan: uploadTokensChan,
		}
		folderHandler := m.createFolderFullTransferHandlerFunc(pcDetails)
		_, err := producerConsumer.AddTaskWithError(folderHandler(folderParams{repoKey: m.repoKey, relativePath: "."}), errorsQueue.AddError)
		if err != nil {
			log.Error("error occurred when adding new task to producer consumer:" + err.Error())
		}
	}()

	var pollingError error
	runWaitGroup.Add(1)
	go func() {
		defer runWaitGroup.Done()
		pollingError = pollUploads(m.srcUpService, uploadTokensChan, doneChan, errorChannel)
	}()

	var runnerErr error
	runWaitGroup.Add(1)
	go func() {
		defer runWaitGroup.Done()
		// When the producer consumer is idle for assumeProducerConsumerDoneWhenIdleForSeconds (not tasks are being handled)
		// the work is assumed to be done.
		runnerErr = producerConsumer.DoneWhenAllIdle(assumeProducerConsumerDoneWhenIdleForSeconds)
		// Notify the other go routines that work is done.
		doneChan <- true
		doneChan <- true
	}()
	// Blocked until finish consuming
	producerConsumer.Run()
	runWaitGroup.Wait()

	close(errorChannel)

	// Checking if we had an error while writing the transfer's errors files
	if writingErrorsErr != nil {
		return writingErrorsErr
	}

	var returnedError error
	for _, err := range []error{runnerErr, pollingError, errorsQueue.GetError()} {
		if err != nil {
			log.Error(err)
			returnedError = err
		}
	}
	return returnedError
}

type folderFullTransferHandlerFunc func(params folderParams) parallel.TaskFunc

type folderParams struct {
	repoKey      string
	relativePath string
}

func (m *fullTransferPhase) createFolderFullTransferHandlerFunc(pcDetails producerConsumerDetails) folderFullTransferHandlerFunc {
	return func(params folderParams) parallel.TaskFunc {
		return func(threadId int) error {
			logMsgPrefix := clientUtils.GetLogMsgPrefix(threadId, false)
			return m.transferFolder(params, logMsgPrefix, pcDetails)
		}
	}
}

func (m *fullTransferPhase) transferFolder(params folderParams, logMsgPrefix string, pcDetails producerConsumerDetails) error {
	log.Debug(logMsgPrefix+"Visited folder:", path.Join(params.repoKey, params.relativePath))

	result, err := m.getDirectoryContentsAql(params.repoKey, params.relativePath)
	if err != nil {
		return err
	}

	curUploadChunk := UploadChunk{
		TargetAuth:                createTargetAuth(m.targetRtDetails),
		CheckExistenceInFilestore: m.checkExistenceInFilestore,
	}

	for _, item := range result.Results {
		if item.Name == "." {
			continue
		}
		switch item.Type {
		case "folder":
			newRelativePath := item.Name
			if params.relativePath != "." {
				newRelativePath = path.Join(params.relativePath, newRelativePath)
			}
			folderHandler := m.createFolderFullTransferHandlerFunc(pcDetails)
			_, err = pcDetails.producerConsumer.AddTaskWithError(folderHandler(folderParams{repoKey: params.repoKey, relativePath: newRelativePath}), pcDetails.errorsQueue.AddError)
			if err != nil {
				log.Error("error occurred when adding new task to producer consumer:" + err.Error())
			}
		case "file":
			curUploadChunk.appendUploadCandidate(item.Repo, item.Path, item.Name)
			if len(curUploadChunk.UploadCandidates) == uploadChunkSize {
				err := uploadChunkWhenPossible(m.srcUpService, curUploadChunk, pcDetails.uploadTokensChan)
				if err != nil {
					log.Error(err)
				}
				// Increase phase1 progress bar with the uploaded number of files.
				if m.progressBar != nil {
					err = m.progressBar.IncrementPhaseBy(m.getPhaseId(), len(curUploadChunk.UploadCandidates))
					if err != nil {
						return err
					}
				}
				// Empty the uploaded chunk.
				curUploadChunk.UploadCandidates = []FileRepresentation{}
			}
		}
	}

	// Empty folder. Add it as candidate.
	if len(result.Results) == 0 {
		curUploadChunk.appendUploadCandidate(params.repoKey, path.Dir(params.relativePath), path.Base(params.relativePath))
	}

	// Chunk didn't reach full size. Upload the remaining files.
	if len(curUploadChunk.UploadCandidates) > 0 {
		err = uploadChunkWhenPossible(m.srcUpService, curUploadChunk, pcDetails.uploadTokensChan)
		if err != nil {
			return err
		}
		// Increase phase1 progress bar with the uploaded number of files.
		if m.progressBar != nil {
			err = m.progressBar.IncrementPhaseBy(m.getPhaseId(), len(curUploadChunk.UploadCandidates))
			if err != nil {
				return err
			}
		}
	}
	log.Debug(logMsgPrefix+"Done transferring folder:", path.Join(params.repoKey, params.relativePath))
	return nil
}

func (m *fullTransferPhase) getDirectoryContentsAql(repoKey, relativePath string) (result *artifactoryUtils.AqlSearchResult, err error) {
	query := generateFolderContentsAqlQuery(repoKey, relativePath)
	return runAql(m.srcRtDetails, query)
}

func generateFolderContentsAqlQuery(repoKey, relativePath string) string {
	return fmt.Sprintf(`items.find({"type":"any","$or":[{"$and":[{"repo":"%s","path":{"$match":"%s"},"name":{"$match":"*"}}]}]}).include("repo","path","name","type")`, repoKey, relativePath)
}
