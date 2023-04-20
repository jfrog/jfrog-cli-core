package jas

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/sarif"
	"gopkg.in/yaml.v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	applicabilityScanCommand = "ca"
	applicabilityScanType    = "analyze-applicability"
)

var (
	analyzerManagerExecuter                  AnalyzerManager = &analyzerManager{}
	technologiesEligibleForApplicabilityScan                 = []coreutils.Technology{coreutils.Npm, coreutils.Pip,
		coreutils.Poetry, coreutils.Pipenv, coreutils.Pypi}
)

type ExtendedScanResults struct {
	XrayResults                 []services.ScanResponse
	ApplicabilityScannerResults map[string]string
	EntitledForJas              bool
}

func (e *ExtendedScanResults) GetXrayScanResults() []services.ScanResponse {
	return e.XrayResults
}

func GetExtendedScanResults(results []services.ScanResponse, dependencyTrees []*services.GraphNode, serverDetails *config.ServerDetails) (*ExtendedScanResults, error) {
	applicabilityScanManager, err := NewApplicabilityScanManager(results, dependencyTrees, serverDetails)
	if err != nil {
		return handleApplicabilityScanError(err, applicabilityScanManager)
	}
	if !applicabilityScanManager.shouldRun() {
		log.Info("user not entitled for jas, didnt execute applicability scan")
		return &ExtendedScanResults{XrayResults: results, ApplicabilityScannerResults: nil, EntitledForJas: false}, nil
	}
	entitledForJas, err := applicabilityScanManager.Run()
	if !entitledForJas {
		log.Info("got not entitled error from analyzer manager")
		return &ExtendedScanResults{XrayResults: results, ApplicabilityScannerResults: nil, EntitledForJas: false}, nil
	}
	if err != nil {
		return handleApplicabilityScanError(err, applicabilityScanManager)
	}
	applicabilityScanResults := applicabilityScanManager.getApplicabilityScanResults()
	extendedScanResults := ExtendedScanResults{XrayResults: results, ApplicabilityScannerResults: applicabilityScanResults, EntitledForJas: true}
	return &extendedScanResults, nil
}

func (a *ApplicabilityScanManager) shouldRun() bool {
	return a.analyzerManager.DoesAnalyzerManagerExecutableExist() && a.resultsIncludeEligibleTechnologies() &&
		(len(a.xrayVulnerabilities) != 0 || len(a.xrayViolations) != 0)
}

func (a *ApplicabilityScanManager) resultsIncludeEligibleTechnologies() bool {
	for _, vuln := range a.xrayVulnerabilities {
		for _, technology := range technologiesEligibleForApplicabilityScan {
			if vuln.Technology == technology.ToString() {
				return true
			}
		}
	}
	for _, vuolation := range a.xrayViolations {
		for _, technology := range technologiesEligibleForApplicabilityScan {
			if vuolation.Technology == technology.ToString() {
				return true
			}
		}
	}
	return false
}

func handleApplicabilityScanError(err error, applicabilityScanManager *ApplicabilityScanManager) (*ExtendedScanResults, error) {
	log.Info("failed to run applicability scan: " + err.Error())
	deleteFilesError := applicabilityScanManager.DeleteApplicabilityScanProcessFiles()
	if deleteFilesError != nil {
		return nil, deleteFilesError
	}
	return nil, err
}

type ApplicabilityScanManager struct {
	applicabilityScannerResults map[string]string
	xrayVulnerabilities         []services.Vulnerability
	xrayViolations              []services.Violation
	configFileName              string
	resultsFileName             string
	analyzerManager             AnalyzerManager
	serverDetails               *config.ServerDetails
}

func NewApplicabilityScanManager(xrayScanResults []services.ScanResponse, dependencyTrees []*services.GraphNode, serverDetails *config.ServerDetails) (*ApplicabilityScanManager, error) {
	directDependencies := getDirectDependenciesList(dependencyTrees)
	configFileName, err := generateRandomFileName()
	if err != nil {
		return nil, err
	}
	resultsFileName, err := generateRandomFileName()
	if err != nil {
		return nil, err
	}
	return &ApplicabilityScanManager{
		applicabilityScannerResults: map[string]string{},
		xrayVulnerabilities:         setXrayDirectVulnerabilities(xrayScanResults, directDependencies),
		xrayViolations:              setXrayDirectViolations(xrayScanResults, directDependencies),
		configFileName:              configFileName + ".yaml",
		resultsFileName:             resultsFileName + ".sarif",
		analyzerManager:             analyzerManagerExecuter,
		serverDetails:               serverDetails,
	}, nil
}

func setXrayDirectViolations(xrayScanResults []services.ScanResponse, directDependencies []string) []services.Violation {
	xrayViolationsDirectDependency := []services.Violation{}
	for _, violation := range getXrayViolations(xrayScanResults) {
		for _, dep := range directDependencies {
			if _, ok := violation.Components[dep]; ok {
				xrayViolationsDirectDependency = append(xrayViolationsDirectDependency, violation)
			}
		}
	}
	return xrayViolationsDirectDependency
}

func setXrayDirectVulnerabilities(xrayScanResults []services.ScanResponse, directDependencies []string) []services.Vulnerability {
	xrayVulnerabilitiesDirectDependency := []services.Vulnerability{}
	for _, vulnerability := range getXrayVulnerabilities(xrayScanResults) {
		for _, dep := range directDependencies {
			if _, ok := vulnerability.Components[dep]; ok {
				xrayVulnerabilitiesDirectDependency = append(xrayVulnerabilitiesDirectDependency, vulnerability)
			}
		}
	}
	return xrayVulnerabilitiesDirectDependency
}

func getDirectDependenciesList(dependencyTrees []*services.GraphNode) []string {
	directDependencies := []string{}
	for _, tree := range dependencyTrees {
		for _, node := range tree.Nodes {
			directDependencies = append(directDependencies, node.Id)
		}
	}
	return directDependencies
}

func getXrayVulnerabilities(xrayScanResults []services.ScanResponse) []services.Vulnerability {
	xrayVulnerabilities := []services.Vulnerability{}
	for _, result := range xrayScanResults {
		xrayVulnerabilities = append(xrayVulnerabilities, result.Vulnerabilities...)
	}
	return xrayVulnerabilities
}

func getXrayViolations(xrayScanResults []services.ScanResponse) []services.Violation {
	xrayViolations := []services.Violation{}
	for _, result := range xrayScanResults {
		xrayViolations = append(xrayViolations, result.Violations...)
	}
	return xrayViolations
}

func (a *ApplicabilityScanManager) getApplicabilityScanResults() map[string]string {
	return a.applicabilityScannerResults
}

func (a *ApplicabilityScanManager) Run() (bool, error) {
	if err := a.createConfigFile(); err != nil {
		return true, err
	}
	if entitledForJas, err := a.runAnalyzerManager(); err != nil {
		return entitledForJas, err
	}
	if err := a.parseResults(); err != nil {
		return true, err
	}
	if err := a.DeleteApplicabilityScanProcessFiles(); err != nil {
		return true, err
	}
	return true, nil
}

type applicabilityScanConfig struct {
	Scans []scanConfiguration `yaml:"scans"`
}

type scanConfiguration struct {
	Roots          []string `yaml:"roots"`
	Output         string   `yaml:"output"`
	Type           string   `yaml:"type"`
	GrepDisable    bool     `yaml:"grep-disable"`
	CveWhitelist   []string `yaml:"cve-whitelist"`
	SkippedFolders []string `yaml:"skipped-folders"`
}

func (a *ApplicabilityScanManager) createConfigFile() error {
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}
	cveWhiteList := removeDuplicateValues(a.createCveList())
	configFileContent := applicabilityScanConfig{
		Scans: []scanConfiguration{
			{
				Roots:          []string{currentDir},
				Output:         filepath.Join(currentDir, a.resultsFileName),
				Type:           applicabilityScanType,
				GrepDisable:    false,
				CveWhitelist:   cveWhiteList,
				SkippedFolders: []string{},
			},
		},
	}
	yamlData, err := yaml.Marshal(&configFileContent)
	if err != nil {
		return err
	}
	err = os.WriteFile(a.configFileName, yamlData, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (a *ApplicabilityScanManager) runAnalyzerManager() (bool, error) {
	err := setAnalyzerManagerEnvVariables(a.serverDetails)
	if err != nil {
		return true, err
	}
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return true, err
	}
	err = a.analyzerManager.RunAnalyzerManager(filepath.Join(currentDir, a.configFileName))
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode := exitError.ExitCode()
			if exitCode == 31 { // user not entitled error
				return false, err
			}
		}
		return true, err
	}
	return true, nil
}

func (a *ApplicabilityScanManager) parseResults() error {
	report, err := sarif.Open(a.resultsFileName)
	if err != nil {
		return err
	}
	var fullVulnerabilitiesList []*sarif.Result
	if len(report.Runs) > 0 {
		fullVulnerabilitiesList = report.Runs[0].Results
	}

	xrayCves := removeDuplicateValues(a.createCveList())
	for _, xrayCve := range xrayCves {
		a.applicabilityScannerResults[xrayCve] = "unknown"
	}

	for _, vulnerability := range fullVulnerabilitiesList {
		applicableVulnerabilityName := getVulnerabilityName(*vulnerability.RuleID)
		if isVulnerabilityApplicable(vulnerability) {
			a.applicabilityScannerResults[applicableVulnerabilityName] = "Yes"
		} else {
			a.applicabilityScannerResults[applicableVulnerabilityName] = "No"
		}
	}
	return nil
}

func (a *ApplicabilityScanManager) DeleteApplicabilityScanProcessFiles() error {
	if _, err := os.Stat(a.configFileName); err == nil {
		err = os.Remove(a.configFileName)
		if err != nil {
			return err
		}
	}
	if _, err := os.Stat(a.resultsFileName); err == nil {
		err = os.Remove(a.resultsFileName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *ApplicabilityScanManager) createCveList() []string {
	cveWhiteList := []string{}
	for _, vulnerability := range a.xrayVulnerabilities {
		for _, cve := range vulnerability.Cves {
			if cve.Id != "" {
				cveWhiteList = append(cveWhiteList, cve.Id)
			}
		}
	}
	for _, violation := range a.xrayViolations {
		for _, cve := range violation.Cves {
			if cve.Id != "" {
				cveWhiteList = append(cveWhiteList, cve.Id)
			}
		}
	}
	return cveWhiteList
}

func isVulnerabilityApplicable(vulnerability *sarif.Result) bool {
	return !(vulnerability.Kind != nil && *vulnerability.Kind == "pass")
}

func getVulnerabilityName(sarifRuleId string) string {
	return strings.Split(sarifRuleId, "_")[1]
}

type AnalyzerManager interface {
	DoesAnalyzerManagerExecutableExist() bool
	RunAnalyzerManager(string) error
}

type analyzerManager struct {
}

func (am *analyzerManager) DoesAnalyzerManagerExecutableExist() bool {
	if _, err := os.Stat(getAnalyzerManagerAbsolutePath()); err != nil {
		return false
	}
	return true
}

func (am *analyzerManager) RunAnalyzerManager(configFile string) error {
	var err error
	if runtime.GOOS == "windows" {
		_, err = exec.Command(getAnalyzerManagerAbsolutePath()+".exe", applicabilityScanCommand, configFile).Output()
	} else {
		_, err = exec.Command(getAnalyzerManagerAbsolutePath(), applicabilityScanCommand, configFile).Output()
	}
	if err != nil {
		return err
	}
	return nil
}
