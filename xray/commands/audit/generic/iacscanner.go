package audit

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/generic/jas"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
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
	analyzerManager   utils.AnalyzerManagerInterface
	serverDetails     *config.ServerDetails
	projectRootPath   string
}

func GetIacScanResults(serverDetails *config.ServerDetails, analyzerManager utils.AnalyzerManagerInterface) ([]Iac, bool, error) {
	iacScanManager, err := NewsIacScanManager(serverDetails, analyzerManager)
	if err != nil {
		log.Info("failed to run iac scan: " + err.Error())
		return nil, false, err
	}
	err = iacScanManager.Run()
	if err != nil {
		if utils.IsNotEntitledError(err) || utils.IsUnsupportedCommandError(err) {
			return nil, false, nil
		}
		log.Info("failed to run iac scan: " + err.Error())
		return nil, true, err
	}
	return iacScanManager.iacScannerResults, true, nil
}

func NewsIacScanManager(serverDetails *config.ServerDetails, analyzerManager utils.AnalyzerManagerInterface) (*IacScanManager, error) {
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
	defer utils.DeleteJasScanProcessFiles(iac.configFileName, iac.resultsFileName)
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
	err := utils.SetAnalyzerManagerEnvVariables(iac.serverDetails)
	if err != nil {
		return err
	}
	err = iac.analyzerManager.Exec(iac.configFileName)
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
			Severity:   jas.getResultSeverity(result),
			File:       jas.extractRelativePath(jas.getResultFileName(result), iac.projectRootPath),
			LineColumn: jas.getResultLocationInFile(result),
			Text:       *result.Message.Text,
			Type:       *result.RuleID,
		}
		finalIacList = append(finalIacList, newIac)
	}
	iac.iacScannerResults = finalIacList
	return nil
}
