package jas

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	iacScannerType = "iac-scan-modules"
	iacScanCommand = "iac"
)

type IacScanManager struct {
	iacScannerResults []utils.SourceCodeScanResult
	scanner           *AdvancedSecurityScanner
}

// The getIacScanResults function runs the iac scan flow, which includes the following steps:
// Creating an IacScanManager object.
// Running the analyzer manager executable.
// Parsing the analyzer manager results.
// Return values:
// []utils.SourceCodeScanResult: a list of the iac violations that were found.
// bool: true if the user is entitled to iac scan, false otherwise.
// error: An error object (if any).
func getIacScanResults(scanner *AdvancedSecurityScanner) (results []utils.SourceCodeScanResult, err error) {
	iacScanManager := newIacScanManager(scanner)
	log.Info("Running IaC scanning...")
	if err = iacScanManager.scanner.Run(iacScanManager); err != nil {
		err = utils.ParseAnalyzerManagerError(utils.IaC, err)
		return
	}
	if len(iacScanManager.iacScannerResults) > 0 {
		log.Info("Found", len(iacScanManager.iacScannerResults), "IaC vulnerabilities")
	}
	results = iacScanManager.iacScannerResults
	return
}

func newIacScanManager(scanner *AdvancedSecurityScanner) (manager *IacScanManager) {
	return &IacScanManager{
		iacScannerResults: []utils.SourceCodeScanResult{},
		scanner:           scanner,
	}
}

func (iac *IacScanManager) Run(wd string) (err error) {
	scanner := iac.scanner
	if err = iac.createConfigFile(wd); err != nil {
		return
	}
	if err = iac.runAnalyzerManager(); err != nil {
		return
	}
	var workingDirResults []utils.SourceCodeScanResult
	workingDirResults, err = getSourceCodeScanResults(scanner.resultsFileName, wd, utils.IaC)
	iac.iacScannerResults = append(iac.iacScannerResults, workingDirResults...)
	return
}

type iacScanConfig struct {
	Scans []iacScanConfiguration `yaml:"scans"`
}

type iacScanConfiguration struct {
	Roots       []string `yaml:"roots"`
	Output      string   `yaml:"output"`
	Type        string   `yaml:"type"`
	SkippedDirs []string `yaml:"skipped-folders"`
}

func (iac *IacScanManager) createConfigFile(currentWd string) error {
	configFileContent := iacScanConfig{
		Scans: []iacScanConfiguration{
			{
				Roots:       []string{currentWd},
				Output:      iac.scanner.resultsFileName,
				Type:        iacScannerType,
				SkippedDirs: skippedDirs,
			},
		},
	}
	return createScannersConfigFile(iac.scanner.configFileName, configFileContent)
}

func (iac *IacScanManager) runAnalyzerManager() error {
	return iac.scanner.analyzerManager.Exec(iac.scanner.configFileName, iacScanCommand, iac.scanner.analyzerManager.GetAnalyzerManagerDir(), iac.scanner.serverDetails)
}
