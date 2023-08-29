package jas

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"gopkg.in/yaml.v3"
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

func RunScannersAndSetResults(scanResults *utils.ExtendedScanResults, directDependencies []string,
	serverDetails *config.ServerDetails, workingDirs []string, progress io.ProgressMgr) (err error) {
	if serverDetails == nil || len(serverDetails.Url) == 0 {
		log.Warn("To include 'Advanced Security' scan as part of the audit output, please run the 'jf c add' command before running this command.")
		return
	}
	scanner, err := NewAdvancedSecurityScanner(workingDirs, serverDetails)
	if err != nil {
		return
	}
	defer func() {
		cleanup := scanner.scannerDirCleanupFunc
		err = errors.Join(err, cleanup())
	}()
	if progress != nil {
		progress.SetHeadlineMsg("Running applicability scanning")
	}
	scanResults.ApplicabilityScanResults, err = getApplicabilityScanResults(scanResults.XrayResults, directDependencies, scanResults.ScannedTechnologies, scanner)
	if err != nil {
		return
	}
	if progress != nil {
		progress.SetHeadlineMsg("Running secrets scanning")
	}
	scanResults.SecretsScanResults, err = getSecretsScanResults(scanner)
	if err != nil {
		return
	}
	if progress != nil {
		progress.SetHeadlineMsg("Running IaC scanning")
	}
	scanResults.IacScanResults, err = getIacScanResults(scanner)
	if err != nil {
		return
	}
	if progress != nil {
		progress.SetHeadlineMsg("Running SAST scanning")
	}
	scanResults.SastResults, err = getSastScanResults(scanner)
	return
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

func getSourceCodeScanResults(resultsFileName, workingDir string, scanType utils.ScanType) ([]utils.SourceCodeScanResult, error) {
	report, err := sarif.Open(resultsFileName)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}
	var results []*sarif.Result
	if len(report.Runs) > 0 {
		results = report.Runs[0].Results
	}

	var sourceCodeScanResults []utils.SourceCodeScanResult
	for _, result := range results {
		// Describes a request to “suppress” a result (to exclude it from result lists)
		if len(result.Suppressions) > 0 {
			continue
		}
		index := utils.GetOrCreateCodeScanResult(result, workingDir, &sourceCodeScanResults)
		if scanType == utils.Secrets {
			sourceCodeScanResults[index].Text = hideSecret(utils.GetResultLocationSnippet(result.Locations[0]))
		}
		if scanType == utils.Sast {
			flows := utils.GetResultCodeFlows(result, workingDir)
			sourceCodeScanResults[index].CodeFlow = append(sourceCodeScanResults[index].CodeFlow, flows...)
		}
	}
	return sourceCodeScanResults, nil
}

func createScannersConfigFile(fileName string, fileContent interface{}) error {
	yamlData, err := yaml.Marshal(&fileContent)
	if errorutils.CheckError(err) != nil {
		return err
	}
	err = os.WriteFile(fileName, yamlData, 0644)
	return errorutils.CheckError(err)
}
