package utils

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	levelToSeverity = map[string]string{"error": "High", "warning": "Medium", "info": "Low"}
)

const (
	EntitlementsMinVersion        = "3.66.5"
	ApplicabilityFeatureId        = "contextual_analysis"
	AnalyzerManagerZipName        = "analyzerManager.zip"
	analyzerManagerVersion        = "1.2.4.1921744"
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

type IacOrSecretResult struct {
	Severity   string
	File       string
	LineColumn string
	Type       string
	Text       string
}

type ExtendedScanResults struct {
	XrayResults                  []services.ScanResponse
	ScannedTechnologies          []coreutils.Technology
	ApplicabilityScanResults     map[string]string
	SecretsScanResults           []IacOrSecretResult
	IacScanResults               []IacOrSecretResult
	EntitledForJas               bool
	EligibleForApplicabilityScan bool
	EligibleForSecretScan        bool
	EligibleForIacScan           bool
}

func (e *ExtendedScanResults) getXrayScanResults() []services.ScanResponse {
	return e.XrayResults
}

// AnalyzerManagerInterface represents the analyzer manager executable file that exists locally as a Jfrog dependency.
// It triggers JAS capabilities by verifying user's entitlements and running the JAS scanners.
// Analyzer manager input:
//   - scan command: ca (contextual analysis) / sec (secrets) / iac
//   - path to configuration file
//
// Analyzer manager output:
//   - sarif file containing the scan results
type AnalyzerManagerInterface interface {
	ExistLocally() (bool, error)
	Exec(string, string, *config.ServerDetails) error
}

type AnalyzerManager struct {
	analyzerManagerFullPath string
}

func (am *AnalyzerManager) ExistLocally() (bool, error) {
	analyzerManagerPath, err := getAnalyzerManagerExecutable()
	if err != nil {
		return false, err
	}
	am.analyzerManagerFullPath = analyzerManagerPath
	return fileutils.IsFileExists(analyzerManagerPath, false)
}

func (am *AnalyzerManager) Exec(configFile, scanCommand string, serverDetails *config.ServerDetails) (err error) {
	if err = SetAnalyzerManagerEnvVariables(serverDetails); err != nil {
		return err
	}
	cmd := exec.Command(am.analyzerManagerFullPath, scanCommand, configFile)
	defer func() {
		if !cmd.ProcessState.Exited() {
			if killProcessError := cmd.Process.Kill(); errorutils.CheckError(killProcessError) != nil {
				err = errors.Join(err, killProcessError)
			}
		}
	}()
	cmd.Dir = filepath.Dir(am.analyzerManagerFullPath)
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

func getAnalyzerManagerExecutable() (string, error) {
	analyzerManagerDir, err := GetAnalyzerManagerDirAbsolutePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(analyzerManagerDir, GetAnalyzerManagerExecutableName()), nil
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

func GetResultFileName(result *sarif.Result) string {
	if len(result.Locations) > 0 {
		filePath := result.Locations[0].PhysicalLocation.ArtifactLocation.URI
		if filePath != nil {
			return *filePath
		}
	}
	return ""
}

func GetResultLocationInFile(result *sarif.Result) string {
	if len(result.Locations) > 0 {
		startLine := result.Locations[0].PhysicalLocation.Region.StartLine
		startColumn := result.Locations[0].PhysicalLocation.Region.StartColumn
		if startLine != nil && startColumn != nil {
			return strconv.Itoa(*startLine) + ":" + strconv.Itoa(*startColumn)
		}
	}
	return ""
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
