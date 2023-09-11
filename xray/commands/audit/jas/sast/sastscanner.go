package sast

import (
	"fmt"

	"path/filepath"

	jfrogappsconfig "github.com/jfrog/jfrog-apps-config/go"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"golang.org/x/exp/maps"
)

const (
	sastScannerType = "sast"
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

func (ssm *SastScanManager) Run(module jfrogappsconfig.Module) (err error) {
	if jas.ShouldSkipScanner(module, utils.Sast) {
		return
	}
	if err = ssm.createConfigFile(module); err != nil {
		return
	}
	scanner := ssm.scanner
	if err = ssm.runAnalyzerManager(filepath.Dir(ssm.scanner.AnalyzerManager.AnalyzerManagerFullPath)); err != nil {
		return
	}
	workingDirRuns, err := jas.ReadJasScanRunsFromFile(scanner.ResultsFileName, module.SourceRoot)
	if err != nil {
		return
	}
	groupResultsByLocation(workingDirRuns)
	ssm.sastScannerResults = append(ssm.sastScannerResults, workingDirRuns...)
	return
}

type sastScanConfig struct {
	Scans []scanConfiguration `yaml:"scans,omitempty"`
}

type scanConfiguration struct {
	Roots           []string `yaml:"roots,omitempty"`
	Type            string   `yaml:"type,omitempty"`
	Language        string   `yaml:"language,omitempty"`
	ExcludePatterns []string `yaml:"exclude_patterns,omitempty"`
	ExcludedRules   []string `yaml:"excluded-rules,omitempty"`
}

func (ssm *SastScanManager) createConfigFile(module jfrogappsconfig.Module) error {
	sastScanner := module.Scanners.Sast
	if sastScanner == nil {
		sastScanner = &jfrogappsconfig.SastScanner{}
	}
	roots, err := jas.GetSourceRoots(module, &sastScanner.Scanner)
	if err != nil {
		return err
	}
	configFileContent := sastScanConfig{
		Scans: []scanConfiguration{
			{
				Type:            sastScannerType,
				Roots:           roots,
				Language:        sastScanner.Language,
				ExcludedRules:   sastScanner.ExcludedRules,
				ExcludePatterns: jas.GetExcludePatterns(module, &sastScanner.Scanner),
			},
		},
	}
	return jas.CreateScannersConfigFile(ssm.scanner.ConfigFileName, configFileContent)
}

func (ssm *SastScanManager) runAnalyzerManager(wd string) error {
	return ssm.scanner.AnalyzerManager.ExecWithOutputFile(ssm.scanner.ConfigFileName, sastScanCommand, wd, ssm.scanner.ResultsFileName, ssm.scanner.ServerDetails)
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

func getResultLocationStr(result *sarif.Result) string {
	if len(result.Locations) == 0 {
		return ""
	}
	location := result.Locations[0]
	return fmt.Sprintf("%s%d%d%d%d",
		utils.GetLocationFileName(location),
		utils.GetLocationStartLine(location),
		utils.GetLocationStartColumn(location),
		utils.GetLocationEndLine(location),
		utils.GetLocationEndColumn(location))
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
