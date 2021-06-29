package audit

import (
	"encoding/json"
	"os"
	"regexp"

	"github.com/jfrog/gofrog/io"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/artifactory/spec"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/xray/commands"
	xrutils "github.com/jfrog/jfrog-cli-core/xray/utils"
	"github.com/jfrog/jfrog-client-go/artifactory/services/fspatterns"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	IndexerExecutionName = "indexer-app"
	IndexingCommand      = "graph"
)

type FileContext func(string) parallel.TaskFunc
type indexFileHandlerFunc func(file string)

type XrBinariesScanCommand struct {
	serverDetails *config.ServerDetails
	spec          *spec.SpecFiles
	threads       int
	// The location on the local file system of the downloaded Xray's indexer.
	indexerPath string
	// A path in Artifactory that the Artifacts are intended to be deployed to.
	// This will provide a way to Xray to extract the watches that should be applied on this scan request.
	deployedRepoPath string
	printResults     bool
}

func (scanCmd *XrBinariesScanCommand) SetThreads(threads int) *XrBinariesScanCommand {
	scanCmd.threads = threads
	return scanCmd
}

func (scanCmd *XrBinariesScanCommand) SetServerDetails(server *config.ServerDetails) *XrBinariesScanCommand {
	scanCmd.serverDetails = server
	return scanCmd
}

func (scanCmd *XrBinariesScanCommand) SetSpec(spec *spec.SpecFiles) *XrBinariesScanCommand {
	scanCmd.spec = spec
	return scanCmd
}

func (scanCmd *XrBinariesScanCommand) ServerDetails() (*config.ServerDetails, error) {
	return scanCmd.serverDetails, nil
}

func (scanCmd *XrBinariesScanCommand) SetDeployedRepoPath(deployedRepoPath string) *XrBinariesScanCommand {
	scanCmd.deployedRepoPath = deployedRepoPath
	return scanCmd
}

func (scanCmd *XrBinariesScanCommand) IndexFile(filePath string) (*services.GraphNode, error) {
	var indexerResults services.GraphNode
	indexCmd := &coreutils.GeneralExecCmd{
		ExecPath: scanCmd.indexerPath,
		Command:  []string{IndexingCommand, filePath},
	}
	output, err := io.RunCmdOutput(indexCmd)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(output), &indexerResults)
	return &indexerResults, err

}

func (scanCmd *XrBinariesScanCommand) GetXrScanGraphResults(graph *services.GraphNode) (*services.ScanResponse, error) {
	xrayManager, err := commands.CreateXrayServiceManager(scanCmd.serverDetails)
	if err != nil {
		return nil, err
	}
	params := services.NewXrayGraphScanParams()
	params.RepoPath = scanCmd.deployedRepoPath
	params.Graph = graph
	scanId, err := xrayManager.ScanGraph(params)
	if err != nil {
		return nil, err
	}
	scanResults, err := xrayManager.GetScanGraphResults(scanId)
	if err != nil {
		return nil, err
	}
	return scanResults, nil
}

func (scanCmd *XrBinariesScanCommand) Run() (err error) {
	scanCmd.printResults = true
	_, err = scanCmd.DoScan()
	return
}

func (scanCmd *XrBinariesScanCommand) DoScan() (pass bool, err error) {
	// First download Xray Indexer if needed
	xrayManager, err := commands.CreateXrayServiceManager(scanCmd.serverDetails)
	if err != nil {
		return false, err
	}
	scanCmd.indexerPath, err = xrutils.DownloadIndexerIfNeeded(xrayManager)
	if err != nil {
		return false, err
	}
	resultsArr := make([][]*services.ScanResponse, scanCmd.threads)
	fileProducerConsumer := parallel.NewRunner(scanCmd.threads, 20000, false)
	fileProducerErrorsQueue := clientutils.NewErrorsQueue(1)
	indexedFileProducerConsumer := parallel.NewRunner(scanCmd.threads, 20000, false)
	indexedFileProducerErrorsQueue := clientutils.NewErrorsQueue(1)
	// Start walk on the Filesystem "produce" files that match the given pattern
	// while the consumer uses the indexer to index those files.
	scanCmd.prepareScanTasks(fileProducerConsumer, indexedFileProducerConsumer, resultsArr, fileProducerErrorsQueue, indexedFileProducerErrorsQueue)
	scanOk := scanCmd.performScanTasks(fileProducerConsumer, indexedFileProducerConsumer, resultsArr)
	err = fileProducerErrorsQueue.GetError()
	if err != nil {
		return false, err
	}
	err = indexedFileProducerErrorsQueue.GetError()
	if err != nil {
		return false, err
	}

	return scanOk, nil
}

func NewXrBinariesScanCommand() *XrBinariesScanCommand {
	return &XrBinariesScanCommand{}
}

func (scanCmd *XrBinariesScanCommand) CommandName() string {
	return "xr_scan"
}

func (scanCmd *XrBinariesScanCommand) prepareScanTasks(fileProducer, indexedFileProducer parallel.Runner, resultsArr [][]*services.ScanResponse, fileErrorsQueue, indexedFileErrorsQueue *clientutils.ErrorsQueue) {
	go func() {
		defer fileProducer.Done()
		// Iterate over file-spec groups and produce indexing tasks.
		// When encountering an error, log and move to next group.
		for _, fileGroup := range scanCmd.spec.Files {

			artifactHandlerFunc := scanCmd.createIndexerHandlerFunc(&fileGroup, indexedFileProducer, resultsArr, indexedFileErrorsQueue)
			taskHandler := getAddTaskToProducerFunc(fileProducer, fileErrorsQueue, artifactHandlerFunc)

			err := collectFilesForIndexing(fileGroup, taskHandler)
			if err != nil {
				log.Error(err)
				fileErrorsQueue.AddError(err)
			}
		}
	}()
}

func (scanCmd *XrBinariesScanCommand) createIndexerHandlerFunc(file *spec.File, indexedFileProducer parallel.Runner, resultsArr [][]*services.ScanResponse, errorsQueue *clientutils.ErrorsQueue) FileContext {
	return func(filePath string) parallel.TaskFunc {
		return func(threadId int) (err error) {

			logMsgPrefix := clientutils.GetLogMsgPrefix(threadId, false)
			fileInfo, e := os.Lstat(filePath)
			if errorutils.CheckError(e) != nil {
				return
			}
			log.Info(logMsgPrefix+"Indexing file:", fileInfo.Name())
			graph, err := scanCmd.IndexFile(filePath)
			if err != nil {
				return err
			}
			// Add a new task to the seconde prodicer/consumer
			// which will send the indexed binary to Xray and then will store the given result.
			taskFunc := func(threadId int) (err error) {
				scanResults, err := scanCmd.GetXrScanGraphResults(graph)
				if err != nil {
					return err
				}
				resultsArr[threadId] = append(resultsArr[threadId], scanResults)
				return
			}

			indexedFileProducer.AddTaskWithError(taskFunc, errorsQueue.AddError)
			return
		}
	}
}

func getAddTaskToProducerFunc(producer parallel.Runner, errorsQueue *clientutils.ErrorsQueue, fileHandlerFunc FileContext) indexFileHandlerFunc {
	return func(filePath string) {
		taskFunc := fileHandlerFunc(filePath)
		producer.AddTaskWithError(taskFunc, errorsQueue.AddError)
	}
}

func (scanCmd *XrBinariesScanCommand) performScanTasks(fileConsumer parallel.Runner, indexedFileConsumer parallel.Runner, resultsArr [][]*services.ScanResponse) bool {

	go func() {
		// Blocking until consuming is finished.
		fileConsumer.Run()
		// After all files has been indexed, The seconde producer notifies that no more tasks will be produced.
		indexedFileConsumer.Done()
	}()
	// Blocking until consuming is finished.
	indexedFileConsumer.Run()
	// Handle results
	passScan := true
	violations := []services.Violation{}
	vulnerabilities := []services.Vulnerability{}
	for _, arr := range resultsArr {
		for _, res := range arr {
			if scanCmd.printResults {
				violations = append(violations, res.Violations...)
				vulnerabilities = append(vulnerabilities, res.Vulnerabilities...)
			}
			if len(res.Violations) > 0 {
				// A violation was found, the scan failed.
				passScan = false
			}
		}
	}
	if len(violations) > 0 {
		xrutils.PrintViolationsTable(violations)
	}
	if len(vulnerabilities) > 0 {
		xrutils.PrintVulnerabilitiesTable(vulnerabilities)
	}
	// No violations found, return scan OK.
	return passScan

}

func collectFilesForIndexing(fileData spec.File, dataHandlerFunc indexFileHandlerFunc) error {

	fileData.Pattern = (clientutils.ReplaceTildeWithUserHome(fileData.Pattern))
	// Save parentheses index in pattern, witch have corresponding placeholder.
	patternType := clientutils.WildCardPattern
	if regex, _ := fileData.IsRegexp(false); regex {
		patternType = clientutils.RegExp
	}
	if ant, _ := fileData.IsAnt(false); ant {
		patternType = clientutils.AntPattern
	}

	rootPath, err := fspatterns.GetRootPath(fileData.Pattern, fileData.Target, patternType, false)
	if err != nil {
		return err
	}

	isDir, err := fileutils.IsDirExists(rootPath, false)
	if err != nil {
		return err
	}

	// If the path is a single file, index it and return
	if !isDir {
		dataHandlerFunc(rootPath)
		return nil
	}
	fileData.Pattern = clientutils.PrepareLocalPathForUpload(fileData.Pattern, patternType)
	err = collectPatternMatchingFiles(fileData, rootPath, dataHandlerFunc)
	return err
}

func collectPatternMatchingFiles(fileData spec.File, rootPath string, dataHandlerFunc indexFileHandlerFunc) error {
	fileParams, err := fileData.ToArtifactoryCommonParams()
	if err != nil {
		return err
	}
	excludePathPattern := fspatterns.PrepareExcludePathPattern(fileParams)
	patternRegex, err := regexp.Compile(fileData.Pattern)
	if errorutils.CheckError(err) != nil {
		return err
	}

	paths, err := fspatterns.GetPaths(rootPath, true, false, false)
	if err != nil {
		return err
	}
	for _, path := range paths {
		matches, isDir, _, err := fspatterns.PrepareAndFilterPaths(path, excludePathPattern, false, false, patternRegex)
		if err != nil {
			return err
		}
		// Because paths should contains all files and directories (walks recursively) we can ignore dirs, as only files relevance for indexing.
		if isDir {
			continue
		}
		if matches != nil && len(matches) > 0 {
			dataHandlerFunc(path)
		}
	}
	return nil
}

func printTable(res *services.ScanResponse) error {
	jsonOut, err := json.Marshal(res)
	print(string(jsonOut))
	return err
}
