package jas

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

var (
	skippedDirs = []string{"**/*test*/**", "**/*venv*/**", "**/*node_modules*/**", "**/*target*/**"}
)

type ScannerCmd interface {
	Run(wd string) (err error)
}

type AdvancedSecurityScanner struct {
	configFileName        string
	resultsFileName       string
	analyzerManager       utils.AnalyzerManager
	serverDetails         *config.ServerDetails
	workingDirs           []string
	scannerDirCleanupFunc func() error
}

func NewAdvancedSecurityScanner(workingDirs []string, serverDetails *config.ServerDetails) (scanner *AdvancedSecurityScanner, err error) {
	scanner = &AdvancedSecurityScanner{}
	if scanner.analyzerManager.AnalyzerManagerFullPath, err = utils.GetAnalyzerManagerExecutable(); err != nil {
		err = errors.New("unable to locate the analyzer manager package. Advanced security scans cannot be performed without this package")
		return
	}
	var tempDir string
	if tempDir, err = fileutils.CreateTempDir(); err != nil {
		return
	}
	scanner.scannerDirCleanupFunc = func() error {
		return fileutils.RemoveTempDir(tempDir)
	}
	scanner.serverDetails = serverDetails
	scanner.configFileName = filepath.Join(tempDir, "config.yaml")
	scanner.resultsFileName = filepath.Join(tempDir, "results.sarif")
	scanner.workingDirs, err = utils.GetFullPathsWorkingDirs(workingDirs)
	return
}

func (a *AdvancedSecurityScanner) Run(scannerCmd ScannerCmd) (err error) {
	for _, workingDir := range a.workingDirs {
		func() {
			defer func() {
				err = errors.Join(err, deleteJasProcessFiles(a.configFileName, a.resultsFileName))
			}()
			if err = scannerCmd.Run(workingDir); err != nil {
				return
			}
		}()
	}
	return
}

func GetExtendedScanResults(xrayResults []services.ScanResponse, dependencyTrees []*xrayUtils.GraphNode,
	serverDetails *config.ServerDetails, scannedTechnologies []coreutils.Technology, workingDirs []string) (*utils.ExtendedScanResults, error) {
	if serverDetails == nil || len(serverDetails.Url) == 0 {
		log.Warn("To include 'Advanced Security' scan as part of the audit output, please run the 'jf c add' command before running this command.")
		return &utils.ExtendedScanResults{XrayResults: xrayResults}, nil
	}
	scanner, err := NewAdvancedSecurityScanner(workingDirs, serverDetails)
	if err != nil {
		return nil, err
	}
	defer func() {
		cleanup := scanner.scannerDirCleanupFunc
		if cleanup != nil {
			err = errors.Join(err, cleanup())
		}
	}()
	applicabilityScanResults, err := getApplicabilityScanResults(
		xrayResults, dependencyTrees, scannedTechnologies, scanner)
	if err != nil {
		return nil, err
	}
	secretsScanResults, err := getSecretsScanResults(scanner)
	if err != nil {
		return nil, err
	}
	iacScanResults, err := getIacScanResults(scanner)
	if err != nil {
		return nil, err
	}
	return &utils.ExtendedScanResults{
		EntitledForJas:           true,
		XrayResults:              xrayResults,
		ScannedTechnologies:      scannedTechnologies,
		ApplicabilityScanResults: applicabilityScanResults,
		SecretsScanResults:       secretsScanResults,
		IacScanResults:           iacScanResults,
	}, nil
}

func deleteJasProcessFiles(configFile string, resultFile string) error {
	exist, err := fileutils.IsFileExists(configFile, false)
	if err != nil {
		return err
	}
	if exist {
		if err = os.Remove(configFile); err != nil {
			return errorutils.CheckError(err)
		}
	}
	exist, err = fileutils.IsFileExists(resultFile, false)
	if err != nil {
		return err
	}
	if exist {
		err = os.Remove(resultFile)
	}
	return errorutils.CheckError(err)
}

func getIacOrSecretsScanResults(resultsFileName string, isSecret bool) ([]utils.IacOrSecretResult, error) {
	report, err := sarif.Open(resultsFileName)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}
	var results []*sarif.Result
	if len(report.Runs) > 0 {
		results = report.Runs[0].Results

	}
	currWd, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return nil, err
	}

	var iacOrSecretResults []utils.IacOrSecretResult
	for _, result := range results {
		text := *result.Message.Text
		if isSecret {
			text = hideSecret(*result.Locations[0].PhysicalLocation.Region.Snippet.Text)
		}
		newResult := utils.IacOrSecretResult{
			Severity:   utils.GetResultSeverity(result),
			File:       utils.ExtractRelativePath(utils.GetResultFileName(result), currWd),
			LineColumn: utils.GetResultLocationInFile(result),
			Text:       text,
			Type:       *result.RuleID,
		}
		iacOrSecretResults = append(iacOrSecretResults, newResult)
	}
	return iacOrSecretResults, nil
}

func createScannersConfigFile(fileName string, fileContent interface{}) error {
	yamlData, err := yaml.Marshal(&fileContent)
	if errorutils.CheckError(err) != nil {
		return err
	}
	err = os.WriteFile(fileName, yamlData, 0644)
	return errorutils.CheckError(err)
}
