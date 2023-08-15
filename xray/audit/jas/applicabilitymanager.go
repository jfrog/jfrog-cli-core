package jas

import (
	"errors"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"golang.org/x/exp/maps"
	"path/filepath"
	"strings"
)

const (
	ApplicabilityFeatureId   = "contextual_analysis"
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
func getApplicabilityScanResults(results []services.ScanResponse, dependencyTrees []*xrayUtils.GraphNode,
	serverDetails *config.ServerDetails, scannedTechnologies []coreutils.Technology, workingDirs []string, analyzerManager utils.AnalyzerManagerInterface) (map[string]string, bool, error) {
	applicabilityScanManager, cleanupFunc, err := newApplicabilityScanManager(results, dependencyTrees, serverDetails, workingDirs, analyzerManager)
	if err != nil {
		return nil, false, utils.Applicability.FormattedError(err)
	}
	defer func() {
		if cleanupFunc != nil {
			err = errors.Join(err, cleanupFunc())
		}
	}()
	if !applicabilityScanManager.shouldRunApplicabilityScan(scannedTechnologies) {
		log.Debug("The technologies that have been scanned are currently not supported for contextual analysis scanning, or we couldn't find any vulnerable direct dependencies. Skipping....")
		return nil, false, nil
	}
	if err = applicabilityScanManager.run(); err != nil {
		return nil, false, utils.ParseAnalyzerManagerError(utils.Applicability, err)
	}
	return applicabilityScanManager.applicabilityScanResults, true, nil
}

type ApplicabilityScanManager struct {
	applicabilityScanResults map[string]string
	directDependenciesCves   *datastructures.Set[string]
	xrayResults              []services.ScanResponse
	configFileName           string
	resultsFileName          string
	analyzerManager          utils.AnalyzerManagerInterface
	serverDetails            *config.ServerDetails
	workingDirs              []string
}

func newApplicabilityScanManager(xrayScanResults []services.ScanResponse, dependencyTrees []*xrayUtils.GraphNode,
	serverDetails *config.ServerDetails, workingDirs []string, analyzerManager utils.AnalyzerManagerInterface) (manager *ApplicabilityScanManager, cleanup func() error, err error) {
	directDependencies := getDirectDependenciesSet(dependencyTrees)
	tempDir, err := fileutils.CreateTempDir()
	if err != nil {
		return
	}
	cleanup = func() error {
		return fileutils.RemoveTempDir(tempDir)
	}
	fullPathWorkingDirs, err := utils.GetFullPathsWorkingDirs(workingDirs)
	if err != nil {
		return nil, cleanup, err
	}
	directDependenciesCves := extractDirectDependenciesCvesFromScan(xrayScanResults, directDependencies)
	return &ApplicabilityScanManager{
		applicabilityScanResults: map[string]string{},
		directDependenciesCves:   directDependenciesCves,
		configFileName:           filepath.Join(tempDir, "config.yaml"),
		resultsFileName:          filepath.Join(tempDir, "results.sarif"),
		xrayResults:              xrayScanResults,
		analyzerManager:          analyzerManager,
		serverDetails:            serverDetails,
		workingDirs:              fullPathWorkingDirs,
	}, cleanup, nil
}

// This function gets a list of xray scan responses that contain direct and indirect vulnerabilities and returns only direct
// vulnerabilities of the scanned project, ignoring indirect vulnerabilities
func extractDirectDependenciesCvesFromScan(xrayScanResults []services.ScanResponse, directDependencies *datastructures.Set[string]) *datastructures.Set[string] {
	directsCves := datastructures.MakeSet[string]()
	for _, scanResult := range xrayScanResults {
		for _, vulnerability := range scanResult.Vulnerabilities {
			if isDirectComponents(maps.Keys(vulnerability.Components), directDependencies) {
				for _, cve := range vulnerability.Cves {
					directsCves.Add(cve.Id)
				}
			}
		}
		for _, violation := range scanResult.Violations {
			if isDirectComponents(maps.Keys(violation.Components), directDependencies) {
				for _, cve := range violation.Cves {
					directsCves.Add(cve.Id)
				}
			}
		}
	}

	return directsCves
}

func isDirectComponents(components []string, directDependencies *datastructures.Set[string]) bool {
	for _, component := range components {
		if directDependencies.Exists(component) {
			return true
		}
	}
	return false
}

// This function retrieves the dependency trees of the scanned project and extracts a set that contains only the direct dependencies.
func getDirectDependenciesSet(dependencyTrees []*xrayUtils.GraphNode) *datastructures.Set[string] {
	directDependencies := datastructures.MakeSet[string]()
	for _, tree := range dependencyTrees {
		for _, node := range tree.Nodes {
			directDependencies.Add(node.Id)
		}
	}
	return directDependencies
}

func (a *ApplicabilityScanManager) run() (err error) {
	log.Info("Running applicability scanning for the identified vulnerable direct dependencies...")
	for _, workingDir := range a.workingDirs {
		var workingDirResults map[string]string
		if workingDirResults, err = a.runApplicabilityScan(workingDir); err != nil {
			return
		}
		for cve, result := range workingDirResults {
			a.applicabilityScanResults[cve] = result
		}
	}
	return
}

func (a *ApplicabilityScanManager) runApplicabilityScan(workingDir string) (results map[string]string, err error) {
	defer func() {
		err = errors.Join(err, deleteJasProcessFiles(a.configFileName, a.resultsFileName))
	}()
	if err = a.createConfigFile(workingDir); err != nil {
		return
	}
	if err = a.runAnalyzerManager(); err != nil {
		return
	}
	results, err = a.getScanResults()
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
				Output:       a.resultsFileName,
				Type:         applicabilityScanType,
				GrepDisable:  false,
				CveWhitelist: a.directDependenciesCves.ToSlice(),
				SkippedDirs:  skippedDirs,
			},
		},
	}
	return createScannersConfigFile(a.configFileName, configFileContent)
}

// Runs the analyzerManager app and returns a boolean to indicate whether the user is entitled for
// advance security feature
func (a *ApplicabilityScanManager) runAnalyzerManager() error {
	return a.analyzerManager.Exec(a.configFileName, applicabilityScanCommand, a.serverDetails)
}

func (a *ApplicabilityScanManager) getScanResults() (map[string]string, error) {
	report, err := sarif.Open(a.resultsFileName)
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
