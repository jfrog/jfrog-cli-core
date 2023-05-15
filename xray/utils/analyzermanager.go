package utils

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"os"
	"os/exec"
	"path/filepath"
)

var (
	analyzerManagerFilePath  = filepath.Join("analyzerManager", "analyzerManager")
	analyzerManagerLogFolder = ""
)

const (
	analyzerManagerDirName   = "analyzerManagerLogs"
	jfUserEnvVariable        = "JF_USER"
	jfPasswordEnvVariable    = "JF_PASS"
	jfTokenEnvVariable       = "JF_TOKEN"
	jfPlatformUrlEnvVariable = "JF_PLATFORM_URL"
	logDirEnvVariable        = "AM_LOG_DIRECTORY"
	applicabilityScanCommand = "ca"
)

const (
	ApplicableStringValue                = "Applicable"
	NotApplicableStringValue             = "Not Applicable"
	ApplicabilityUndeterminedStringValue = "Undetermined"
)

type ExtendedScanResults struct {
	XrayResults                 []services.ScanResponse
	ApplicabilityScannerResults map[string]string
	EntitledForJas              bool
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
	Exec(string) error
}

type AnalyzerManager struct {
	analyzerManagerFullPath string
}

func (am *AnalyzerManager) ExistLocally() (bool, error) {
	analyzerManagerPath, err := getAnalyzerManagerAbsolutePath()
	if err != nil {
		return false, err
	}
	am.analyzerManagerFullPath = analyzerManagerFilePath
	return fileutils.IsFileExists(analyzerManagerPath, false)
}

func (am *AnalyzerManager) Exec(configFile string) error {
	return exec.Command(am.analyzerManagerFullPath, applicabilityScanCommand, configFile).Run()
}

func CreateAnalyzerManagerLogDir() error {
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
	analyzerManager := analyzerManagerFilePath
	if coreutils.IsWindows() {
		analyzerManagerFilePath += ".exe"
	}
	return filepath.Join(jfrogDir, analyzerManager), nil
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
	if err := os.Setenv(logDirEnvVariable, analyzerManagerLogFolder); errorutils.CheckError(err) != nil {
		return err
	}
	return nil
}
