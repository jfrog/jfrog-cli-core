package jas

import (
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/sarif"
	"google.golang.org/genproto/googleapis/devtools/containeranalysis/v1beta1/vulnerability"
	"gopkg.in/yaml.v2"
	"os"
	"os/exec"
	"strings"
)

const (
	applicabilityScanCommand = "ca"
	applicabilityScanType    = "analyze-applicability"
)

var eligibleTechnologiesForApplicabilityScan = []string{"npm", "python"}

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

type applicabilityScanManager struct {
	tech                      string
	applicableVulnerabilities []string

	configFileName  string
	resultsFileName string
}

func NewApplicabilityScanManager(tech string, xrayVulnerabilities []services.Vulnerability) *applicabilityScanManager {
	return &applicabilityScanManager{
		tech:                      tech,
		applicableVulnerabilities: []string{},
		configFileName:            GenerateRandomFileName() + ".yaml",
		resultsFileName:           GenerateRandomFileName() + ".sarif",
	}
}

func (a *applicabilityScanManager) Run() error {
	if !IsTechEligibleForJas(a.tech, eligibleTechnologiesForApplicabilityScan) {
		return nil
	}
	if err := IsAnalyzerManagerExecutableExist(); err != nil {
		return err
	}
	err := a.createConfigFile()
	if err != nil {
		return err
	}
	return nil
}

func (a *applicabilityScanManager) createConfigFile() error {
	configFileContent := applicabilityScanConfig{
		Scans: []scanConfiguration{
			{
				Roots:          []string{GetScanRootFolder()},
				Output:         a.resultsFileName,
				Type:           applicabilityScanType,
				GrepDisable:    false,
				CveWhitelist:   GetXrayVulnerabilities(),
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

func (a *applicabilityScanManager) runAnalyzerManager() error {
	_, err := exec.Command(AnalyzerManagerFilePath, applicabilityScanCommand, a.configFileName).Output()
	if err != nil {
		return err
	}
	return nil
}

func (a *applicabilityScanManager) parseResults() error {
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

func (a *applicabilityScanManager) addApplicabilityResultsToXrayResults() error {
	return nil
}

func (a *applicabilityScanManager) DeleteApplicabilityScanProcessFiles() error {
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

func isVulnerabilityApplicable(vulnerability *sarif.Result) bool {
	return !(vulnerability.Kind != nil && *vulnerability.Kind == "pass")
}

func getVulnerabilityName(sarifRuleId string) string {
	return strings.Split(sarifRuleId, "_")[1]
}
