package jas

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"gopkg.in/yaml.v2"
	"os"
	"path/filepath"
)

const (
	iacScannerType        = "iac-scan-modules"
	iacScanFailureMessage = "failed to run Infrastructure as Code scan. Cause: %s"
	iacScanCommand        = "iac"
)

type IacScanManager struct {
	iacScannerResults []utils.IacOrSecretResult
	configFileName    string
	resultsFileName   string
	analyzerManager   utils.AnalyzerManagerInterface
	serverDetails     *config.ServerDetails
	workingDirs       []string
}

// The getIacScanResults function runs the iac scan flow, which includes the following steps:
// Creating an IacScanManager object.
// Running the analyzer manager executable.
// Parsing the analyzer manager results.
// Return values:
// []utils.IacOrSecretResult: a list of the iac violations that were found.
// bool: true if the user is entitled to iac scan, false otherwise.
// error: An error object (if any).
func getIacScanResults(serverDetails *config.ServerDetails, workingDirs []string, analyzerManager utils.AnalyzerManagerInterface) ([]utils.IacOrSecretResult,
	bool, error) {
	iacScanManager, cleanupFunc, err := newIacScanManager(serverDetails, workingDirs, analyzerManager)
	if err != nil {
		return nil, false, fmt.Errorf(iacScanFailureMessage, err.Error())
	}
	defer func() {
		if cleanupFunc != nil {
			err = errors.Join(err, cleanupFunc())
		}
	}()
	log.Info("Running IaC scanning...")
	if err = iacScanManager.run(); err != nil {
		return nil, false, utils.ParseAnalyzerManagerError(utils.IaC, err)
	}
	if len(iacScanManager.iacScannerResults) > 0 {
		log.Info("Found", len(iacScanManager.iacScannerResults), "IaC vulnerabilities")
	}
	return iacScanManager.iacScannerResults, true, nil
}

func newIacScanManager(serverDetails *config.ServerDetails, workingDirs []string, analyzerManager utils.AnalyzerManagerInterface) (manager *IacScanManager,
	cleanup func() error, err error) {
	tempDir, err := fileutils.CreateTempDir()
	if err != nil {
		return
	}
	cleanup = func() error {
		return fileutils.RemoveTempDir(tempDir)
	}
	return &IacScanManager{
		iacScannerResults: []utils.IacOrSecretResult{},
		configFileName:    filepath.Join(tempDir, "config.yaml"),
		resultsFileName:   filepath.Join(tempDir, "results.sarif"),
		analyzerManager:   analyzerManager,
		serverDetails:     serverDetails,
		workingDirs:       workingDirs,
	}, cleanup, nil
}

func (iac *IacScanManager) run() (err error) {
	defer func() {
		if deleteJasProcessFiles(iac.configFileName, iac.resultsFileName) != nil {
			deleteFilesError := deleteJasProcessFiles(iac.configFileName, iac.resultsFileName)
			err = errors.Join(err, deleteFilesError)
		}
	}()
	if err = iac.createConfigFile(); err != nil {
		return
	}
	if err = iac.runAnalyzerManager(); err != nil {
		return
	}
	return iac.setScanResults()
}

type iacScanConfig struct {
	Scans []iacScanConfiguration `yaml:"scans"`
}

type iacScanConfiguration struct {
	Roots       []string `yaml:"roots"`
	Output      string   `yaml:"output"`
	Type        string   `yaml:"type"`
	SkippedDirs []string `yaml:"skipped-folders"`
}

func (iac *IacScanManager) createConfigFile() error {
	// Currently, IaC supports only one directory scan at a time.
	// If the user didn't specify any working directory, we'll use the current working directory.
	// If the user specified one working directory, we'll use it.
	// However, if the user specified more than one working directory, due to the current limitation, we'll only take the first one.
	fullPathWorkingDirs, err := utils.GetFullPathsWorkingDirs(iac.workingDirs)
	if err != nil {
		return err
	}
	configFileContent := iacScanConfig{
		Scans: []iacScanConfiguration{
			{
				Roots:       []string{fullPathWorkingDirs[0]},
				Output:      iac.resultsFileName,
				Type:        iacScannerType,
				SkippedDirs: skippedDirs,
			},
		},
	}
	yamlData, err := yaml.Marshal(&configFileContent)
	if errorutils.CheckError(err) != nil {
		return err
	}
	err = os.WriteFile(iac.configFileName, yamlData, 0644)
	return errorutils.CheckError(err)
}

func (iac *IacScanManager) runAnalyzerManager() error {
	if err := utils.SetAnalyzerManagerEnvVariables(iac.serverDetails); err != nil {
		return err
	}
	return iac.analyzerManager.Exec(iac.configFileName, iacScanCommand)
}
func (iac *IacScanManager) setScanResults() error {
	report, err := sarif.Open(iac.resultsFileName)
	if errorutils.CheckError(err) != nil {
		return err
	}
	var iacResults []*sarif.Result
	if len(report.Runs) > 0 {
		iacResults = report.Runs[0].Results
	}
	currWd, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}

	var finalIacList []utils.IacOrSecretResult
	for _, result := range iacResults {
		newIac := utils.IacOrSecretResult{
			Severity:   utils.GetResultSeverity(result),
			File:       utils.ExtractRelativePath(utils.GetResultFileName(result), currWd),
			LineColumn: utils.GetResultLocationInFile(result),
			Text:       *result.Message.Text,
			Type:       *result.RuleID,
		}
		finalIacList = append(finalIacList, newIac)
	}
	iac.iacScannerResults = finalIacList
	return nil
}
