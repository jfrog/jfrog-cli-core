package jas

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
)

const (
	iacScanCommand = "iac"
	iacScannerType = "iac-scan-modules"
)

type Iac struct { // TODO
	Severity   string
	File       string
	LineColumn string
	Type       string
	Text       string
}

type IacScanManager struct {
	iacScannerResults []Iac
	configFileName    string
	resultsFileName   string
	analyzerManager   AnalyzerManager
	serverDetails     *config.ServerDetails
	projectRootPath   string
}

func getIacScanResults(serverDetails *config.ServerDetails, analyzerManager AnalyzerManager) ([]Iac, bool, error) {
	iacScanManager, err := NewsIacScanManager(serverDetails, analyzerManager)
	if err != nil {
		log.Info("failed to run iac scan: " + err.Error())
		return nil, false, err
	}
	err = iacScanManager.Run()
	if err != nil {
		if isNotEntitledError(err) || isUnsupportedCommandError(err) {
			return nil, false, nil
		}
		log.Info("failed to run iac scan: " + err.Error())
		return nil, true, err
	}
	return iacScanManager.iacScannerResults, true, nil
}

func NewsIacScanManager(serverDetails *config.ServerDetails, analyzerManager AnalyzerManager) (*IacScanManager, error) {
	if serverDetails == nil {
		return nil, errors.New("cant get xray server details")
	}
	tempDir, err := fileutils.CreateTempDir()
	if err != nil {
		return nil, err
	}
	return &IacScanManager{
		iacScannerResults: []Iac{},
		configFileName:    filepath.Join(tempDir, "config.yaml"),
		resultsFileName:   filepath.Join(tempDir, "results.sarif"),
		analyzerManager:   analyzerManager,
		serverDetails:     serverDetails,
	}, nil
}

func (iac *IacScanManager) Run() error {
	defer deleteJasScanProcessFiles(iac.configFileName, iac.resultsFileName)
	if err := iac.createConfigFile(); err != nil {
		return err
	}
	if err := iac.runAnalyzerManager(); err != nil {
		return err
	}
	if err := iac.parseResults(); err != nil {
		return err
	}
	return nil
}

type iacScanConfig struct {
	Scans []iacScanConfiguration `yaml:"scans"`
}

type iacScanConfiguration struct {
	Roots  []string `yaml:"roots"`
	Output string   `yaml:"output"`
	Type   string   `yaml:"type"`
}

func (iac *IacScanManager) createConfigFile() error {
	currentDir, err := coreutils.GetWorkingDirectory()
	iac.projectRootPath = currentDir
	if err != nil {
		return err
	}
	configFileContent := iacScanConfig{
		Scans: []iacScanConfiguration{
			{
				Roots:  []string{currentDir},
				Output: filepath.Join(currentDir, iac.resultsFileName),
				Type:   iacScannerType,
			},
		},
	}
	yamlData, err := yaml.Marshal(&configFileContent)
	if err != nil {
		return err
	}
	err = os.WriteFile(iac.configFileName, yamlData, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (iac *IacScanManager) runAnalyzerManager() error {
	err := setAnalyzerManagerEnvVariables(iac.serverDetails)
	if err != nil {
		return err
	}
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}
	err = iac.analyzerManager.RunAnalyzerManager(filepath.Join(currentDir, iac.configFileName), iacScanCommand)
	return err
}

func (iac *IacScanManager) parseResults() error {
	report, err := sarif.Open(iac.resultsFileName)
	if err != nil {
		return err
	}
	var iacResults []*sarif.Result
	if len(report.Runs) > 0 {
		iacResults = report.Runs[0].Results
	}

	finalIacList := []Iac{}

	for _, result := range iacResults {
		newIac := Iac{
			Severity:   getResultSeverity(result),
			File:       extractRelativePath(getResultFileName(result), iac.projectRootPath),
			LineColumn: getResultLocationInFile(result),
			Text:       *result.Locations[0].PhysicalLocation.Region.Snippet.Text,
			Type:       *result.RuleID,
		}
		finalIacList = append(finalIacList, newIac)
	}
	iac.iacScannerResults = finalIacList
	return nil
}
