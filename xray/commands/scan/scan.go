package scan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/exp/slices"

	rtutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/scangraph"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"golang.org/x/sync/errgroup"

	"github.com/jfrog/gofrog/parallel"
	"github.com/jfrog/jfrog-cli-core/v2/common/format"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/formats"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/artifactory/services/fspatterns"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type FileContext func(string) parallel.TaskFunc
type indexFileHandlerFunc func(file string)

const (
	BypassArchiveLimitsMinXrayVersion = "3.59.0"
	indexingCommand                   = "graph"
	fileNotSupportedExitCode          = 3
)

type ScanCommand struct {
	serverDetails *config.ServerDetails
	spec          *spec.SpecFiles
	threads       int
	// The location of the downloaded Xray indexer binary on the local file system.
	indexerPath            string
	indexerTempDir         string
	outputFormat           format.OutputFormat
	projectKey             string
	minSeverityFilter      string
	watches                []string
	includeVulnerabilities bool
	includeLicenses        bool
	fail                   bool
	printExtendedTable     bool
	bypassArchiveLimits    bool
	fixableOnly            bool
	progress               ioUtils.ProgressMgr
}

func (scanCmd *ScanCommand) SetMinSeverityFilter(minSeverityFilter string) *ScanCommand {
	scanCmd.minSeverityFilter = minSeverityFilter
	return scanCmd
}

func (scanCmd *ScanCommand) SetFixableOnly(fixable bool) *ScanCommand {
	scanCmd.fixableOnly = fixable
	return scanCmd
}

func (scanCmd *ScanCommand) SetProgress(progress ioUtils.ProgressMgr) {
	scanCmd.progress = progress
}

func (scanCmd *ScanCommand) SetThreads(threads int) *ScanCommand {
	scanCmd.threads = threads
	return scanCmd
}

func (scanCmd *ScanCommand) SetOutputFormat(format format.OutputFormat) *ScanCommand {
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

func (scanCmd *ScanCommand) SetBypassArchiveLimits(bypassArchiveLimits bool) *ScanCommand {
	scanCmd.bypassArchiveLimits = bypassArchiveLimits
	return scanCmd
}

func (scanCmd *ScanCommand) indexFile(filePath string) (*xrayUtils.BinaryGraphNode, error) {
	var indexerResults xrayUtils.BinaryGraphNode
	indexerCmd := exec.Command(scanCmd.indexerPath, indexingCommand, filePath, "--temp-dir", scanCmd.indexerTempDir)
	if scanCmd.bypassArchiveLimits {
		indexerCmd.Args = append(indexerCmd.Args, "--bypass-archive-limits")
	}
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	indexerCmd.Stdout = &stdout
	indexerCmd.Stderr = &stderr
	err := indexerCmd.Run()
	if err != nil {
		var e *exec.ExitError
		if errors.As(err, &e) {
			if e.ExitCode() == fileNotSupportedExitCode {
				log.Debug(fmt.Sprintf("File %s is not supported by Xray indexer app.", filePath))
				return &indexerResults, nil
			}
		}
		return nil, errorutils.CheckErrorf("Xray indexer app failed indexing %s with %s: %s", filePath, err, stderr.String())
	}
	if stderr.String() != "" {
		log.Info(stderr.String())
	}
	err = json.Unmarshal(stdout.Bytes(), &indexerResults)
	return &indexerResults, errorutils.CheckError(err)
}

func (scanCmd *ScanCommand) Run() (err error) {
	defer func() {
		if err != nil {
			var e *exec.ExitError
			if errors.As(err, &e) {
				if e.ExitCode() != coreutils.ExitCodeVulnerableBuild.Code {
					err = errors.New("Scan command failed. " + err.Error())
				}
			}
		}
	}()
	xrayManager, xrayVersion, err := xrutils.CreateXrayServiceManagerAndGetVersion(scanCmd.serverDetails)
	if err != nil {
		return err
	}

	// Validate Xray minimum version for graph scan command
	err = clientutils.ValidateMinimumVersion(clientutils.Xray, xrayVersion, scangraph.GraphScanMinXrayVersion)
	if err != nil {
		return err
	}

	if scanCmd.bypassArchiveLimits {
		// Validate Xray minimum version for BypassArchiveLimits flag for indexer
		err = clientutils.ValidateMinimumVersion(clientutils.Xray, xrayVersion, BypassArchiveLimitsMinXrayVersion)
		if err != nil {
			return err
		}
	}
	log.Info("JFrog Xray version is:", xrayVersion)
	// First download Xray Indexer if needed
	scanCmd.indexerPath, err = DownloadIndexerIfNeeded(xrayManager, xrayVersion)
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

	// resultsArr is a two-dimensional array. Each array in it contains a list of ScanResponses that were requested and collected by a specific thread.
	resultsArr := make([][]*services.ScanResponse, threads)
	fileProducerConsumer := parallel.NewRunner(scanCmd.threads, 20000, false)
	fileProducerErrors := make([][]formats.SimpleJsonError, threads)
	indexedFileProducerConsumer := parallel.NewRunner(scanCmd.threads, 20000, false)
	indexedFileProducerErrors := make([][]formats.SimpleJsonError, threads)
	fileCollectingErrorsQueue := clientutils.NewErrorsQueue(1)
	// Start walking on the filesystem to "produce" files that match the given pattern
	// while the consumer uses the indexer to index those files.
	scanCmd.prepareScanTasks(fileProducerConsumer, indexedFileProducerConsumer, resultsArr, fileProducerErrors, indexedFileProducerErrors, fileCollectingErrorsQueue, xrayVersion)
	scanCmd.performScanTasks(fileProducerConsumer, indexedFileProducerConsumer)

	// Handle results
	flatResults := []services.ScanResponse{}
	for _, arr := range resultsArr {
		for _, res := range arr {
			flatResults = append(flatResults, *res)
		}
	}
	if scanCmd.progress != nil {
		if err = scanCmd.progress.Quit(); err != nil {
			return err
		}

	}

	fileCollectingErr := fileCollectingErrorsQueue.GetError()
	var scanErrors []formats.SimpleJsonError
	if fileCollectingErr != nil {
		scanErrors = append(scanErrors, formats.SimpleJsonError{ErrorMessage: fileCollectingErr.Error()})
	}
	scanErrors = appendErrorSlice(scanErrors, fileProducerErrors)
	scanErrors = appendErrorSlice(scanErrors, indexedFileProducerErrors)

	scanResults := xrutils.NewAuditResults()
	scanResults.XrayVersion = xrayVersion
	scanResults.ScaResults = []xrutils.ScaScanResult{{XrayResults: flatResults}}

	scanResults.ExtendedScanResults.EntitledForJas, err = xrutils.IsEntitledForJas(xrayManager, xrayVersion)
	errGroup := new(errgroup.Group)
	if scanResults.ExtendedScanResults.EntitledForJas {
		// Download (if needed) the analyzer manager in a background routine.
		errGroup.Go(rtutils.DownloadAnalyzerManagerIfNeeded)
	}
	// Wait for the Download of the AnalyzerManager to complete.
	if err = errGroup.Wait(); err != nil {
		err = errors.New("failed while trying to get Analyzer Manager: " + err.Error())
	}

	if scanResults.ExtendedScanResults.EntitledForJas {
		cveList := cveListFromVulnerabilities(flatResults)
		workingDirs := []string{scanCmd.spec.Files[0].Pattern}
		scanResults.JasError = runJasScannersAndSetResults(scanResults, cveList, scanCmd.serverDetails, workingDirs)
	}

	if err = xrutils.NewResultsWriter(scanResults).
		SetOutputFormat(scanCmd.outputFormat).
		SetIncludeVulnerabilities(scanCmd.includeVulnerabilities).
		SetIncludeLicenses(scanCmd.includeLicenses).
		SetPrintExtendedTable(scanCmd.printExtendedTable).
		SetIsMultipleRootProject(true).
		SetScanType(services.Binary).
		PrintScanResults(); err != nil {
		return
	}

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
	if len(scanErrors) > 0 {
		return errorutils.CheckErrorf(scanErrors[0].ErrorMessage)
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

func (scanCmd *ScanCommand) prepareScanTasks(fileProducer, indexedFileProducer parallel.Runner, resultsArr [][]*services.ScanResponse, fileErrors, indexedFileErrors [][]formats.SimpleJsonError, fileCollectingErrorsQueue *clientutils.ErrorsQueue, xrayVersion string) {
	go func() {
		defer fileProducer.Done()
		// Iterate over file-spec groups and produce indexing tasks.
		// When encountering an error, log and move to next group.
		specFiles := scanCmd.spec.Files
		for i := range specFiles {
			artifactHandlerFunc := scanCmd.createIndexerHandlerFunc(&specFiles[i], indexedFileProducer, resultsArr, fileErrors, indexedFileErrors, xrayVersion)
			taskHandler := getAddTaskToProducerFunc(fileProducer, artifactHandlerFunc)

			err := collectFilesForIndexing(specFiles[i], taskHandler)
			if err != nil {
				log.Error(err)
				fileCollectingErrorsQueue.AddError(err)
			}
		}
	}()
}

func (scanCmd *ScanCommand) createIndexerHandlerFunc(file *spec.File, indexedFileProducer parallel.Runner, resultsArr [][]*services.ScanResponse, fileErrors, indexedFileErrors [][]formats.SimpleJsonError, xrayVersion string) FileContext {
	return func(filePath string) parallel.TaskFunc {
		return func(threadId int) (err error) {
			logMsgPrefix := clientutils.GetLogMsgPrefix(threadId, false)
			log.Info(logMsgPrefix+"Indexing file:", filePath)
			if scanCmd.progress != nil {
				scanCmd.progress.SetHeadlineMsg("Indexing file: " + filepath.Base(filePath) + " 🗄")
			}
			graph, err := scanCmd.indexFile(filePath)
			if err != nil {
				fileErrors[threadId] = append(fileErrors[threadId], formats.SimpleJsonError{FilePath: filePath, ErrorMessage: err.Error()})
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
				params := &services.XrayGraphScanParams{
					BinaryGraph:            graph,
					RepoPath:               getXrayRepoPathFromTarget(file.Target),
					Watches:                scanCmd.watches,
					IncludeLicenses:        scanCmd.includeLicenses,
					IncludeVulnerabilities: scanCmd.includeVulnerabilities,
					ProjectKey:             scanCmd.projectKey,
					ScanType:               services.Binary,
				}
				if scanCmd.progress != nil {
					scanCmd.progress.SetHeadlineMsg("Scanning 🔍")
				}
				scanGraphParams := scangraph.NewScanGraphParams().
					SetServerDetails(scanCmd.serverDetails).
					SetXrayGraphScanParams(params).
					SetXrayVersion(xrayVersion).
					SetFixableOnly(scanCmd.fixableOnly).
					SetSeverityLevel(scanCmd.minSeverityFilter)
				scanResults, err := scangraph.RunScanGraphAndGetResults(scanGraphParams)
				if err != nil {
					log.Error(fmt.Sprintf("scanning '%s' failed with error: %s", graph.Id, err.Error()))
					indexedFileErrors[threadId] = append(indexedFileErrors[threadId], formats.SimpleJsonError{FilePath: filePath, ErrorMessage: err.Error()})
					return
				}
				resultsArr[threadId] = append(resultsArr[threadId], scanResults)
				return
			}

			_, _ = indexedFileProducer.AddTask(taskFunc)
			return
		}
	}
}

func getAddTaskToProducerFunc(producer parallel.Runner, fileHandlerFunc FileContext) indexFileHandlerFunc {
	return func(filePath string) {
		taskFunc := fileHandlerFunc(filePath)
		_, _ = producer.AddTask(taskFunc)
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
	excludePathPattern := fspatterns.PrepareExcludePathPattern(fileParams.Exclusions, fileParams.GetPatternType(), fileParams.IsRecursive())
	patternRegex, err := regexp.Compile(fileData.Pattern)
	if errorutils.CheckError(err) != nil {
		return err
	}
	recursive, err := fileData.IsRecursive(true)
	if err != nil {
		return err
	}

	paths, err := fspatterns.ListFiles(rootPath, recursive, false, false, false, excludePathPattern)
	if err != nil {
		return err
	}
	for _, path := range paths {
		matches, isDir, err := fspatterns.SearchPatterns(path, false, false, patternRegex)
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

func appendErrorSlice(scanErrors []formats.SimpleJsonError, errorsToAdd [][]formats.SimpleJsonError) []formats.SimpleJsonError {
	for _, errorSlice := range errorsToAdd {
		scanErrors = append(scanErrors, errorSlice...)
	}
	return scanErrors
}

func cveListFromVulnerabilities(flatResults []services.ScanResponse) []string {
	var cveList []string
	var technologiesList []string
	for _, result := range flatResults {
		for _, vulnerability := range result.Vulnerabilities {
			for _, cve := range vulnerability.Cves {
				if !slices.Contains(cveList, cve.Id) && (cve.Id != "") {
					cveList = append(cveList, cve.Id)
				}
			}
			if !slices.Contains(technologiesList, vulnerability.Technology) && (vulnerability.Technology != "") {
				technologiesList = append(technologiesList, vulnerability.Technology)
			}
		}
	}
	return cveList
}
