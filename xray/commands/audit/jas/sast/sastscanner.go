package sast

import (
	"strconv"

	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"golang.org/x/exp/maps"
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
		log.Info("Found", utils.GetResultsLocationCount(sastScanManager.sastScannerResults...), "SAST vulnerabilities")
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
	workingDirRuns, err := jas.ReadJasScanRunsFromFile(scanner.ResultsFileName, wd)
	if err != nil {
		return
	}
	groupResultsByLocation(workingDirRuns)
	ssm.sastScannerResults = append(ssm.sastScannerResults, workingDirRuns...)
	return
}

func (ssm *SastScanManager) runAnalyzerManager(wd string) error {
	return ssm.scanner.AnalyzerManager.Exec(ssm.scanner.ResultsFileName, sastScanCommand, wd, ssm.scanner.ServerDetails)
}

// In the Sast scanner, there can be multiple results with the same location.
// The only difference is that their CodeFlow values are different.
// We combine those under the same result location value
func groupResultsByLocation(sarifRuns []*sarif.Run) {
	for _, sastRun := range sarifRuns {
		locationToResult := map[string]*sarif.Result{}
		for _, sastResult := range sastRun.Results {
			resultID := getResultId(sastResult)
			if result, exists := locationToResult[resultID]; exists {
				result.CodeFlows = append(result.CodeFlows, sastResult.CodeFlows...)
			} else {
				locationToResult[resultID] = sastResult
			}
		}
		sastRun.Results = maps.Values(locationToResult)
	}
}

// In Sast there is only one location for each result
func getResultLocationStr(result *sarif.Result) string {
	if len(result.Locations) == 0 {
		return ""
	}
	return utils.GetLocationFileName(result.Locations[0]) +
		strconv.Itoa(utils.GetLocationStartLine(result.Locations[0])) +
		strconv.Itoa(utils.GetLocationStartColumn(result.Locations[0])) +
		strconv.Itoa(utils.GetLocationEndLine(result.Locations[0])) +
		strconv.Itoa(utils.GetLocationEndColumn(result.Locations[0]))
}

func getResultRuleId(result *sarif.Result) string {
	if result.RuleID == nil {
		return ""
	}
	return *result.RuleID
}

func getResultId(result *sarif.Result) string {
	return getResultRuleId(result) + utils.GetResultSeverity(result) + utils.GetResultMsgText(result) + getResultLocationStr(result)
}
