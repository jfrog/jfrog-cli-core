package jas

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
)

var (
	SkippedDirs = []string{"**/*test*/**", "**/*venv*/**", "**/*node_modules*/**", "**/*target*/**"}

	mapSeverityToScore = map[string]string{
		"":         "0.0",
		"unknown":  "0.0",
		"low":      "3.9",
		"medium":   "6.9",
		"high":     "8.9",
		"critical": "10",
	}
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

func ReadJasScanRunsFromFile(fileName, wd string) (sarifRuns []*sarif.Run, err error) {
	if sarifRuns, err = utils.ReadScanRunsFromFile(fileName); err != nil {
		return
	}
	for i := 0; i < len(sarifRuns); i++ {
		sarifRuns[i] = processJasScanRun(sarifRuns[i], wd)
	}
	return
}

func processJasScanRun(sarifRun *sarif.Run, workingDir string) *sarif.Run {
	processed := sarif.NewRun(sarifRun.Tool)
	// Jas reports has only one invocation
	invocation := sarifRun.Invocations[0]
	// Set the actual working directory to the invocation, not the analyzerManager directory
	// Also used to calculate relative paths if needed with it
	invocation.WorkingDirectory.WithUri(workingDir)
	processed.Invocations = append(processed.Invocations, invocation)
	// Process results
	for _, sarifResult := range sarifRun.Results {
		if len(sarifResult.Suppressions) > 0 {
			// Describes a request to “suppress” a result (to exclude it from result lists)
			continue
		}
		processed.Results = append(processed.Results, sarifResult)
		if rule, err := sarifRun.GetRuleById(*sarifResult.RuleID); err == nil {
			// Add to the rule security-severity score base on results severity
			score := convertToScore(utils.GetResultSeverity(sarifResult))
			if score != utils.MissingCveScore && rule.Properties == nil {
				properties := sarif.NewPropertyBag()
				properties.Add("security-severity", score)
				rule.WithProperties(properties.Properties)
			}
		}
	}
	return processed
}

func convertToScore(severity string) string {
	if level, ok := mapSeverityToScore[strings.ToLower(severity)]; ok {
		return level
	}
	return ""
}

func GetJasMarkdownDescription(scanType utils.JasScanType, location *sarif.Location, severity, content string) string {
	dataColumnHeader := "Finding"
	if scanType == utils.Secrets {
		dataColumnHeader = "Secret"
	}
	headerRow := fmt.Sprintf("| Severity | File | Line:Column | %s |\n", dataColumnHeader)
	separatorRow := "| :---: | :---: | :---: | :---: |\n"
	tableHeader := headerRow + separatorRow
	return tableHeader + fmt.Sprintf("| %s | %s | %s | %s |", severity, utils.GetLocationFileName(location), utils.GetStartLocationInFile(location), content)
}

func CreateScannersConfigFile(fileName string, fileContent interface{}) error {
	yamlData, err := yaml.Marshal(&fileContent)
	if errorutils.CheckError(err) != nil {
		return err
	}
	err = os.WriteFile(fileName, yamlData, 0644)
	return errorutils.CheckError(err)
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
			{IssueId: "issueId_1", Technology: coreutils.Pipenv.ToString(),
				Cves:       []services.Cve{{Id: "testCve1"}, {Id: "testCve2"}, {Id: "testCve3"}},
				Components: map[string]services.Component{"issueId_1_direct_dependency": {}, "issueId_3_direct_dependency": {}}},
		},
		Violations: []services.Violation{
			{IssueId: "issueId_2", Technology: coreutils.Pipenv.ToString(),
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
