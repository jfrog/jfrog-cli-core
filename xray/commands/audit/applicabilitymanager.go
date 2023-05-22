package audit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"gopkg.in/yaml.v2"
)

const (
	ApplicabilityFeatureId          = "contextual_analysis"
	applicabilityScanType           = "analyze-applicability"
	applicabilityScanFailureMessage = "failed to run applicability scan. Cause: %s"
	noEntitledExitCode              = 31
)

var (
	analyzerManagerExecuter                  utils.AnalyzerManagerInterface = &utils.AnalyzerManager{}
	technologiesEligibleForApplicabilityScan                                = []coreutils.Technology{coreutils.Npm, coreutils.Pip,
		coreutils.Poetry, coreutils.Pipenv, coreutils.Pypi}
	skippedDirs = []string{"**/*test*/**", "**/*venv*/**", "**/*node_modules*/**", "**/*target*/**"}
)

func GetExtendedScanResults(results []services.ScanResponse, dependencyTrees []*xrayUtils.GraphNode,
	serverDetails *config.ServerDetails) (extendedResults *utils.ExtendedScanResults, err error) {
	if err = utils.CreateAnalyzerManagerLogDir(); err != nil {
		return
	}
	applicabilityScanManager, cleanupFunc, err := NewApplicabilityScanManager(results, dependencyTrees, serverDetails)
	if err != nil {
		return nil, fmt.Errorf(applicabilityScanFailureMessage, err.Error())
	}
	defer func() {
		if cleanupFunc != nil {
			e := cleanupFunc()
			if err == nil {
				err = e
			}
		}
	}()
	shouldRun, err := applicabilityScanManager.shouldRun()
	if err != nil {
		return nil, fmt.Errorf(applicabilityScanFailureMessage, err.Error())
	}
	if !shouldRun {
		if len(serverDetails.Url) > 0 {
			log.Warn("To include 'Contextual Analysis' information as part of the audit output, please run the 'jf c add' command before running this command.")
		}
		log.Debug("The conditions required for running 'Contextual Analysis' as part of the audit are not met.")
		return &utils.ExtendedScanResults{XrayResults: results, ApplicabilityScannerResults: nil, EntitledForJas: false}, nil
	}
	entitledForJas, err := applicabilityScanManager.Run()
	if err != nil {
		return nil, fmt.Errorf(applicabilityScanFailureMessage, err.Error())
	}
	if !entitledForJas {
		log.Debug("the current user is not entitled for the Advanced Security package")
		return &utils.ExtendedScanResults{XrayResults: results, ApplicabilityScannerResults: nil, EntitledForJas: false}, nil
	}
	applicabilityScanResults := applicabilityScanManager.getApplicabilityScanResults()
	extendedScanResults := utils.ExtendedScanResults{XrayResults: results, ApplicabilityScannerResults: applicabilityScanResults, EntitledForJas: true}
	return &extendedScanResults, nil
}

func (a *ApplicabilityScanManager) shouldRun() (bool, error) {
	analyzerManagerExist, err := a.analyzerManager.ExistLocally()
	if err != nil {
		return false, err
	}
	return analyzerManagerExist && resultsIncludeEligibleTechnologies(a.xrayVulnerabilities, a.xrayViolations) &&
		len(createCveList(a.xrayVulnerabilities, a.xrayViolations)) > 0 && len(a.serverDetails.Url) > 0, nil
}

// Applicability scan is relevant only to specific programming languages (the languages in this list:
// technologiesEligibleForApplicabilityScan). therefore, the applicability scan will not be performed on projects that
// do not contain those technologies.
// resultsIncludeEligibleTechnologies() runs over the xray scan results, and check if at least one of them is one of
// the techs on technologiesEligibleForApplicabilityScan. otherwise, the applicability scan will not be executed.
func resultsIncludeEligibleTechnologies(xrayVulnerabilities []services.Vulnerability, xrayViolations []services.Violation) bool {
	for _, vuln := range xrayVulnerabilities {
		for _, technology := range technologiesEligibleForApplicabilityScan {
			if vuln.Technology == technology.ToString() {
				return true
			}
		}
	}
	for _, violation := range xrayViolations {
		for _, technology := range technologiesEligibleForApplicabilityScan {
			if violation.Technology == technology.ToString() {
				return true
			}
		}
	}
	return false
}

type ApplicabilityScanManager struct {
	applicabilityScannerResults map[string]string
	xrayVulnerabilities         []services.Vulnerability
	xrayViolations              []services.Violation
	configFileName              string
	resultsFileName             string
	analyzerManager             utils.AnalyzerManagerInterface
	serverDetails               *config.ServerDetails
}

func NewApplicabilityScanManager(xrayScanResults []services.ScanResponse, dependencyTrees []*xrayUtils.GraphNode,
	serverDetails *config.ServerDetails) (manager *ApplicabilityScanManager, cleanup func() error, err error) {
	directDependencies := getDirectDependenciesList(dependencyTrees)
	tempDir, err := fileutils.CreateTempDir()
	if err != nil {
		return
	}
	cleanup = func() error {
		return fileutils.RemoveTempDir(tempDir)
	}
	return &ApplicabilityScanManager{
		applicabilityScannerResults: map[string]string{},
		xrayVulnerabilities:         extractXrayDirectVulnerabilities(xrayScanResults, directDependencies),
		xrayViolations:              extractXrayDirectViolations(xrayScanResults, directDependencies),
		configFileName:              filepath.Join(tempDir, "config.yaml"),
		resultsFileName:             filepath.Join(tempDir, "results.sarif"),
		analyzerManager:             analyzerManagerExecuter,
		serverDetails:               serverDetails,
	}, cleanup, nil
}

// This function gets a liat of xray scan responses that contains direct and indirect violations, and returns only direct
// violation of the scanned project, ignoring indirect violations
func extractXrayDirectViolations(xrayScanResults []services.ScanResponse, directDependencies []string) []services.Violation {
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

// This function gets a liat of xray scan responses that contains direct and indirect vulnerabilities, and returns only direct
// vulnerabilities of the scanned project, ignoring indirect vulnerabilities
func extractXrayDirectVulnerabilities(xrayScanResults []services.ScanResponse, directDependencies []string) []services.Vulnerability {
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

// This function gets the dependencies tress of the scanned project, and extract a list containing only directed
// dependencies node ids.
func getDirectDependenciesList(dependencyTrees []*xrayUtils.GraphNode) []string {
	directDependencies := []string{}
	for _, tree := range dependencyTrees {
		for _, node := range tree.Nodes {
			directDependencies = append(directDependencies, node.Id)
		}
	}
	return directDependencies
}

// Gets xray scan response and returns only the vulnerabilities part of it
func getXrayVulnerabilities(xrayScanResults []services.ScanResponse) []services.Vulnerability {
	xrayVulnerabilities := []services.Vulnerability{}
	for _, result := range xrayScanResults {
		xrayVulnerabilities = append(xrayVulnerabilities, result.Vulnerabilities...)
	}
	return xrayVulnerabilities
}

// Gets xray scan response and returns only the violations part of it
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
	var err error
	defer func() {
		if a.deleteApplicabilityScanProcessFiles() != nil {
			e := a.deleteApplicabilityScanProcessFiles()
			if err == nil {
				err = e
			}
		}
	}()
	if err = a.createConfigFile(); err != nil {
		return true, err
	}
	if entitledForJas, err := a.runAnalyzerManager(); err != nil {
		return entitledForJas, err
	}
	if err = a.parseResults(); err != nil {
		return true, err
	}
	return true, nil
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
	cveWhiteList := utils.RemoveDuplicateValues(createCveList(a.xrayVulnerabilities, a.xrayViolations))
	configFileContent := applicabilityScanConfig{
		Scans: []scanConfiguration{
			{
				Roots:        []string{currentDir},
				Output:       a.resultsFileName,
				Type:         applicabilityScanType,
				GrepDisable:  false,
				CveWhitelist: cveWhiteList,
				SkippedDirs:  skippedDirs,
			},
		},
	}
	yamlData, err := yaml.Marshal(&configFileContent)
	if err != nil {
		return err
	}
	err = os.WriteFile(a.configFileName, yamlData, 0644)
	return err
}

// Runs the analyzerManager app and returns a boolean indicates if the user is entitled for
// advance security feature
func (a *ApplicabilityScanManager) runAnalyzerManager() (bool, error) {
	if err := utils.SetAnalyzerManagerEnvVariables(a.serverDetails); err != nil {
		return true, err
	}

	if err := a.analyzerManager.Exec(a.configFileName); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode := exitError.ExitCode()
			// User not entitled error
			if exitCode == noEntitledExitCode {
				return false, err
			}
		}
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

	xrayCves := utils.RemoveDuplicateValues(createCveList(a.xrayVulnerabilities, a.xrayViolations))
	for _, xrayCve := range xrayCves {
		a.applicabilityScannerResults[xrayCve] = utils.ApplicabilityUndeterminedStringValue
	}

	for _, vulnerability := range fullVulnerabilitiesList {
		applicableVulnerabilityName := getVulnerabilityName(*vulnerability.RuleID)
		if isVulnerabilityApplicable(vulnerability) {
			a.applicabilityScannerResults[applicableVulnerabilityName] = utils.ApplicableStringValue
		} else {
			a.applicabilityScannerResults[applicableVulnerabilityName] = utils.NotApplicableStringValue
		}
	}
	return nil
}

func (a *ApplicabilityScanManager) deleteApplicabilityScanProcessFiles() error {
	exist, err := fileutils.IsFileExists(a.configFileName, false)
	if err != nil {
		return err
	}
	if exist {
		err = os.Remove(a.configFileName)
		if errorutils.CheckError(err) != nil {
			return err
		}
	}
	exist, err = fileutils.IsFileExists(a.resultsFileName, false)
	if err != nil {
		return err
	}
	if exist {
		err = os.Remove(a.resultsFileName)
		if errorutils.CheckError(err) != nil {
			return err
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
