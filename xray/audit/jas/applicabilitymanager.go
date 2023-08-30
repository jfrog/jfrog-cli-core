package jas

import (
	"path/filepath"
	"strings"

	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

const (
	applicabilityScanType    = "analyze-applicability"
	applicabilityScanCommand = "ca"
)

// The getApplicabilityScanResults function runs the applicability scan flow, which includes the following steps:
// Creating an ApplicabilityScanManager object.
// Checking if the scanned project is eligible for applicability scan.
// Running the analyzer manager executable.
// Parsing the analyzer manager results.
// Return values:
// map[string]string: A map containing the applicability result of each XRAY CVE.
// bool: true if the user is entitled to the applicability scan, false otherwise.
// error: An error object (if any).
func getApplicabilityScanResults(xrayResults []services.ScanResponse, directDependencies []string,
	scannedTechnologies []coreutils.Technology, scanner *AdvancedSecurityScanner) (results map[string]string, err error) {
	applicabilityScanManager := newApplicabilityScanManager(xrayResults, directDependencies, scanner)
	if !applicabilityScanManager.shouldRunApplicabilityScan(scannedTechnologies) {
		log.Debug("The technologies that have been scanned are currently not supported for contextual analysis scanning, or we couldn't find any vulnerable direct dependencies. Skipping....")
		return
	}
	if err = applicabilityScanManager.scanner.Run(applicabilityScanManager); err != nil {
		err = utils.ParseAnalyzerManagerError(utils.Applicability, err)
		return
	}
	results = applicabilityScanManager.applicabilityScanResults
	return
}

type ApplicabilityScanManager struct {
	applicabilityScanResults map[string]string
	directDependenciesCves   *datastructures.Set[string]
	xrayResults              []services.ScanResponse
	scanner                  *AdvancedSecurityScanner
}

func newApplicabilityScanManager(xrayScanResults []services.ScanResponse, directDependencies []string, scanner *AdvancedSecurityScanner) (manager *ApplicabilityScanManager) {
	directDependenciesCves := extractDirectDependenciesCvesFromScan(xrayScanResults, directDependencies)
	return &ApplicabilityScanManager{
		applicabilityScanResults: map[string]string{},
		directDependenciesCves:   directDependenciesCves,
		xrayResults:              xrayScanResults,
		scanner:                  scanner,
	}
}

// This function gets a list of xray scan responses that contain direct and indirect vulnerabilities and returns only direct
// vulnerabilities of the scanned project, ignoring indirect vulnerabilities
func extractDirectDependenciesCvesFromScan(xrayScanResults []services.ScanResponse, directDependencies []string) *datastructures.Set[string] {
	directsCves := datastructures.MakeSet[string]()
	for _, scanResult := range xrayScanResults {
		for _, vulnerability := range scanResult.Vulnerabilities {
			if isDirectComponents(maps.Keys(vulnerability.Components), directDependencies) {
				for _, cve := range vulnerability.Cves {
					if cve.Id != "" {
						directsCves.Add(cve.Id)
					}
				}
			}
		}
		for _, violation := range scanResult.Violations {
			if isDirectComponents(maps.Keys(violation.Components), directDependencies) {
				for _, cve := range violation.Cves {
					if cve.Id != "" {
						directsCves.Add(cve.Id)
					}
				}
			}
		}
	}

	return directsCves
}

func isDirectComponents(components []string, directDependencies []string) bool {
	for _, component := range components {
		if slices.Contains(directDependencies, component) {
			return true
		}
	}
	return false
}

func (a *ApplicabilityScanManager) Run(wd string) (err error) {
	if len(a.scanner.workingDirs) > 1 {
		log.Info("Running applicability scanning in the", wd, "directory...")
	} else {
		log.Info("Running applicability scanning...")
	}
	if err = a.createConfigFile(wd); err != nil {
		return
	}
	if err = a.runAnalyzerManager(); err != nil {
		return
	}
	var workingDirResults map[string]string
	if workingDirResults, err = a.getScanResults(); err != nil {
		return
	}
	for cve, result := range workingDirResults {
		a.applicabilityScanResults[cve] = result
	}
	return
}

func (a *ApplicabilityScanManager) directDependenciesExist() bool {
	return a.directDependenciesCves.Size() > 0
}

func (a *ApplicabilityScanManager) shouldRunApplicabilityScan(technologies []coreutils.Technology) bool {
	return a.directDependenciesExist() && coreutils.ContainsApplicabilityScannableTech(technologies)
}

type applicabilityScanConfig struct {
	Scans []scanConfiguration `yaml:"scans"`
}

type scanConfiguration struct {
	Roots        []string `yaml:"roots"`
	Output       string   `yaml:"output"`
	Type         string   `yaml:"type"`
	GrepDisable  bool     `yaml:"grep-disable"`
	CveWhitelist []string `yaml:"cve-whitelist"`
	SkippedDirs  []string `yaml:"skipped-folders"`
}

func (a *ApplicabilityScanManager) createConfigFile(workingDir string) error {
	configFileContent := applicabilityScanConfig{
		Scans: []scanConfiguration{
			{
				Roots:        []string{workingDir},
				Output:       a.scanner.resultsFileName,
				Type:         applicabilityScanType,
				GrepDisable:  false,
				CveWhitelist: a.directDependenciesCves.ToSlice(),
				SkippedDirs:  skippedDirs,
			},
		},
	}
	return createScannersConfigFile(a.scanner.configFileName, configFileContent)
}

// Runs the analyzerManager app and returns a boolean to indicate whether the user is entitled for
// advance security feature
func (a *ApplicabilityScanManager) runAnalyzerManager() error {
	return a.scanner.analyzerManager.Exec(a.scanner.configFileName, applicabilityScanCommand, filepath.Dir(a.scanner.analyzerManager.AnalyzerManagerFullPath), a.scanner.serverDetails)
}

func (a *ApplicabilityScanManager) getScanResults() (map[string]string, error) {
	report, err := sarif.Open(a.scanner.resultsFileName)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}
	var fullVulnerabilitiesList []*sarif.Result
	if len(report.Runs) > 0 {
		fullVulnerabilitiesList = report.Runs[0].Results
	}

	applicabilityScanResults := make(map[string]string)
	for _, cve := range a.directDependenciesCves.ToSlice() {
		applicabilityScanResults[cve] = utils.ApplicabilityUndeterminedStringValue
	}

	for _, vulnerability := range fullVulnerabilitiesList {
		applicableVulnerabilityName := getVulnerabilityName(*vulnerability.RuleID)
		if isVulnerabilityApplicable(vulnerability) {
			applicabilityScanResults[applicableVulnerabilityName] = utils.ApplicableStringValue
		} else {
			applicabilityScanResults[applicableVulnerabilityName] = utils.NotApplicableStringValue
		}
	}
	return applicabilityScanResults, nil
}

// Gets a result of one CVE from the scanner, and returns true if the CVE is applicable, false otherwise
func isVulnerabilityApplicable(result *sarif.Result) bool {
	return !(result.Kind != nil && *result.Kind == "pass")
}

func getVulnerabilityName(sarifRuleId string) string {
	return strings.TrimPrefix(sarifRuleId, "applic_")
}
