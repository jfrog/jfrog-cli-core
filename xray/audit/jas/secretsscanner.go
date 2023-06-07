package jas

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
)

const (
	secretsScanCommand    = "sec"
	secretsScannersNames  = "tokens, entropy"
	secretsScannerType    = "secrets-scan"
	secScanFailureMessage = "failed to run secrets scan. Cause: %s"
)

type SecretScanManager struct {
	secretsScannerResults []utils.IacOrSecretResult
	configFileName        string
	resultsFileName       string
	analyzerManager       utils.AnalyzerManagerInterface
	serverDetails         *config.ServerDetails
	projectRootPath       string
}

// The getSecretsScanResults function runs the secrets scan flow, which includes the following steps:
// Creating an SecretScanManager object.
// Running the analyzer manager executable.
// Parsing the analyzer manager results.
// Return values:
// []utils.IacOrSecretResult: a list of the secrets that were found.
// bool: true if the user is entitled to secrets scan, false otherwise.
// error: An error object (if any).
func getSecretsScanResults(serverDetails *config.ServerDetails, analyzerManager utils.AnalyzerManagerInterface) ([]utils.IacOrSecretResult,
	bool, error) {
	secretScanManager, cleanupFunc, err := newSecretsScanManager(serverDetails, analyzerManager)
	if err != nil {
		return nil, false, fmt.Errorf(secScanFailureMessage, err.Error())
	}
	defer func() {
		if cleanupFunc != nil {
			cleanupError := cleanupFunc()
			err = errors.Join(err, cleanupError)
		}
	}()
	err = secretScanManager.run()
	if err != nil {
		if utils.IsNotEntitledError(err) || utils.IsUnsupportedCommandError(err) {
			return nil, false, nil
		}
		return nil, true, fmt.Errorf(secScanFailureMessage, err.Error())
	}
	return secretScanManager.secretsScannerResults, true, nil
}

func newSecretsScanManager(serverDetails *config.ServerDetails, analyzerManager utils.AnalyzerManagerInterface) (manager *SecretScanManager,
	cleanup func() error, err error) {
	tempDir, err := fileutils.CreateTempDir()
	if err != nil {
		return
	}
	cleanup = func() error {
		return fileutils.RemoveTempDir(tempDir)
	}
	return &SecretScanManager{
		secretsScannerResults: []utils.IacOrSecretResult{},
		configFileName:        filepath.Join(tempDir, "config.yaml"),
		resultsFileName:       filepath.Join(tempDir, "results.sarif"),
		analyzerManager:       analyzerManager,
		serverDetails:         serverDetails,
	}, cleanup, nil
}

func (s *SecretScanManager) run() error {
	var err error
	defer func() {
		if deleteJasProcessFiles(s.configFileName, s.resultsFileName) != nil {
			e := deleteJasProcessFiles(s.configFileName, s.resultsFileName)
			if err == nil {
				err = e
			}
		}
	}()
	if err = s.createConfigFile(); err != nil {
		return err
	}
	if err = s.runAnalyzerManager(); err != nil {
		return err
	}
	return s.parseResults()
}

type secretsScanConfig struct {
	Scans []secretsScanConfiguration `yaml:"scans"`
}

type secretsScanConfiguration struct {
	Roots       []string `yaml:"roots"`
	Output      string   `yaml:"output"`
	Type        string   `yaml:"type"`
	Scanners    string   `yaml:"scanners"`
	SkippedDirs []string `yaml:"skipped-folders"`
}

func (s *SecretScanManager) createConfigFile() error {
	currentDir, err := coreutils.GetWorkingDirectory()
	s.projectRootPath = currentDir
	if err != nil {
		return err
	}
	configFileContent := secretsScanConfig{
		Scans: []secretsScanConfiguration{
			{
				Roots:       []string{currentDir},
				Output:      s.resultsFileName,
				Type:        secretsScannerType,
				Scanners:    secretsScannersNames,
				SkippedDirs: skippedDirs,
			},
		},
	}
	yamlData, err := yaml.Marshal(&configFileContent)
	if errorutils.CheckError(err) != nil {
		return err
	}
	err = os.WriteFile(s.configFileName, yamlData, 0644)
	return errorutils.CheckError(err)
}

func (s *SecretScanManager) runAnalyzerManager() error {
	if err := utils.SetAnalyzerManagerEnvVariables(s.serverDetails); err != nil {
		return err
	}
	return s.analyzerManager.Exec(s.configFileName, secretsScanCommand)
}

func (s *SecretScanManager) parseResults() error {
	report, err := sarif.Open(s.resultsFileName)
	if errorutils.CheckError(err) != nil {
		return err
	}
	var secretsResults []*sarif.Result
	if len(report.Runs) > 0 {
		secretsResults = report.Runs[0].Results
	}

	finalSecretsList := []utils.IacOrSecretResult{}

	for _, secret := range secretsResults {
		newSecret := utils.IacOrSecretResult{
			Severity:   utils.GetResultSeverity(secret),
			File:       utils.ExtractRelativePath(utils.GetResultFileName(secret), s.projectRootPath),
			LineColumn: utils.GetResultLocationInFile(secret),
			Text:       hideSecret(*secret.Locations[0].PhysicalLocation.Region.Snippet.Text),
			Type:       *secret.RuleID,
		}
		finalSecretsList = append(finalSecretsList, newSecret)
	}
	s.secretsScannerResults = finalSecretsList
	return nil
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
