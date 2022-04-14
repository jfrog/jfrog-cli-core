package scan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/artifactory/services/fspatterns"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type FileContext func(string) parallel.TaskFunc
type indexFileHandlerFunc func(file string)

const (
	indexingCommand          = "graph"
	fileNotSupportedExitCode = 3
)

type ScanCommand struct {
	serverDetails *config.ServerDetails
	spec          *spec.SpecFiles
	threads       int
	// The location of the downloaded Xray indexer binary on the local file system.
	indexerPath            string
	indexerTempDir         string
	outputFormat           xrutils.OutputFormat
	projectKey             string
	watches                []string
	includeVulnerabilities bool
	includeLicenses        bool
	fail                   bool
	printExtendedTable     bool
	progress               ioUtils.ProgressMgr
}

func (scanCmd *ScanCommand) SetProgress(progress ioUtils.ProgressMgr) {
	scanCmd.progress = progress
}

func (scanCmd *ScanCommand) SetThreads(threads int) *ScanCommand {
	scanCmd.threads = threads
	return scanCmd
}

func (scanCmd *ScanCommand) SetOutputFormat(format xrutils.OutputFormat) *ScanCommand {
	scanCmd.outputFormat = format
	return scanCmd
}

func (scanCmd *ScanCommand) SetServerDetails(server *config.ServerDetails) *ScanCommand {
	scanCmd.serverDetails = server
	return scanCmd
}

func (scanCmd *ScanCommand) SetSpec(spec *spec.SpecFiles) *ScanCommand {
	scanCmd.spec = spec
	return scanCmd
}

func (scanCmd *ScanCommand) SetProject(project string) *ScanCommand {
	scanCmd.projectKey = project
	return scanCmd
}

func (scanCmd *ScanCommand) SetWatches(watches []string) *ScanCommand {
	scanCmd.watches = watches
	return scanCmd
}

func (scanCmd *ScanCommand) SetIncludeVulnerabilities(include bool) *ScanCommand {
	scanCmd.includeVulnerabilities = include
	return scanCmd
}

func (scanCmd *ScanCommand) SetIncludeLicenses(include bool) *ScanCommand {
	scanCmd.includeLicenses = include
	return scanCmd
}

func (scanCmd *ScanCommand) ServerDetails() (*config.ServerDetails, error) {
	return scanCmd.serverDetails, nil
}

func (scanCmd *ScanCommand) SetFail(fail bool) *ScanCommand {
	scanCmd.fail = fail
	return scanCmd
}

func (scanCmd *ScanCommand) SetPrintExtendedTable(printExtendedTable bool) *ScanCommand {
	scanCmd.printExtendedTable = printExtendedTable
	return scanCmd
}

func (scanCmd *ScanCommand) indexFile(filePath string) (*services.GraphNode, error) {
	var indexerResults services.GraphNode
	indexerCmd := exec.Command(scanCmd.indexerPath, indexingCommand, filePath, "--temp-dir", scanCmd.indexerTempDir)
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	indexerCmd.Stdout = &stdout
	indexerCmd.Stderr = &stderr
	err := indexerCmd.Run()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			if e.ExitCode() == fileNotSupportedExitCode {
				log.Debug(fmt.Sprintf("File %s is not supported by Xray indexer app.", filePath))
				return &indexerResults, nil
			}
		}
		return nil, errorutils.CheckErrorf("Xray indexer app failed indexing %s with %s: %s", filePath, err, stderr.String())
	}
	log.Info(stderr.String())
	err = json.Unmarshal(stdout.Bytes(), &indexerResults)
	return &indexerResults, errorutils.CheckError(err)
}

func (scanCmd *ScanCommand) Run() (err error) {
	defer func() {
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				if e.ExitCode() != coreutils.ExitCodeVulnerableBuild.Code {
					err = errors.New("Scan command failed. " + err.Error())
				}
			}
		}
	}()
	// Validate Xray minimum version
	xrayManager, xrayVersion, err := commands.CreateXrayServiceManagerAndGetVersion(scanCmd.serverDetails)
	if err != nil {
		return err
	}
	err = commands.ValidateXrayMinimumVersion(xrayVersion, commands.GraphScanMinXrayVersion)
	if err != nil {
		return err
	}
	// First download Xray Indexer if needed
	scanCmd.indexerPath, err = xrutils.DownloadIndexerIfNeeded(xrayManager, xrayVersion)
	if err != nil {
		return err
	}
	// Create Temp dir for Xray Indexer
	scanCmd.indexerTempDir, err = fileutils.CreateTempDir()
	if err != nil {
		return err
	}
	defer func() {
		e := fileutils.RemoveTempDir(scanCmd.indexerTempDir)
		if err == nil {
			err = e
		}
	}()
	threads := 1
	if scanCmd.threads > 1 {
		threads = scanCmd.threads
	}
	resultsArr := make([][]*services.ScanResponse, threads)
	fileProducerConsumer := parallel.NewRunner(scanCmd.threads, 20000, false)
	fileProducerErrorsQueue := clientutils.NewErrorsQueue(1)
	indexedFileProducerConsumer := parallel.NewRunner(scanCmd.threads, 20000, false)
	indexedFileProducerErrorsQueue := clientutils.NewErrorsQueue(1)
	// Start walking on the filesystem to "produce" files that match the given pattern
	// while the consumer uses the indexer to index those files.
	scanCmd.prepareScanTasks(fileProducerConsumer, indexedFileProducerConsumer, resultsArr, fileProducerErrorsQueue, indexedFileProducerErrorsQueue, xrayVersion)
	scanCmd.performScanTasks(fileProducerConsumer, indexedFileProducerConsumer)

	// Handle results
	flatResults := []services.ScanResponse{}
	for _, arr := range resultsArr {
		for _, res := range arr {
			flatResults = append(flatResults, *res)
		}
	}
	if scanCmd.progress != nil {
		scanCmd.progress.ClearHeadlineMsg()
	}
	err = xrutils.PrintScanResults(flatResults, scanCmd.outputFormat, scanCmd.includeVulnerabilities, scanCmd.includeLicenses, true, scanCmd.printExtendedTable)
	if err != nil {
		return err
	}
	// If includeVulnerabilities is false it means that context was provided, so we need to check for build violations.
	// If user provided --fail=false, don't fail the build.
	if scanCmd.fail && !scanCmd.includeVulnerabilities {
		if xrutils.CheckIfFailBuild(flatResults) {
			return xrutils.NewFailBuildError()
		}
	}
	err = fileProducerErrorsQueue.GetError()
	if err != nil {
		return err
	}
	err = indexedFileProducerErrorsQueue.GetError()
	if err != nil {
		return err
	}
	log.Info("Scan completed successfully.")
	return nil
}

func NewScanCommand() *ScanCommand {
	return &ScanCommand{}
}

func (scanCmd *ScanCommand) CommandName() string {
	return "xr_scan"
}

func (scanCmd *ScanCommand) prepareScanTasks(fileProducer, indexedFileProducer parallel.Runner, resultsArr [][]*services.ScanResponse, fileErrorsQueue, indexedFileErrorsQueue *clientutils.ErrorsQueue, xrayVersion string) {
	go func() {
		defer fileProducer.Done()
		// Iterate over file-spec groups and produce indexing tasks.
		// When encountering an error, log and move to next group.
		specFiles := scanCmd.spec.Files
		for i := range specFiles {
			artifactHandlerFunc := scanCmd.createIndexerHandlerFunc(&specFiles[i], indexedFileProducer, resultsArr, indexedFileErrorsQueue, xrayVersion)
			taskHandler := getAddTaskToProducerFunc(fileProducer, fileErrorsQueue, artifactHandlerFunc)

			err := collectFilesForIndexing(specFiles[i], taskHandler)
			if err != nil {
				log.Error(err)
				fileErrorsQueue.AddError(err)
			}
		}
	}()
}

func (scanCmd *ScanCommand) createIndexerHandlerFunc(file *spec.File, indexedFileProducer parallel.Runner, resultsArr [][]*services.ScanResponse, errorsQueue *clientutils.ErrorsQueue, xrayVersion string) FileContext {
	return func(filePath string) parallel.TaskFunc {
		return func(threadId int) (err error) {
			logMsgPrefix := clientutils.GetLogMsgPrefix(threadId, false)
			log.Info(logMsgPrefix+"Indexing file:", filePath)
			if scanCmd.progress != nil {
				scanCmd.progress.SetHeadlineMsg("Indexing file: " + filepath.Base(filePath) + " 🗄")
			}
			graph, err := scanCmd.indexFile(filePath)
			if err != nil {
				return err
			}
			// In case of empty graph returned by the indexer,
			// for instance due to unsupported file format, continue without sending a
			// graph request to Xray.
			if graph.Id == "" {
				return nil
			}
			// Add a new task to the second producer/consumer
			// which will send the indexed binary to Xray and then will store the received result.
			taskFunc := func(threadId int) (err error) {
				params := services.XrayGraphScanParams{
					Graph:      graph,
					RepoPath:   getXrayRepoPathFromTarget(file.Target),
					Watches:    scanCmd.watches,
					ProjectKey: scanCmd.projectKey,
					ScanType:   services.Binary,
				}
				if scanCmd.progress != nil {
					scanCmd.progress.SetHeadlineMsg("Scanning 🔍")
				}
				scanResults, err := commands.RunScanGraphAndGetResults(scanCmd.serverDetails, params, scanCmd.includeVulnerabilities, scanCmd.includeLicenses, xrayVersion)
				if err != nil {
					log.Error(fmt.Sprintf("Scanning %s failed with error: %s", graph.Id, err.Error()))
					return
				}
				resultsArr[threadId] = append(resultsArr[threadId], scanResults)
				return
			}

			_, _ = indexedFileProducer.AddTaskWithError(taskFunc, errorsQueue.AddError)
			return
		}
	}
}

func getAddTaskToProducerFunc(producer parallel.Runner, errorsQueue *clientutils.ErrorsQueue, fileHandlerFunc FileContext) indexFileHandlerFunc {
	return func(filePath string) {
		taskFunc := fileHandlerFunc(filePath)
		_, _ = producer.AddTaskWithError(taskFunc, errorsQueue.AddError)
	}
}

func (scanCmd *ScanCommand) performScanTasks(fileConsumer parallel.Runner, indexedFileConsumer parallel.Runner) {
	go func() {
		// Blocking until consuming is finished.
		fileConsumer.Run()
		// After all files have been indexed, The second producer notifies that no more tasks will be produced.
		indexedFileConsumer.Done()
	}()
	// Blocking until consuming is finished.
	indexedFileConsumer.Run()
}

func collectFilesForIndexing(fileData spec.File, dataHandlerFunc indexFileHandlerFunc) error {

	fileData.Pattern = clientutils.ReplaceTildeWithUserHome(fileData.Pattern)
	patternType := fileData.GetPatternType()
	rootPath, err := fspatterns.GetRootPath(fileData.Pattern, fileData.Target, "", patternType, false)
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
	fileData.Pattern = clientutils.ConvertLocalPatternToRegexp(fileData.Pattern, patternType)
	return collectPatternMatchingFiles(fileData, rootPath, dataHandlerFunc)
}

func collectPatternMatchingFiles(fileData spec.File, rootPath string, dataHandlerFunc indexFileHandlerFunc) error {
	fileParams, err := fileData.ToCommonParams()
	if err != nil {
		return err
	}
	excludePathPattern := fspatterns.PrepareExcludePathPattern(fileParams)
	patternRegex, err := regexp.Compile(fileData.Pattern)
	if errorutils.CheckError(err) != nil {
		return err
	}
	recursive, err := fileData.IsRecursive(true)
	if err != nil {
		return err
	}

	paths, err := fspatterns.GetPaths(rootPath, recursive, false, false)
	if err != nil {
		return err
	}
	for _, path := range paths {
		matches, isDir, _, err := fspatterns.PrepareAndFilterPaths(path, excludePathPattern, false, false, patternRegex)
		if err != nil {
			return err
		}
		// Because paths should contain all files and directories (walks recursively) we can ignore dirs, as only files relevance for indexing.
		if isDir {
			continue
		}
		if len(matches) > 0 {
			dataHandlerFunc(path)
		}
	}
	return nil
}

// Xray expects a path inside a repo, but does not accept a path to a file.
// Therefore, if the given target path is a path to a file,
// the path to the parent directory will be returned.
// Otherwise, the func will return the path itself.
func getXrayRepoPathFromTarget(target string) (repoPath string) {
	if strings.HasSuffix(target, "/") {
		return target
	}
	return target[:strings.LastIndex(target, "/")+1]
}
