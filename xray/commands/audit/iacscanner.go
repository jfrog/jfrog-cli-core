package audit

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
)

const (
	iacScannerType        = "iac-scan-modules"
	iacScanFailureMessage = "failed to run iac scan. Cause: %s"
	iacScanCommand        = "iac"
)

type Iac struct {
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
	analyzerManager   utils.AnalyzerManagerInterface
	serverDetails     *config.ServerDetails
	projectRootPath   string
}

func getIacScanResults(serverDetails *config.ServerDetails, analyzerManager utils.AnalyzerManagerInterface) ([]Iac, bool, error) {
	iacScanManager, cleanupFunc, err := newsIacScanManager(serverDetails, analyzerManager)
	if err != nil {
		return nil, false, fmt.Errorf(iacScanFailureMessage, err.Error())
	}
	defer func() {
		if cleanupFunc != nil {
			e := cleanupFunc()
			if err == nil {
				err = e
			}
		}
	}()
	err = iacScanManager.run()
	if err != nil {
		if utils.IsNotEntitledError(err) || utils.IsUnsupportedCommandError(err) {
			return nil, false, nil
		}
		return nil, true, fmt.Errorf(iacScanFailureMessage, err.Error())
	}
	return iacScanManager.iacScannerResults, true, nil
}

func newsIacScanManager(serverDetails *config.ServerDetails, analyzerManager utils.AnalyzerManagerInterface) (manager *IacScanManager,
	cleanup func() error, err error) {
	tempDir, err := fileutils.CreateTempDir()
	if err != nil {
		return
	}
	cleanup = func() error {
		return fileutils.RemoveTempDir(tempDir)
	}
	return &IacScanManager{
		iacScannerResults: []Iac{},
		configFileName:    filepath.Join(tempDir, "config.yaml"),
		resultsFileName:   filepath.Join(tempDir, "results.sarif"),
		analyzerManager:   analyzerManager,
		serverDetails:     serverDetails,
	}, cleanup, nil
}

func (iac *IacScanManager) run() error {
	var err error
	defer func() {
		if deleteJasProcessFiles(iac.configFileName, iac.resultsFileName) != nil {
			e := deleteJasProcessFiles(iac.configFileName, iac.resultsFileName)
			if err == nil {
				err = e
			}
		}
	}()
	if err = iac.createConfigFile(); err != nil {
		return err
	}
	if err = iac.runAnalyzerManager(); err != nil {
		return err
	}
	err = iac.parseResults()
	return err
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
	return err
}

func (iac *IacScanManager) runAnalyzerManager() error {
	if err := utils.SetAnalyzerManagerEnvVariables(iac.serverDetails); err != nil {
		return err
	}
	return iac.analyzerManager.Exec(iac.configFileName, iacScanCommand)
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
			Severity:   utils.GetResultSeverity(result),
			File:       utils.ExtractRelativePath(utils.GetResultFileName(result), iac.projectRootPath),
			LineColumn: utils.GetResultLocationInFile(result),
			Text:       *result.Message.Text,
			Type:       *result.RuleID,
		}
		finalIacList = append(finalIacList, newIac)
	}
	iac.iacScannerResults = finalIacList
	return nil
}
