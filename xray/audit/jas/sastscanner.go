package jas

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	sastScanCommand = "zd"
	sastScannerType = "analyze-codebase"
)

type SastScanManager struct {
	sastScannerResults []utils.SourceCodeScanResult
	scanner            *AdvancedSecurityScanner
}

func newSastScanManager(scanner *AdvancedSecurityScanner) (manager *SastScanManager) {
	return &SastScanManager{
		sastScannerResults: []utils.SourceCodeScanResult{},
		scanner:            scanner,
	}
}

func (zd *SastScanManager) Run(wd string) (err error) {
	scanner := zd.scanner
	if err = zd.runAnalyzerManager(wd); err != nil {
		return
	}
	var workingDirResults []utils.SourceCodeScanResult
	if workingDirResults, err = getSourceCodeScanResults(scanner.resultsFileName, wd, utils.Sast); err != nil {
		return
	}
	zd.sastScannerResults = append(zd.sastScannerResults, workingDirResults...)
	return
}

func (zd *SastScanManager) runAnalyzerManager(wd string) error {
	return zd.scanner.analyzerManager.Exec(zd.scanner.resultsFileName, sastScanCommand, wd, zd.scanner.serverDetails)
}

func getSastScanResults(scanner *AdvancedSecurityScanner) (results []utils.SourceCodeScanResult, err error) {
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
