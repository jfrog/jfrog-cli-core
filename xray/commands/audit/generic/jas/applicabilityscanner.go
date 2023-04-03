package jas

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/sarif"
	"gopkg.in/yaml.v2"
	"os"
	"os/exec"
	"strings"
)

const (
	applicabilityScanCommand = "ca"
	applicabilityScanType    = "analyze-applicability"
)

//var eligibleTechnologiesForApplicabilityScan = []coreutils.Technology{
//	coreutils.Npm, coreutils.Pip, coreutils.Poetry, coreutils.Pipenv }

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

type ApplicabilityScanManager struct {
	applicableVulnerabilities []string
	xrayVulnerabilities       []services.Vulnerability
	configFileName            string
	resultsFileName           string
}

func NewApplicabilityScanManager(xrayScanResults []services.ScanResponse) *ApplicabilityScanManager {
	xrayVulnerabilities := getXrayVulnerabilities(xrayScanResults)
	return &ApplicabilityScanManager{
		applicableVulnerabilities: []string{},
		xrayVulnerabilities:       xrayVulnerabilities,
		configFileName:            generateRandomFileName() + ".yaml",
		resultsFileName:           generateRandomFileName() + ".sarif",
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

//func (a *ApplicabilityScanManager) ShouldRunApplicabilityScan() bool {
//	if isTechEligibleForJas(a.tech, eligibleTechnologiesForApplicabilityScan) {
//		return true
//	}
//	return false
//}

func (a *ApplicabilityScanManager) Run() error {
	if err := isAnalyzerManagerExecutableExist(); err != nil {
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

func (a *ApplicabilityScanManager) createConfigFile() error {
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}
	configFileContent := applicabilityScanConfig{
		Scans: []scanConfiguration{
			{
				Roots:          []string{currentDir},
				Output:         a.resultsFileName,
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
	_, err := exec.Command(analyzerManagerFilePath, applicabilityScanCommand, a.configFileName).Output()
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
		cves := utils.ConvertCves(vulnerability.Cves)
		for _, cve := range cves {
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
