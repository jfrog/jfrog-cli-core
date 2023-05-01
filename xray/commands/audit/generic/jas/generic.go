package jas

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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


func generateRandomFileName() (string, error) {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	result := make([]byte, 10)
	for i := 0; i < 10; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		result[i] = letters[num.Int64()]
	}

	return string(result), nil
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
	if _, err := os.Stat(configFile); err == nil {
		err = os.Remove(configFile)
		if err != nil {
			return err
		}
	}
	if _, err := os.Stat(resultsFile); err == nil {
		err = os.Remove(resultsFile)
		if err != nil {
			return err
		}
	}
	return nil
}

type AnalyzerManager interface {
	DoesAnalyzerManagerExecutableExist() bool
	RunAnalyzerManager(string, string) error
}

type analyzerManager struct {
}

func (am *analyzerManager) DoesAnalyzerManagerExecutableExist() bool {
	if _, err := os.Stat(getAnalyzerManagerAbsolutePath()); err != nil {
		return false
	}
	return true
}

func (am *analyzerManager) RunAnalyzerManager(configFile string, scanCommand string) error {
	var err error
	if runtime.GOOS == "windows" {
		_, err = exec.Command(getAnalyzerManagerAbsolutePath()+".exe", scanCommand, configFile).Output()
	} else {
		_, err = exec.Command(getAnalyzerManagerAbsolutePath(), scanCommand, configFile).Output()
	}
	if err != nil {
		return err
	}
	return nil
}

type ExtendedScanResults struct {
	XrayResults                  []services.ScanResponse
	ApplicabilityScannerResults  map[string]string
	SecretsScanResults           []Secret
	EntitledForJas               bool
	EligibleForApplicabilityScan bool
	EligibleForSecretScan        bool
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
	applicabilityScanResults, eligibleForApplicabilityScan, err := getApplicabilityScanResults(results,
		dependencyTrees, serverDetails, analyzerManagerExecuter)
	if err != nil {
		return nil, err
	}
	secretsScanResults, eligibleForSecretsScan, err := getSecretsScanResults(serverDetails)
	if err != nil {
		return nil, err
	}
	return &ExtendedScanResults{
		XrayResults:                  results,
		SecretsScanResults:           secretsScanResults,
		ApplicabilityScannerResults:  applicabilityScanResults,
		EntitledForJas:               true,
		EligibleForApplicabilityScan: eligibleForApplicabilityScan,
		EligibleForSecretScan:        eligibleForSecretsScan,
	}, nil
}
