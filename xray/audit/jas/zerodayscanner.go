package jas

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	zeroDayScanCommand = "zd"
	zeroDayScannerType = "analyze-codebase"
)

type ZeroDayScanManager struct {
	zeroDayScannerResults []utils.SourceCodeScanResult
	scanner               *AdvancedSecurityScanner
}

func newZeroDayScanManager(scanner *AdvancedSecurityScanner) (manager *ZeroDayScanManager) {
	return &ZeroDayScanManager{
		zeroDayScannerResults: []utils.SourceCodeScanResult{},
		scanner:               scanner,
	}
}

func (zd *ZeroDayScanManager) Run(wd string) (err error) {
	scanner := zd.scanner
	if err = zd.runAnalyzerManager(wd); err != nil {
		return
	}
	var workingDirResults []utils.SourceCodeScanResult
	workingDirResults, err = getSourceCodeScanResults(scanner.resultsFileName, wd, utils.ZeroDay)
	zd.zeroDayScannerResults = append(zd.zeroDayScannerResults, workingDirResults...)
	return
}

func (zd *ZeroDayScanManager) runAnalyzerManager(wd string) error {
	return zd.scanner.analyzerManager.Exec(zd.scanner.resultsFileName, zeroDayScanCommand, wd, zd.scanner.serverDetails)
}

func getZeroDayScanResults(scanner *AdvancedSecurityScanner) (results []utils.SourceCodeScanResult, err error) {
	zeroDayScanManager := newZeroDayScanManager(scanner)
	log.Info("Running SAST scanning...")
	if err = zeroDayScanManager.scanner.Run(zeroDayScanManager); err != nil {
		err = utils.ParseAnalyzerManagerError(utils.ZeroDay, err)
		return
	}
	if len(zeroDayScanManager.zeroDayScannerResults) > 0 {
		log.Info(len(zeroDayScanManager.zeroDayScannerResults), "SAST vulnerabilities")
	}
	results = zeroDayScanManager.zeroDayScannerResults
	return
}
