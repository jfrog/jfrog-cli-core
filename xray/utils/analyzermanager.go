package utils

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/v2/sarif"
)

var (
	// Default is Medium for all other values
	levelToSeverity = map[string]string{"error": "High", "note": "Low", "none": "Unknown"}
)

const (
	EntitlementsMinVersion        = "3.66.5"
	ApplicabilityFeatureId        = "contextual_analysis"
	AnalyzerManagerZipName        = "analyzerManager.zip"
	analyzerManagerVersion        = "1.2.4.1953469"
	analyzerManagerDownloadPath   = "xsc-gen-exe-analyzer-manager-local/v1"
	analyzerManagerDirName        = "analyzerManager"
	analyzerManagerExecutableName = "analyzerManager"
	analyzerManagerLogDirName     = "analyzerManagerLogs"
	jfUserEnvVariable             = "JF_USER"
	jfPasswordEnvVariable         = "JF_PASS"
	jfTokenEnvVariable            = "JF_TOKEN"
	jfPlatformUrlEnvVariable      = "JF_PLATFORM_URL"
	logDirEnvVariable             = "AM_LOG_DIRECTORY"
	SeverityDefaultValue          = "Medium"
	notEntitledExitCode           = 31
	unsupportedCommandExitCode    = 13
	unsupportedOsExitCode         = 55
	ErrFailedScannerRun           = "failed to run %s scan. Exit code received: %s"
)

const (
	ApplicableStringValue                = "Applicable"
	NotApplicableStringValue             = "Not Applicable"
	ApplicabilityUndeterminedStringValue = "Undetermined"
)

type ScanType string

const (
	Applicability ScanType = "Applicability"
	Secrets       ScanType = "Secrets"
	IaC           ScanType = "IaC"
	ZeroDay       ScanType = "ZeroDay"
)

func (st ScanType) FormattedError(err error) error {
	if err != nil {
		return fmt.Errorf(ErrFailedScannerRun, st, err.Error())
	}
	return nil
}

var exitCodeErrorsMap = map[int]string{
	notEntitledExitCode:        "got not entitled error from analyzer manager",
	unsupportedCommandExitCode: "got unsupported scan command error from analyzer manager",
	unsupportedOsExitCode:      "got unsupported operating system error from analyzer manager",
}

type SourceCodeLocation struct {
	File       string
	LineColumn string
	Text       string
}

type SourceCodeScanResult struct {
	SourceCodeLocation
	Severity string
	Type     string
	CodeFlow [][]SourceCodeLocation
}

type ExtendedScanResults struct {
	XrayResults              []services.ScanResponse
	ScannedTechnologies      []coreutils.Technology
	ApplicabilityScanResults map[string]string
	SecretsScanResults       []SourceCodeScanResult
	IacScanResults           []SourceCodeScanResult
	ZeroDayResults           []SourceCodeScanResult
	EntitledForJas           bool
}

func (e *ExtendedScanResults) getXrayScanResults() []services.ScanResponse {
	return e.XrayResults
}

type AnalyzerManager struct {
	AnalyzerManagerFullPath string
}

func (am *AnalyzerManager) GetAnalyzerManagerDir() string {
	return filepath.Dir(am.AnalyzerManagerFullPath)
}

func (am *AnalyzerManager) Exec(configFile, scanCommand, workingDir string, serverDetails *config.ServerDetails) (err error) {
	if err = SetAnalyzerManagerEnvVariables(serverDetails); err != nil {
		return err
	}
	cmd := exec.Command(am.AnalyzerManagerFullPath, scanCommand, configFile)
	defer func() {
		if !cmd.ProcessState.Exited() {
			if killProcessError := cmd.Process.Kill(); errorutils.CheckError(killProcessError) != nil {
				err = errors.Join(err, killProcessError)
			}
		}
	}()
	cmd.Dir = workingDir
	err = cmd.Run()
	return errorutils.CheckError(err)
}

func GetAnalyzerManagerDownloadPath() (string, error) {
	osAndArc, err := coreutils.GetOSAndArc()
	if err != nil {
		return "", err
	}
	return path.Join(analyzerManagerDownloadPath, analyzerManagerVersion, osAndArc, AnalyzerManagerZipName), nil
}

func GetAnalyzerManagerDirAbsolutePath() (string, error) {
	jfrogDir, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(jfrogDir, analyzerManagerDirName), nil
}

func GetAnalyzerManagerExecutable() (analyzerManagerPath string, err error) {
	analyzerManagerDir, err := GetAnalyzerManagerDirAbsolutePath()
	if err != nil {
		return "", err
	}
	analyzerManagerPath = filepath.Join(analyzerManagerDir, GetAnalyzerManagerExecutableName())
	var exists bool
	if exists, err = fileutils.IsFileExists(analyzerManagerPath, false); err != nil {
		return
	}
	if !exists {
		err = errors.New("unable to locate the analyzer manager package. Advanced security scans cannot be performed without this package")
	}
	return analyzerManagerPath, err
}

func GetAnalyzerManagerExecutableName() string {
	analyzerManager := analyzerManagerExecutableName
	if coreutils.IsWindows() {
		return analyzerManager + ".exe"
	}
	return analyzerManager
}

func SetAnalyzerManagerEnvVariables(serverDetails *config.ServerDetails) error {
	if serverDetails == nil {
		return errors.New("cant get xray server details")
	}
	if err := os.Setenv(jfUserEnvVariable, serverDetails.User); errorutils.CheckError(err) != nil {
		return err
	}
	if err := os.Setenv(jfPasswordEnvVariable, serverDetails.Password); errorutils.CheckError(err) != nil {
		return err
	}
	if err := os.Setenv(jfPlatformUrlEnvVariable, serverDetails.Url); errorutils.CheckError(err) != nil {
		return err
	}
	if err := os.Setenv(jfTokenEnvVariable, serverDetails.AccessToken); errorutils.CheckError(err) != nil {
		return err
	}
	analyzerManagerLogFolder, err := coreutils.CreateDirInJfrogHome(filepath.Join(coreutils.JfrogLogsDirName, analyzerManagerLogDirName))
	if err != nil {
		return err
	}
	if err = os.Setenv(logDirEnvVariable, analyzerManagerLogFolder); errorutils.CheckError(err) != nil {
		return err
	}
	return nil
}

func ParseAnalyzerManagerError(scanner ScanType, err error) error {
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		exitCode := exitError.ExitCode()
		if exitCodeDescription, exitCodeExists := exitCodeErrorsMap[exitCode]; exitCodeExists {
			log.Warn(exitCodeDescription)
			return nil
		}
	}
	return scanner.FormattedError(err)
}

func RemoveDuplicateValues(stringSlice []string) []string {
	keys := make(map[string]bool)
	finalSlice := []string{}
	for _, entry := range stringSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			finalSlice = append(finalSlice, entry)
		}
	}
	return finalSlice
}

func GetResultIfExists(file, lineCol, text string, results []SourceCodeScanResult) int {
	for i, result := range results {
		if result.File == file && result.LineColumn == lineCol && result.Text == text {
			return i
		}
	}
	return -1
}

// If a result with the same file, line, column and text exists return it. otherwise create a new result and add it to results
// Used to combine results from similar places instead of reporting multiple duplicate rows
func GetOrCreateCodeScanResult(result *sarif.Result, workingDir string, results *[]SourceCodeScanResult) int {
	file := ExtractRelativePath(GetResultFileName(result), workingDir)
	lineCol := GetResultLocationInFile(result)
	text := *result.Message.Text
	// Already exists
	if index := GetResultIfExists(file, lineCol, text, *results); index >= 0 {
		return index
	}
	// New result
	newResult := SourceCodeScanResult{
		Severity: GetResultSeverity(result),
		SourceCodeLocation: SourceCodeLocation{
			File:       file,
			LineColumn: lineCol,
			Text:       text,
		},
		Type: *result.RuleID,
	}
	index := len(*results)
	*results = append(*results, newResult)

	return index
}

func GetResultFileName(result *sarif.Result) string {
	if len(result.Locations) > 0 {
		return getResultFileName(result.Locations[0])
	}
	return ""
}

func getResultFileName(location *sarif.Location) string {
	filePath := location.PhysicalLocation.ArtifactLocation.URI
	if filePath != nil {
		return *filePath
	}
	return ""
}

func GetResultLocationInFile(result *sarif.Result) string {
	if len(result.Locations) > 0 {
		return getResultLocationInFile(result.Locations[0])
	}
	return ""
}

func getResultLocationInFile(location *sarif.Location) string {
	startLine := location.PhysicalLocation.Region.StartLine
	startColumn := location.PhysicalLocation.Region.StartColumn
	if startLine != nil && startColumn != nil {
		return strconv.Itoa(*startLine) + ":" + strconv.Itoa(*startColumn)
	}
	return ""
}

func GetResultLocationSnippet(location *sarif.Location) string {
	if location != nil && location.PhysicalLocation != nil && location.PhysicalLocation.Region != nil && location.PhysicalLocation.Region.Snippet != nil {
		return *location.PhysicalLocation.Region.Snippet.Text
	}
	return ""
}

func GetResultCodeFlows(result *sarif.Result, workingDir string) (flows [][]SourceCodeLocation) {
	if len(result.CodeFlows) == 0 {
		return
	}
	for _, codeFlow := range result.CodeFlows {
		if codeFlow == nil || len(codeFlow.ThreadFlows) == 0 {
			continue
		}
		for _, threadFlow := range codeFlow.ThreadFlows {
			if threadFlow == nil || len(threadFlow.Locations) == 0 {
				continue
			}
			flow := []SourceCodeLocation{}
			for _, location := range threadFlow.Locations {
				if location == nil {
					continue
				}
				flow = append(flow, SourceCodeLocation{
					File:       ExtractRelativePath(getResultFileName(location.Location), workingDir),
					LineColumn: getResultLocationInFile(location.Location),
					Text:       GetResultLocationSnippet(location.Location),
				})
			}
			if len(flow) == 0 {
				continue
			}
			flows = append(flows, flow)
		}
	}
	return
}

func ExtractRelativePath(resultPath string, projectRoot string) string {
	filePrefix := "file://"
	relativePath := strings.ReplaceAll(strings.ReplaceAll(resultPath, projectRoot, ""), filePrefix, "")
	return relativePath
}

func GetResultSeverity(result *sarif.Result) string {
	if result.Level != nil {
		if severity, ok := levelToSeverity[*result.Level]; ok {
			return severity
		}
	}
	return SeverityDefaultValue
}

// Receives a list of relative path working dirs, returns a list of full paths working dirs
func GetFullPathsWorkingDirs(workingDirs []string) ([]string, error) {
	if len(workingDirs) == 0 {
		currentDir, err := coreutils.GetWorkingDirectory()
		if err != nil {
			return nil, err
		}
		return []string{currentDir}, nil
	}

	var fullPathsWorkingDirs []string
	for _, wd := range workingDirs {
		fullPathWd, err := filepath.Abs(wd)
		if err != nil {
			return nil, err
		}
		fullPathsWorkingDirs = append(fullPathsWorkingDirs, fullPathWd)
	}
	return fullPathsWorkingDirs, nil
}
