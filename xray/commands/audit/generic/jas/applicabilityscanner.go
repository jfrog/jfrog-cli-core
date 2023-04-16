package jas

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/sarif"
	"gopkg.in/yaml.v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	applicabilityScanCommand = "ca"
	applicabilityScanType    = "analyze-applicability"
)

var (
	analyzerManagerExecuter AnalyzerManager = &analyzerManager{}
)

type ExtendedScanResults struct {
	XrayResults    []services.ScanResponse
	ApplicableCves []string
}

func (e *ExtendedScanResults) GetXrayScanResults() []services.ScanResponse {
	return e.XrayResults
}

func GetExtendedScanResults(results []services.ScanResponse) (*ExtendedScanResults, error) {
	applicabilityScanManager := NewApplicabilityScanManager(results)
	err := applicabilityScanManager.Run()
	if err != nil {
		log.Info("failed to run applicability scan: " + err.Error())
		deleteFilesError := applicabilityScanManager.DeleteApplicabilityScanProcessFiles()
		if deleteFilesError != nil {
			return nil, deleteFilesError
		}
		extendedScanResults := ExtendedScanResults{XrayResults: results, ApplicableCves: nil}
		return &extendedScanResults, nil
	}
	applicabilityScanResults := applicabilityScanManager.GetApplicableVulnerabilities()
	extendedScanResults := ExtendedScanResults{XrayResults: results, ApplicableCves: applicabilityScanResults}
	return &extendedScanResults, nil
}

type ApplicabilityScanManager struct {
	applicableVulnerabilities []string
	xrayVulnerabilities       []services.Vulnerability
	configFileName            string
	resultsFileName           string
	analyzerManager           AnalyzerManager
}

func NewApplicabilityScanManager(xrayScanResults []services.ScanResponse) *ApplicabilityScanManager {
	xrayVulnerabilities := getXrayVulnerabilities(xrayScanResults)
	return &ApplicabilityScanManager{
		applicableVulnerabilities: []string{},
		xrayVulnerabilities:       xrayVulnerabilities,
		configFileName:            generateRandomFileName() + ".yaml",
		resultsFileName:           "sarif.sarif", //generateRandomFileName() + ".sarif",
		analyzerManager:           analyzerManagerExecuter,
	}
}

func getXrayVulnerabilities(xrayScanResults []services.ScanResponse) []services.Vulnerability {
	xrayVulnerabilities := []services.Vulnerability{}
	for _, result := range xrayScanResults {
		for _, vul := range result.Vulnerabilities {
			xrayVulnerabilities = append(xrayVulnerabilities, vul)
		}
	}
	return xrayVulnerabilities
}

func (a *ApplicabilityScanManager) GetApplicableVulnerabilities() []string {
	return a.applicableVulnerabilities
}

func (a *ApplicabilityScanManager) Run() error {
	if err := a.analyzerManager.DoesAnalyzerManagerExecutableExist(); err != nil {
		return err
	}
	if err := a.createConfigFile(); err != nil {
		return err
	}
	if err := a.runAnalyzerManager(); err != nil {
		return err
	}
	if err := a.parseResults(); err != nil {
		return err
	}
	if err := a.DeleteApplicabilityScanProcessFiles(); err != nil {
		return err
	}
	return nil
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
	configFileContent := applicabilityScanConfig{
		Scans: []scanConfiguration{
			{
				Roots:          []string{currentDir},
				Output:         filepath.Join(currentDir, a.resultsFileName),
				Type:           applicabilityScanType,
				GrepDisable:    false,
				CveWhitelist:   a.createCveWhiteList(),
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

func (a *ApplicabilityScanManager) runAnalyzerManager() error {
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}
	err = a.analyzerManager.RunAnalyzerManager(filepath.Join(currentDir, a.configFileName))
	if err != nil {
		return err
	}
	return nil
}

func (a *ApplicabilityScanManager) parseResults() error {
	report, err := sarif.Open(a.resultsFileName)
	if err != nil {
		return err
	}
	fullVulnerabilitiesList := report.Runs[0].Results
	for _, vulnerability := range fullVulnerabilitiesList {
		if isVulnerabilityApplicable(vulnerability) {
			applicableVulnerabilityName := getVulnerabilityName(*vulnerability.RuleID)
			a.applicableVulnerabilities = append(a.applicableVulnerabilities, applicableVulnerabilityName)
		}
	}
	return nil
}

func (a *ApplicabilityScanManager) DeleteApplicabilityScanProcessFiles() error {
	err := os.Remove(a.configFileName)
	if err != nil {
		return err
	}
	err = os.Remove(a.resultsFileName)
	if err != nil {
		return err
	}
	return nil
}

func (a *ApplicabilityScanManager) createCveWhiteList() []string {
	cveWhiteList := []string{}
	for _, vulnerability := range a.xrayVulnerabilities {
		for _, cve := range vulnerability.Cves {
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
	DoesAnalyzerManagerExecutableExist() error
	RunAnalyzerManager(string) error
}

type analyzerManager struct {
}

func (am *analyzerManager) DoesAnalyzerManagerExecutableExist() error {
	if _, err := os.Stat(analyzerManagerFilePath); err != nil {
		return err
	}
	return nil
}

func (am *analyzerManager) RunAnalyzerManager(configFile string) error {
	_, err := exec.Command(analyzerManagerFilePath, applicabilityScanCommand, configFile).Output()
	if err != nil {
		return err
	}
	return nil
}
