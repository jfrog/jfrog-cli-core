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
	workingDirResults, err := jas.ReadJasScanRunsFromFile(scanner.ResultsFileName, wd)
	if err != nil {
		return
	}
	ssm.sastScannerResults = append(ssm.sastScannerResults, processSastScanResults(workingDirResults)...)
	return
}

func (ssm *SastScanManager) runAnalyzerManager(wd string) error {
	return ssm.scanner.AnalyzerManager.Exec(ssm.scanner.ResultsFileName, sastScanCommand, wd, ssm.scanner.ServerDetails)
}

// In the Sast scanner, there can be multiple results with the same location.
// The only difference is that their CodeFlow values are different.
// We combine those under the same result location value
func processSastScanResults(sarifRuns []*sarif.Run) (processed []*sarif.Run) {
	for _, sastRun := range sarifRuns {
		processedResults := map[string]*sarif.Result{}
		for _, sastResult := range sastRun.Results {
			resultID := GetResultId(sastResult)
			if result, exists := processedResults[resultID]; exists {
				result.CodeFlows = append(result.CodeFlows, sastResult.CodeFlows...)
			} else {
				processedResults[resultID] = sastResult
			}
		}
		// Register processed results as run
		resultSlice := []*sarif.Result{}
		for _, result := range processedResults {
			resultSlice = append(resultSlice, result)
		}
		processed = append(processed, sarif.NewRun(sastRun.Tool).WithResults(resultSlice))
	}
	return
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
