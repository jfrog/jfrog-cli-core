package sast

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	sastScanCommand = "zd"
	sastScannerType = "analyze-codebase"
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

func (ssm *SastScanManager) Run(wd string) (err error) {
	scanner := ssm.scanner
	if err = ssm.runAnalyzerManager(wd); err != nil {
		return
	}
	var workingDirResults []utils.SourceCodeScanResult
	if workingDirResults, err = jas.GetSourceCodeScanResults(scanner.ResultsFileName, wd, utils.Sast); err != nil {
		return
	}
	ssm.sastScannerResults = append(ssm.sastScannerResults, workingDirResults...)
	return
}

func (ssm *SastScanManager) runAnalyzerManager(wd string) error {
	return ssm.scanner.AnalyzerManager.Exec(ssm.scanner.ResultsFileName, sastScanCommand, wd, ssm.scanner.ServerDetails)
}
