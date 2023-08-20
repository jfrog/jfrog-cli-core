package jas

import (
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	secretsScanCommand = "sec"
	secretsScannerType = "secrets-scan"
)

type SecretScanManager struct {
	secretsScannerResults []utils.IacOrSecretResult
	scanner               *AdvancedSecurityScanner
}

// The getSecretsScanResults function runs the secrets scan flow, which includes the following steps:
// Creating an SecretScanManager object.
// Running the analyzer manager executable.
// Parsing the analyzer manager results.
// Return values:
// []utils.IacOrSecretResult: a list of the secrets that were found.
// error: An error object (if any).
func getSecretsScanResults(scanner *AdvancedSecurityScanner) (results []utils.IacOrSecretResult, err error) {
	secretScanManager := newSecretsScanManager(scanner)
	log.Info("Running secrets scanning...")
	if err = secretScanManager.scanner.Run(secretScanManager); err != nil {
		err = utils.ParseAnalyzerManagerError(utils.Secrets, err)
		return
	}
	results = secretScanManager.secretsScannerResults
	if len(results) > 0 {
		log.Info(len(results), "secrets were found")
	}
	return
}

func newSecretsScanManager(scanner *AdvancedSecurityScanner) (manager *SecretScanManager) {
	return &SecretScanManager{
		secretsScannerResults: []utils.IacOrSecretResult{},
		scanner:               scanner,
	}
}

func (s *SecretScanManager) Run(wd string) (err error) {
	scanner := s.scanner
	if err = s.createConfigFile(wd); err != nil {
		return
	}
	if err = s.runAnalyzerManager(); err != nil {
		return
	}
	var workingDirResults []utils.IacOrSecretResult
	workingDirResults, err = getIacOrSecretsScanResults(scanner.resultsFileName, wd, true)
	s.secretsScannerResults = append(s.secretsScannerResults, workingDirResults...)
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
				Output:      s.scanner.resultsFileName,
				Type:        secretsScannerType,
				SkippedDirs: skippedDirs,
			},
		},
	}
	return createScannersConfigFile(s.scanner.configFileName, configFileContent)
}

func (s *SecretScanManager) runAnalyzerManager() error {
	return s.scanner.analyzerManager.Exec(s.scanner.configFileName, secretsScanCommand, s.scanner.serverDetails)
}

func hideSecret(secret string) string {
	if len(secret) <= 3 {
		return "***"
	}
	return secret[:3] + strings.Repeat("*", len(secret)-3)
}
