package jas

import (
	"errors"
	rtutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	SkippedDirs = []string{"**/*test*/**", "**/*venv*/**", "**/*node_modules*/**", "**/*target*/**"}
)

type JasScanner struct {
	ConfigFileName        string
	ResultsFileName       string
	AnalyzerManager       utils.AnalyzerManager
	ServerDetails         *config.ServerDetails
	WorkingDirs           []string
	ScannerDirCleanupFunc func() error
}

func NewJasScanner(workingDirs []string, serverDetails *config.ServerDetails) (scanner *JasScanner, err error) {
	scanner = &JasScanner{}
	if scanner.AnalyzerManager.AnalyzerManagerFullPath, err = utils.GetAnalyzerManagerExecutable(); err != nil {
		return
	}
	var tempDir string
	if tempDir, err = fileutils.CreateTempDir(); err != nil {
		return
	}
	scanner.ScannerDirCleanupFunc = func() error {
		return fileutils.RemoveTempDir(tempDir)
	}
	scanner.ServerDetails = serverDetails
	scanner.ConfigFileName = filepath.Join(tempDir, "config.yaml")
	scanner.ResultsFileName = filepath.Join(tempDir, "results.sarif")
	scanner.WorkingDirs, err = coreutils.GetFullPathsWorkingDirs(workingDirs)
	return
}

type ScannerCmd interface {
	Run(wd string) (err error)
}

func (a *JasScanner) Run(scannerCmd ScannerCmd) (err error) {
	for _, workingDir := range a.WorkingDirs {
		func() {
			defer func() {
				err = errors.Join(err, deleteJasProcessFiles(a.ConfigFileName, a.ResultsFileName))
			}()
			if err = scannerCmd.Run(workingDir); err != nil {
				return
			}
		}()
	}
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

func GetSourceCodeScanResults(resultsFileName, workingDir string, scanType utils.JasScanType) (results []utils.SourceCodeScanResult, err error) {
	// Read Sarif format results generated from the Jas scanner
	report, err := sarif.Open(resultsFileName)
	if errorutils.CheckError(err) != nil {
		return nil, err
	}
	var sarifResults []*sarif.Result
	if len(report.Runs) > 0 {
		// Jas scanners returns results in a single run entry
		sarifResults = report.Runs[0].Results
	}
	resultPointers := convertSarifResultsToSourceCodeScanResults(sarifResults, workingDir, scanType)
	for _, res := range resultPointers {
		results = append(results, *res)
	}
	return results, nil
}

func convertSarifResultsToSourceCodeScanResults(sarifResults []*sarif.Result, workingDir string, scanType utils.JasScanType) []*utils.SourceCodeScanResult {
	var sourceCodeScanResults []*utils.SourceCodeScanResult
	for _, sarifResult := range sarifResults {
		// Describes a request to “suppress” a result (to exclude it from result lists)
		if len(sarifResult.Suppressions) > 0 {
			continue
		}
		// Convert
		currentResult := utils.GetResultIfExists(sarifResult, workingDir, sourceCodeScanResults)
		if currentResult == nil {
			currentResult = utils.ConvertSarifResultToSourceCodeScanResult(sarifResult, workingDir)
			// Set specific Jas scan attributes
			if scanType == utils.Secrets {
				currentResult.Text = hideSecret(utils.GetResultLocationSnippet(sarifResult.Locations[0]))
			}
			sourceCodeScanResults = append(sourceCodeScanResults, currentResult)
		}
		if scanType == utils.Sast {
			currentResult.CodeFlow = append(currentResult.CodeFlow, utils.GetResultCodeFlows(sarifResult, workingDir)...)
		}
	}
	return sourceCodeScanResults
}

func CreateScannersConfigFile(fileName string, fileContent interface{}) error {
	yamlData, err := yaml.Marshal(&fileContent)
	if errorutils.CheckError(err) != nil {
		return err
	}
	err = os.WriteFile(fileName, yamlData, 0644)
	return errorutils.CheckError(err)
}

func hideSecret(secret string) string {
	if len(secret) <= 3 {
		return "***"
	}
	return secret[:3] + strings.Repeat("*", 12)
}

var FakeServerDetails = config.ServerDetails{
	Url:      "platformUrl",
	Password: "password",
	User:     "user",
}

var FakeBasicXrayResults = []services.ScanResponse{
	{
		ScanId: "scanId_1",
		Vulnerabilities: []services.Vulnerability{
			{IssueId: "issueId_1", Technology: coreutils.Pipenv.String(),
				Cves:       []services.Cve{{Id: "testCve1"}, {Id: "testCve2"}, {Id: "testCve3"}},
				Components: map[string]services.Component{"issueId_1_direct_dependency": {}, "issueId_3_direct_dependency": {}}},
		},
		Violations: []services.Violation{
			{IssueId: "issueId_2", Technology: coreutils.Pipenv.String(),
				Cves:       []services.Cve{{Id: "testCve4"}, {Id: "testCve5"}},
				Components: map[string]services.Component{"issueId_2_direct_dependency": {}, "issueId_4_direct_dependency": {}}},
		},
	},
}

func InitJasTest(t *testing.T, workingDirs ...string) (*JasScanner, func()) {
	assert.NoError(t, rtutils.DownloadAnalyzerManagerIfNeeded())
	scanner, err := NewJasScanner(workingDirs, &FakeServerDetails)
	assert.NoError(t, err)
	return scanner, func() {
		assert.NoError(t, scanner.ScannerDirCleanupFunc())
	}
}

func GetTestDataPath() string {
	return filepath.Join("..", "..", "..", "testdata")
}
