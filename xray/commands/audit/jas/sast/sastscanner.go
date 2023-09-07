package sast

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/owenrumney/go-sarif/v2/sarif"
)

const (
	sastScanCommand = "zd"
)

type SastScanManager struct {
	sastScannerResults []*sarif.Run
	scanner            *jas.JasScanner
}

func RunSastScan(scanner *jas.JasScanner) (results []*sarif.Run, err error) {
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
		sastScannerResults: []*sarif.Run{},
		scanner:            scanner,
	}
}

func (ssm *SastScanManager) Run(wd string) (err error) {
	scanner := ssm.scanner
	if err = ssm.runAnalyzerManager(wd); err != nil {
		return
	}
	workingDirResults, err := utils.ReadScanRunsFromFile(scanner.ResultsFileName)
	if err != nil {
		return
	}
	processSastScanResults(workingDirResults, wd)
	ssm.sastScannerResults = append(ssm.sastScannerResults, workingDirResults...)
	return
}

func (ssm *SastScanManager) runAnalyzerManager(wd string) error {
	return ssm.scanner.AnalyzerManager.Exec(ssm.scanner.ResultsFileName, sastScanCommand, wd, ssm.scanner.ServerDetails)
}

func processSastScanResults(sarifRuns []*sarif.Run, wd string) {
	for _, sastRun := range sarifRuns {
		// Change general attributes
		jas.ProcessJasScanRun(sastRun, wd)

		// Change specific scan attributes
		processedResults := map[string]*sarif.Result{}
		for index := 0; index < len(sastRun.Results); index++ {
			sastResult := sastRun.Results[index]
			resultID := GetResultId(sastResult)
			if result, exists := processedResults[resultID]; exists {
				// Combine this result with new code flow information to the already existing result
				result.CodeFlows = append(result.CodeFlows, sastResult.CodeFlows...)
				// Remove the duplicate result
				sastRun.Results = append(sastRun.Results[:index], sastRun.Results[index+1:]...)
				index--
			} else {
				processedResults[resultID] = sastResult
			}
		}
	}
}

// In Sast there is only one location for each result
func GetResultFileName(result *sarif.Result) string {
	if len(result.Locations) > 0 {
		return utils.GetLocationFileName(result.Locations[0])
	}
	return ""
}

// In Sast there is only one location for each result
func GetResultStartLocationInFile(result *sarif.Result) string {
	if len(result.Locations) > 0 {
		return utils.GetStartLocationInFile(result.Locations[0])
	}
	return ""
}

func GetResultId(result *sarif.Result) string {
	return GetResultFileName(result) + GetResultStartLocationInFile(result) + utils.GetResultSeverity(result) + *result.Message.Text
}
