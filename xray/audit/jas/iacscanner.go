package jas

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
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
	fullPathWorkingDirs, err := utils.GetFullPathsWorkingDirs(workingDirs)
	if err != nil {
		return nil, cleanup, err
	}
	return &IacScanManager{
		iacScannerResults: []utils.IacOrSecretResult{},
		configFileName:    filepath.Join(tempDir, "config.yaml"),
		resultsFileName:   filepath.Join(tempDir, "results.sarif"),
		analyzerManager:   analyzerManager,
		serverDetails:     serverDetails,
		workingDirs:       fullPathWorkingDirs,
	}, cleanup, nil
}

func (iac *IacScanManager) run() (err error) {
	for _, workingDir := range iac.workingDirs {
		var currWdResults []utils.IacOrSecretResult
		if currWdResults, err = iac.runIacScan(workingDir); err != nil {
			return
		}
		iac.iacScannerResults = append(iac.iacScannerResults, currWdResults...)
	}
	return
}

func (iac *IacScanManager) runIacScan(workingDir string) (results []utils.IacOrSecretResult, err error) {
	defer func() {
		err = errors.Join(err, deleteJasProcessFiles(iac.configFileName, iac.resultsFileName))
	}()
	if err = iac.createConfigFile(workingDir); err != nil {
		return
	}
	if err = iac.runAnalyzerManager(); err != nil {
		return
	}
	results, err = getIacOrSecretsScanResults(iac.resultsFileName, workingDir, false)
	return
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

func (iac *IacScanManager) createConfigFile(currentWd string) error {
	configFileContent := iacScanConfig{
		Scans: []iacScanConfiguration{
			{
				Roots:       []string{currentWd},
				Output:      iac.resultsFileName,
				Type:        iacScannerType,
				SkippedDirs: skippedDirs,
			},
		},
	}
	return createScannersConfigFile(iac.configFileName, configFileContent)
}

func (iac *IacScanManager) runAnalyzerManager() error {
	return iac.analyzerManager.Exec(iac.configFileName, iacScanCommand, iac.serverDetails)
}
