package jas

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	analyzerManagerFilePath  = "analayzerManager/analyzerManager"
	analyzerManagerLogFolder = ""
)

const (
	analyzerManagerDirName   = "analyzerManagerLogs"
	jfUserEnvVariable        = "JF_USER"
	jfPasswordEnvVariable    = "JF_PASS"
	jfTokenEnvVariable       = "JF_TOKEN"
	jfPlatformUrlEnvVariable = "JF_PLATFORM_URL"
	logDirEnvVariable        = "AM_LOG_DIRECTORY"
)

var analyzerManagerExecuter AnalyzerManager = &analyzerManager{}

func createAnalyzerManagerLogDir() error {
	logDir, err := coreutils.CreateDirInJfrogHome(filepath.Join(coreutils.JfrogLogsDirName, analyzerManagerDirName))
	if err != nil {
		return err
	}
	analyzerManagerLogFolder = logDir
	return nil
}

func getAnalyzerManagerAbsolutePath() (string, error) {
	jfrogDir, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(jfrogDir, analyzerManagerFilePath), nil
}

func removeDuplicateValues(stringSlice []string) []string {
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

func setAnalyzerManagerEnvVariables(serverDetails *config.ServerDetails) error {
	if serverDetails == nil {
		return errors.New("cant get xray server details")
	}
	err := os.Setenv(jfUserEnvVariable, serverDetails.User)
	if err != nil {
		return err
	}
	err = os.Setenv(jfPasswordEnvVariable, serverDetails.Password)
	if err != nil {
		return err
	}
	err = os.Setenv(jfPlatformUrlEnvVariable, serverDetails.Url)
	if err != nil {
		return err
	}
	err = os.Setenv(jfTokenEnvVariable, serverDetails.AccessToken)
	if err != nil {
		return err
	}
	err = os.Setenv(logDirEnvVariable, analyzerManagerLogFolder)
	return err
}

func deleteJasScanProcessFiles(configFile string, resultsFile string) error {
	if exist, _ := fileutils.IsFileExists(configFile, false); exist {
		err := os.Remove(configFile)
		if err != nil {
			return err
		}
	}
	if exist, _ := fileutils.IsFileExists(resultsFile, false); exist {
		err := os.Remove(resultsFile)
		if err != nil {
			return err
		}
	}
	return nil
}

func isNotEntitledError(err error) bool {
	if exitError, ok := err.(*exec.ExitError); ok {
		exitCode := exitError.ExitCode()
		// User not entitled error
		if exitCode == 31 {
			log.Info("got not entitled error from analyzer manager")
			return true
		}
	}
	return false
}

func isUnsupportedCommandError(err error) bool {
	if exitError, ok := err.(*exec.ExitError); ok {
		exitCode := exitError.ExitCode()
		// Analyzer manager doesnt support the requested scan command
		if exitCode == 13 {
			log.Info("got unsupported scan command error from analyzer manager")
			return true
		}
	}
	return false
}

func getResultFileName(secret *sarif.Result) string {
	filePath := secret.Locations[0].PhysicalLocation.ArtifactLocation.URI
	if filePath == nil {
		return ""
	}
	return *filePath
}

func getResultLocationInFile(secret *sarif.Result) string {
	startLine := strconv.Itoa(*secret.Locations[0].PhysicalLocation.Region.StartLine)
	startColumn := strconv.Itoa(*secret.Locations[0].PhysicalLocation.Region.StartColumn)
	if startLine != "" && startColumn != "" {
		return startLine + ":" + startColumn
	} else if startLine == "" && startColumn != "" {
		return "startLine:" + startColumn
	} else if startLine != "" && startColumn == "" {
		return startLine + ":startColumn"
	}
	return ""
}

func extractRelativePath(secretPath string, projectRoot string) string {
	filePrefix := "file://"
	relativePath := strings.ReplaceAll(strings.ReplaceAll(secretPath, projectRoot, ""), filePrefix, "")
	return relativePath
}

func getResultSeverity(result *sarif.Result) string {
	if result.Level != nil {
		return *result.Level
	}
	return "Medium" // Default value for severity

}

type ExtendedScanResults struct {
	XrayResults                  []services.ScanResponse
	ApplicabilityScannerResults  map[string]string
	SecretsScanResults           []Secret
	IacScanResults               []Iac
	EntitledForJas               bool
	EligibleForApplicabilityScan bool
	EligibleForSecretScan        bool
	EligibleForIacScan           bool
}

func (e *ExtendedScanResults) GetXrayScanResults() []services.ScanResponse {
	return e.XrayResults
}

func GetExtendedScanResults(results []services.ScanResponse, dependencyTrees []*services.GraphNode,
	serverDetails *config.ServerDetails) (*ExtendedScanResults, error) {
	if !analyzerManagerExecuter.DoesAnalyzerManagerExecutableExist() {
		log.Info("analyzer manager doesnt exist, user is not entitled for jas")
		return &ExtendedScanResults{XrayResults: results}, nil
	}
	err := createAnalyzerManagerLogDir()
	if err != nil {
		return nil, err
	}
	applicabilityScanResults, eligibleForApplicabilityScan, err := getApplicabilityScanResults(results,
		dependencyTrees, serverDetails, analyzerManagerExecuter)
	if err != nil {
		return nil, err
	}
	secretsScanResults, eligibleForSecretsScan, err := getSecretsScanResults(serverDetails, analyzerManagerExecuter)
	if err != nil {
		return nil, err
	}
	iacScanResults, eligibleForIacScan, err := getIacScanResults(serverDetails, analyzerManagerExecuter)
	if err != nil {
		return nil, err
	}
	return &ExtendedScanResults{
		XrayResults:                  results,
		ApplicabilityScannerResults:  applicabilityScanResults,
		SecretsScanResults:           secretsScanResults,
		IacScanResults:               iacScanResults,
		EntitledForJas:               true,
		EligibleForApplicabilityScan: eligibleForApplicabilityScan,
		EligibleForSecretScan:        eligibleForSecretsScan,
		EligibleForIacScan:           eligibleForIacScan,
	}, nil
}
