package sast

import (
	jfrogappsconfig "github.com/jfrog/jfrog-apps-config/go"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	sastScanCommand = "zd"
)

type SastScanManager struct {
	sastScannerResults []utils.SourceCodeScanResult
	scanner            *jas.JasScanner
}

func RunSastScan(scanner *jas.JasScanner) (results []utils.SourceCodeScanResult, err error) {
	sastScanManager := newSastScanManager(scanner)
	log.Info("Running SAST scanning...")
	if err = sastScanManager.scanner.Run(sastScanManager); err != nil {
		err = utils.ParseAnalyzerManagerError(utils.Sast, err)
		return
	}
	if len(sastScanManager.sastScannerResults) > 0 {
		log.Info("Found", len(sastScanManager.sastScannerResults), "SAST vulnerabilities")
	}
	results = sastScanManager.sastScannerResults
	return
}

func newSastScanManager(scanner *jas.JasScanner) (manager *SastScanManager) {
	return &SastScanManager{
		sastScannerResults: []utils.SourceCodeScanResult{},
		scanner:            scanner,
	}
}

func (ssm *SastScanManager) Run(module jfrogappsconfig.Module) (err error) {
	if jas.ShouldSkipScanner(module, utils.Sast) {
		return
	}
	if err = ssm.createConfigFile(module); err != nil {
		return
	}
	if err = ssm.runAnalyzerManager(module.SourceRoot); err != nil {
		return
	}
	var workingDirResults []utils.SourceCodeScanResult
	if workingDirResults, err = jas.GetSourceCodeScanResults(ssm.scanner.ResultsFileName, module.SourceRoot, utils.Sast); err != nil {
		return
	}
	ssm.sastScannerResults = append(ssm.sastScannerResults, workingDirResults...)
	return
}

type sastScanConfig struct {
	Scans []scanConfiguration `yaml:"scans,omitempty"`
}

type scanConfiguration struct {
	Roots           []string `yaml:"roots,omitempty"`
	Languages       []string `yaml:"language,omitempty"`
	ExcludePatterns []string `yaml:"exclude_patterns,omitempty"`
	ExcludedRules   []string `yaml:"excluded-rules,omitempty"`
}

func (ssm *SastScanManager) createConfigFile(module jfrogappsconfig.Module) error {
	sastScanner := module.Scanners.Sast
	if sastScanner == nil {
		sastScanner = &jfrogappsconfig.SastScanner{}
	}
	configFileContent := sastScanConfig{
		Scans: []scanConfiguration{
			{
				Roots:           jas.GetSourceRoots(module, &sastScanner.Scanner),
				Languages:       []string{sastScanner.Language},
				ExcludedRules:   sastScanner.ExcludedRules,
				ExcludePatterns: jas.GetExcludePatterns(module, &sastScanner.Scanner),
			},
		},
	}
	return jas.CreateScannersConfigFile(ssm.scanner.ConfigFileName, configFileContent)
}

func (ssm *SastScanManager) runAnalyzerManager(wd string) error {
	return ssm.scanner.AnalyzerManager.ExecWithOutputFile(ssm.scanner.ResultsFileName, sastScanCommand, wd, ssm.scanner.ResultsFileName, ssm.scanner.ServerDetails)
}
