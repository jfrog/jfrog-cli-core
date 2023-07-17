package jas

import (
	"errors"
	"fmt"
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
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
	"strings"
)

const (
	ApplicabilityFeatureId          = "contextual_analysis"
	applicabilityScanType           = "analyze-applicability"
	applicabilityScanFailureMessage = "failed to run applicability scan. Cause: %s"
	applicabilityScanCommand        = "ca"
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
	serverDetails *config.ServerDetails, scannedTechnologies []coreutils.Technology, analyzerManager utils.AnalyzerManagerInterface) (map[string]string, bool, error) {
	if !coreutils.ContainsApplicabilityScannableTech(scannedTechnologies) {
		log.Debug("The technologies that have been scanned are currently not supported for contextual analysis scanning. Skipping...")
		return nil, false, nil
	}
	applicabilityScanManager, cleanupFunc, err := newApplicabilityScanManager(results, dependencyTrees, serverDetails, analyzerManager)
	if err != nil {
		return nil, false, fmt.Errorf(applicabilityScanFailureMessage, err.Error())
	}
	defer func() {
		if cleanupFunc != nil {
			err = errors.Join(err, cleanupFunc())
		}
	}()
	if err = applicabilityScanManager.run(); err != nil {
		if utils.IsNotEntitledError(err) || utils.IsUnsupportedCommandError(err) {
			return nil, false, nil
		}
		return nil, true, fmt.Errorf(applicabilityScanFailureMessage, err.Error())
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
}

func newApplicabilityScanManager(xrayScanResults []services.ScanResponse, dependencyTrees []*xrayUtils.GraphNode,
	serverDetails *config.ServerDetails, analyzerManager utils.AnalyzerManagerInterface) (manager *ApplicabilityScanManager, cleanup func() error, err error) {
	directDependencies := getDirectDependenciesSet(dependencyTrees)
	tempDir, err := fileutils.CreateTempDir()
	if err != nil {
		return
	}
	cleanup = func() error {
		return fileutils.RemoveTempDir(tempDir)
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
	}, cleanup, nil
}

// This function gets a liat of xray scan responses that contains direct and indirect vulnerabilities, and returns only direct
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
	defer func() {
		if deleteJasProcessFiles(a.configFileName, a.resultsFileName) != nil {
			deleteFilesError := deleteJasProcessFiles(a.configFileName, a.resultsFileName)
			err = errors.Join(err, deleteFilesError)
		}
	}()
	if !a.directDependenciesExist() {
		return nil
	}
	log.Info("Running applicability scanning for the identified vulnerable dependencies...")
	if err = a.createConfigFile(); err != nil {
		return
	}
	if err = a.runAnalyzerManager(); err != nil {
		return
	}
	return a.setScanResults()
}

func (a *ApplicabilityScanManager) directDependenciesExist() bool {
	return a.directDependenciesCves.Size() > 0
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

func (a *ApplicabilityScanManager) createConfigFile() error {
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}
	configFileContent := applicabilityScanConfig{
		Scans: []scanConfiguration{
			{
				Roots:        []string{currentDir},
				Output:       a.resultsFileName,
				Type:         applicabilityScanType,
				GrepDisable:  false,
				CveWhitelist: a.directDependenciesCves.ToSlice(),
				SkippedDirs:  skippedDirs,
			},
		},
	}
	yamlData, err := yaml.Marshal(&configFileContent)
	if errorutils.CheckError(err) != nil {
		return err
	}
	err = os.WriteFile(a.configFileName, yamlData, 0644)
	return errorutils.CheckError(err)
}

// Runs the analyzerManager app and returns a boolean to indicate whether the user is entitled for
// advance security feature
func (a *ApplicabilityScanManager) runAnalyzerManager() error {
	if err := utils.SetAnalyzerManagerEnvVariables(a.serverDetails); err != nil {
		return err
	}
	return a.analyzerManager.Exec(a.configFileName, applicabilityScanCommand)
}

func (a *ApplicabilityScanManager) setScanResults() error {
	report, err := sarif.Open(a.resultsFileName)
	if errorutils.CheckError(err) != nil {
		return err
	}
	var fullVulnerabilitiesList []*sarif.Result
	if len(report.Runs) > 0 {
		fullVulnerabilitiesList = report.Runs[0].Results
	}

	xrayCves := a.directDependenciesCves.ToSlice()
	for _, xrayCve := range xrayCves {
		a.applicabilityScanResults[xrayCve] = utils.ApplicabilityUndeterminedStringValue
	}

	for _, vulnerability := range fullVulnerabilitiesList {
		applicableVulnerabilityName := getVulnerabilityName(*vulnerability.RuleID)
		if isVulnerabilityApplicable(vulnerability) {
			a.applicabilityScanResults[applicableVulnerabilityName] = utils.ApplicableStringValue
		} else {
			a.applicabilityScanResults[applicableVulnerabilityName] = utils.NotApplicableStringValue
		}
	}
	return nil
}

// This function iterate the direct vulnerabilities and violations of the scanned projects, and creates a string list
// of the CVEs ids. This list will be sent as input to analyzer manager.
func createCveList(xrayVulnerabilities []services.Vulnerability, xrayViolations []services.Violation) []string {
	cveWhiteList := []string{}
	for _, vulnerability := range xrayVulnerabilities {
		for _, cve := range vulnerability.Cves {
			if cve.Id != "" {
				cveWhiteList = append(cveWhiteList, cve.Id)
			}
		}
	}
	for _, violation := range xrayViolations {
		for _, cve := range violation.Cves {
			if cve.Id != "" {
				cveWhiteList = append(cveWhiteList, cve.Id)
			}
		}
	}
	return cveWhiteList
}

// Gets a result of one CVE from the scanner, and returns true if the CVE is applicable, false otherwise
func isVulnerabilityApplicable(result *sarif.Result) bool {
	return !(result.Kind != nil && *result.Kind == "pass")
}

func getVulnerabilityName(sarifRuleId string) string {
	return strings.TrimPrefix(sarifRuleId, "applic_")
}
