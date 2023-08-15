package jas

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	secretsScanCommand    = "sec"
	secretsScannerType    = "secrets-scan"
	secScanFailureMessage = "failed to run secrets scan. Cause: %s"
)

type SecretScanManager struct {
	secretsScannerResults []utils.IacOrSecretResult
	configFileName        string
	resultsFileName       string
	analyzerManager       utils.AnalyzerManagerInterface
	serverDetails         *config.ServerDetails
	workingDirs           []string
}

// The getSecretsScanResults function runs the secrets scan flow, which includes the following steps:
// Creating an SecretScanManager object.
// Running the analyzer manager executable.
// Parsing the analyzer manager results.
// Return values:
// []utils.IacOrSecretResult: a list of the secrets that were found.
// bool: true if the user is entitled to secrets scan, false otherwise.
// error: An error object (if any).
func getSecretsScanResults(serverDetails *config.ServerDetails, workingDirs []string, analyzerManager utils.AnalyzerManagerInterface) ([]utils.IacOrSecretResult,
	bool, error) {
	secretScanManager, cleanupFunc, err := newSecretsScanManager(serverDetails, workingDirs, analyzerManager)
	if err != nil {
		return nil, false, fmt.Errorf(secScanFailureMessage, err.Error())
	}
	defer func() {
		if cleanupFunc != nil {
			err = errors.Join(err, cleanupFunc())
		}
	}()
	log.Info("Running secrets scanning...")
	if err = secretScanManager.run(); err != nil {
		return nil, false, utils.ParseAnalyzerManagerError(utils.Secrets, err)
	}
	if len(secretScanManager.secretsScannerResults) > 0 {
		log.Info(len(secretScanManager.secretsScannerResults), "secrets were found")
	}
	return secretScanManager.secretsScannerResults, true, nil
}

func newSecretsScanManager(serverDetails *config.ServerDetails, workingDirs []string, analyzerManager utils.AnalyzerManagerInterface) (manager *SecretScanManager,
	cleanup func() error, err error) {
	tempDir, err := fileutils.CreateTempDir()
	if err != nil {
		return
	}
	cleanup = func() error {
		return fileutils.RemoveTempDir(tempDir)
	}
	fullPathWorkingDirs, err := utils.GetFullPathsWorkingDirs(workingDirs)
	if err != nil {
		return nil, cleanup, err
	}
	return &SecretScanManager{
		secretsScannerResults: []utils.IacOrSecretResult{},
		configFileName:        filepath.Join(tempDir, "config.yaml"),
		resultsFileName:       filepath.Join(tempDir, "results.sarif"),
		analyzerManager:       analyzerManager,
		serverDetails:         serverDetails,
		workingDirs:           fullPathWorkingDirs,
	}, cleanup, nil
}

func (s *SecretScanManager) run() (err error) {
	for _, workingDir := range s.workingDirs {
		var workingDirResults []utils.IacOrSecretResult
		if workingDirResults, err = s.runSecretsScan(workingDir); err != nil {
			return
		}
		s.secretsScannerResults = append(s.secretsScannerResults, workingDirResults...)
	}
	return
}

func (s *SecretScanManager) runSecretsScan(workingDir string) (results []utils.IacOrSecretResult, err error) {
	defer func() {
		err = errors.Join(err, deleteJasProcessFiles(s.configFileName, s.resultsFileName))
	}()
	if err = s.createConfigFile(workingDir); err != nil {
		return
	}
	if err = s.runAnalyzerManager(); err != nil {
		return
	}
	results, err = getIacOrSecretsScanResults(s.resultsFileName, workingDir, true)
	return
}

type secretsScanConfig struct {
	Scans []secretsScanConfiguration `yaml:"scans"`
}

type secretsScanConfiguration struct {
	Roots       []string `yaml:"roots"`
	Output      string   `yaml:"output"`
	Type        string   `yaml:"type"`
	SkippedDirs []string `yaml:"skipped-folders"`
}

func (s *SecretScanManager) createConfigFile(currentWd string) error {
	configFileContent := secretsScanConfig{
		Scans: []secretsScanConfiguration{
			{
				Roots:       []string{currentWd},
				Output:      s.resultsFileName,
				Type:        secretsScannerType,
				SkippedDirs: skippedDirs,
			},
		},
	}
	return createScannersConfigFile(s.configFileName, configFileContent)
}

func (s *SecretScanManager) runAnalyzerManager() error {
	return s.analyzerManager.Exec(s.configFileName, secretsScanCommand, s.serverDetails)
}

func hideSecret(secret string) string {
	if len(secret) <= 3 {
		return "***"
	}
	hiddenSecret := ""
	i := 0
	for i < 3 { // Show first 3 digits
		hiddenSecret += string(secret[i])
		i++
	}
	for i < 15 {
		hiddenSecret += "*"
		i++
	}
	return hiddenSecret
}
