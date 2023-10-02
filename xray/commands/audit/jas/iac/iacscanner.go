package iac

import (
	"path/filepath"

	jfrogappsconfig "github.com/jfrog/jfrog-apps-config/go"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas"

	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/owenrumney/go-sarif/v2/sarif"
)

const (
	iacScannerType = "iac-scan-modules"
	iacScanCommand = "iac"
)

type IacScanManager struct {
	iacScannerResults []*sarif.Run
	scanner           *jas.JasScanner
}

// The getIacScanResults function runs the iac scan flow, which includes the following steps:
// Creating an IacScanManager object.
// Running the analyzer manager executable.
// Parsing the analyzer manager results.
// Return values:
// []utils.SourceCodeScanResult: a list of the iac violations that were found.
// bool: true if the user is entitled to iac scan, false otherwise.
// error: An error object (if any).
func RunIacScan(scanner *jas.JasScanner) (results []*sarif.Run, err error) {
	iacScanManager := newIacScanManager(scanner)
	log.Info("Running IaC scanning...")
	if err = iacScanManager.scanner.Run(iacScanManager); err != nil {
		err = utils.ParseAnalyzerManagerError(utils.IaC, err)
		return
	}
	if len(iacScanManager.iacScannerResults) > 0 {
		log.Info("Found", utils.GetResultsLocationCount(iacScanManager.iacScannerResults...), "IaC vulnerabilities")
	}
	results = iacScanManager.iacScannerResults
	return
}

func newIacScanManager(scanner *jas.JasScanner) (manager *IacScanManager) {
	return &IacScanManager{
		iacScannerResults: []*sarif.Run{},
		scanner:           scanner,
	}
}

func (iac *IacScanManager) Run(module jfrogappsconfig.Module) (err error) {
	if jas.ShouldSkipScanner(module, utils.IaC) {
		return
	}
	if err = iac.createConfigFile(module); err != nil {
		return
	}
	if err = iac.runAnalyzerManager(); err != nil {
		return
	}
	workingDirResults, err := jas.ReadJasScanRunsFromFile(iac.scanner.ResultsFileName, module.SourceRoot)
	if err != nil {
		return
	}
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

func (iac *IacScanManager) createConfigFile(module jfrogappsconfig.Module) error {
	roots, err := jas.GetSourceRoots(module, module.Scanners.Iac)
	if err != nil {
		return err
	}
	configFileContent := iacScanConfig{
		Scans: []iacScanConfiguration{
			{
				Roots:       roots,
				Output:      iac.scanner.ResultsFileName,
				Type:        iacScannerType,
				SkippedDirs: jas.GetExcludePatterns(module, module.Scanners.Iac),
			},
		},
	}
	return jas.CreateScannersConfigFile(iac.scanner.ConfigFileName, configFileContent, utils.IaC)
}

func (iac *IacScanManager) runAnalyzerManager() error {
	return iac.scanner.AnalyzerManager.Exec(iac.scanner.ConfigFileName, iacScanCommand, filepath.Dir(iac.scanner.AnalyzerManager.AnalyzerManagerFullPath), iac.scanner.ServerDetails)
}
