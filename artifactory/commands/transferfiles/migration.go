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

type migrationPhase struct {
	repoKey                   string
	checkExistenceInFilestore bool
	startTime                 time.Time
	srcUpService              *srcUserPluginService
	srcRtDetails              *coreConfig.ServerDetails
	targetRtDetails           *coreConfig.ServerDetails
	progressBar               *progressbar.TransferProgressMng
}

func (m *migrationPhase) getSourceDetails() *coreConfig.ServerDetails {
	return m.srcRtDetails
}

func (m *migrationPhase) setProgressBar(progressbar *progressbar.TransferProgressMng) {
	m.progressBar = progressbar
}

func (m *migrationPhase) initProgressBar() error {
	if m.progressBar == nil {
		return nil
	}
	serviceManager, err := utils.CreateServiceManager(m.getSourceDetails(), -1, 0, false)
	if err != nil {
		return err
	}
	repoSummaryList, err := serviceManager.StorageInfo()
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
	return fmt.Errorf("repository: \"%s\" doesn't exists in Artifactory", m.repoKey)
}

func (m *migrationPhase) getPhaseName() string {
	return "Migration Phase"
}

func (m *migrationPhase) getPhaseId() int {
	return 0
}

func (m *migrationPhase) phaseStarted() error {
	m.startTime = time.Now()
	err := setRepoMigrationStarted(m.repoKey, m.startTime)
	if err != nil {
		return err
	}

	if !isPropertiesPhaseDisabled() {
		return m.srcUpService.storeProperties(m.repoKey)
	}
	return nil
}

func (m *migrationPhase) phaseDone() error {
	err := setRepoMigrationCompleted(m.repoKey)
	if err != nil {
		return err
	}
	return m.progressBar.DonePhase(m.getPhaseId())
}

func (m *migrationPhase) shouldCheckExistenceInFilestore(shouldCheck bool) {
	m.checkExistenceInFilestore = shouldCheck
}

func (m *migrationPhase) shouldSkipPhase() (bool, error) {
	skip, err := isRepoMigrated(m.repoKey)
	if err != nil {
		return false, err
	}
	if skip {
		m.skipPhase()
	}
	return skip, nil
}

func (m *migrationPhase) skipPhase() {
	// Init progress bar as "done" with 0 tasks.
	if m.progressBar != nil {
		m.progressBar.AddPhase1(0)
	}
}

func (m *migrationPhase) setSrcUserPluginService(service *srcUserPluginService) {
	m.srcUpService = service
}

func (m *migrationPhase) setSourceDetails(details *coreConfig.ServerDetails) {
	m.srcRtDetails = details
}

func (m *migrationPhase) setTargetDetails(details *coreConfig.ServerDetails) {
	m.targetRtDetails = details
}

func (m *migrationPhase) run() error {
	producerConsumer := parallel.NewRunner(getThreads(), tasksMaxCapacity, false)
	expectedChan := make(chan int, 1)
	errorsQueue := clientUtils.NewErrorsQueue(1)
	uploadTokensChan := make(chan string, tasksMaxCapacity)
	var runWaitGroup sync.WaitGroup
	// Done channel notifies the polling go routines that no more tasks are expected.
	doneChan := make(chan bool, 2)

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
		folderHandler := m.createFolderMigrationHandlerFunc(pcDetails)
		_, _ = producerConsumer.AddTaskWithError(folderHandler(folderParams{repoKey: m.repoKey, relativePath: "."}), errorsQueue.AddError)
	}()

	var pollingError error
	runWaitGroup.Add(1)
	go func() {
		defer runWaitGroup.Done()
		pollingError = pollUploads(m.srcUpService, uploadTokensChan, doneChan, m.progressBar, m.getPhaseId())
	}()

	var runnerErr error
	runWaitGroup.Add(1)
	go func() {
		defer runWaitGroup.Done()
		runnerErr = producerConsumer.DoneWhenAllIdle(15)
		doneChan <- true
		doneChan <- true
	}()
	// Blocked until finish consuming
	producerConsumer.Run()
	runWaitGroup.Wait()

	var returnedError error
	for _, err := range []error{runnerErr, pollingError, errorsQueue.GetError()} {
		if err != nil {
			log.Error(err)
			returnedError = err
		}
	}
	return returnedError
}

type folderMigrationHandlerFunc func(params folderParams) parallel.TaskFunc

type folderParams struct {
	repoKey      string
	relativePath string
}

func (m *migrationPhase) createFolderMigrationHandlerFunc(pcDetails producerConsumerDetails) folderMigrationHandlerFunc {
	return func(params folderParams) parallel.TaskFunc {
		return func(threadId int) error {
			logMsgPrefix := clientUtils.GetLogMsgPrefix(threadId, false)
			err := m.migrateFolder(params, logMsgPrefix, pcDetails)
			if err != nil {
				return err
			}
			return nil
		}
	}
}

func (m *migrationPhase) migrateFolder(params folderParams, logMsgPrefix string, pcDetails producerConsumerDetails) error {
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
			folderHandler := m.createFolderMigrationHandlerFunc(pcDetails)
			_, _ = pcDetails.producerConsumer.AddTaskWithError(folderHandler(folderParams{repoKey: params.repoKey, relativePath: newRelativePath}), pcDetails.errorsQueue.AddError)
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
					log.Error(err)
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
	return nil
}

func (m *migrationPhase) getDirectoryContentsAql(repoKey, relativePath string) (result *artifactoryUtils.AqlSearchResult, err error) {
	query := generateFolderContentsAqlQuery(repoKey, relativePath)
	return runAql(m.srcRtDetails, query)
}

func generateFolderContentsAqlQuery(repoKey, relativePath string) string {
	return fmt.Sprintf(`items.find({"type":"any","$or":[{"$and":[{"repo":"%s","path":{"$match":"%s"},"name":{"$match":"*"}}]}]}).include("repo","path","name","type")`, repoKey, relativePath)
}
