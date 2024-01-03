package jas

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	jfrogappsconfig "github.com/jfrog/jfrog-apps-config/go"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/dependencies"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

const (
	NodeModulesPattern = "**/*node_modules*/**"
)

var (
	DefaultExcludePatterns = []string{"**/.git/**", "**/*test*/**", "**/*venv*/**", NodeModulesPattern, "**/target/**"}

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
	JFrogAppsConfig       *jfrogappsconfig.JFrogAppsConfig
	ScannerDirCleanupFunc func() error
}

func NewJasScanner(workingDirs []string, serverDetails *config.ServerDetails, multiScanId string) (scanner *JasScanner, err error) {
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
	scanner.JFrogAppsConfig, err = createJFrogAppsConfig(workingDirs)
	scanner.AnalyzerManager.MultiScanId = multiScanId
	return
}

func createJFrogAppsConfig(workingDirs []string) (*jfrogappsconfig.JFrogAppsConfig, error) {
	if jfrogAppsConfig, err := jfrogappsconfig.LoadConfigIfExist(); err != nil {
		return nil, errorutils.CheckError(err)
	} else if jfrogAppsConfig != nil {
		// jfrog-apps-config.yml exist in the workspace
		return jfrogAppsConfig, nil
	}

	// jfrog-apps-config.yml does not exist in the workspace
	fullPathsWorkingDirs, err := coreutils.GetFullPathsWorkingDirs(workingDirs)
	if err != nil {
		return nil, err
	}
	jfrogAppsConfig := new(jfrogappsconfig.JFrogAppsConfig)
	for _, workingDir := range fullPathsWorkingDirs {
		jfrogAppsConfig.Modules = append(jfrogAppsConfig.Modules, jfrogappsconfig.Module{SourceRoot: workingDir})
	}
	return jfrogAppsConfig, nil
}

type ScannerCmd interface {
	Run(module jfrogappsconfig.Module) (err error)
}

func (a *JasScanner) Run(scannerCmd ScannerCmd) (err error) {
	for _, module := range a.JFrogAppsConfig.Modules {
		func() {
			defer func() {
				err = errors.Join(err, deleteJasProcessFiles(a.ConfigFileName, a.ResultsFileName))
			}()
			if err = scannerCmd.Run(module); err != nil {
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

func ReadJasScanRunsFromFile(fileName, wd, informationUrlSuffix string) (sarifRuns []*sarif.Run, err error) {
	if sarifRuns, err = utils.ReadScanRunsFromFile(fileName); err != nil {
		return
	}
	for _, sarifRun := range sarifRuns {
		// Jas reports has only one invocation
		// Set the actual working directory to the invocation, not the analyzerManager directory
		// Also used to calculate relative paths if needed with it
		sarifRun.Invocations[0].WorkingDirectory.WithUri(wd)
		// Process runs values
		fillMissingRequiredDriverInformation(utils.BaseDocumentationURL+informationUrlSuffix, utils.GetAnalyzerManagerVersion(), sarifRun)
		sarifRun.Results = excludeSuppressResults(sarifRun.Results)
		addScoreToRunRules(sarifRun)
	}
	return
}

func fillMissingRequiredDriverInformation(defaultJasInformationUri, defaultVersion string, run *sarif.Run) {
	driver := run.Tool.Driver
	if driver.InformationURI == nil {
		driver.InformationURI = &defaultJasInformationUri
	}
	if driver.Version == nil || !isValidVersion(*driver.Version) {
		driver.Version = &defaultVersion
	}
}

func isValidVersion(version string) bool {
	if len(version) == 0 {
		return false
	}
	firstChar := rune(version[0])
	return unicode.IsDigit(firstChar)
}

func excludeSuppressResults(sarifResults []*sarif.Result) []*sarif.Result {
	results := []*sarif.Result{}
	for _, sarifResult := range sarifResults {
		if len(sarifResult.Suppressions) > 0 {
			// Describes a request to “suppress” a result (to exclude it from result lists)
			continue
		}
		results = append(results, sarifResult)
	}
	return results
}

func addScoreToRunRules(sarifRun *sarif.Run) {
	for _, sarifResult := range sarifRun.Results {
		if rule, err := sarifRun.GetRuleById(*sarifResult.RuleID); err == nil {
			// Add to the rule security-severity score based on results severity
			score := convertToScore(utils.GetResultSeverity(sarifResult))
			if score != utils.MissingCveScore {
				if rule.Properties == nil {
					rule.WithProperties(sarif.NewPropertyBag().Properties)
				}
				rule.Properties["security-severity"] = score
			}
		}
	}
}

func convertToScore(severity string) string {
	if level, ok := mapSeverityToScore[strings.ToLower(severity)]; ok {
		return level
	}
	return ""
}

func CreateScannersConfigFile(fileName string, fileContent interface{}, scanType utils.JasScanType) error {
	yamlData, err := yaml.Marshal(&fileContent)
	if errorutils.CheckError(err) != nil {
		return err
	}
	log.Debug(scanType.String() + " scanner input YAML:\n" + string(yamlData))
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
	assert.NoError(t, dependencies.DownloadAnalyzerManagerIfNeeded())
	scanner, err := NewJasScanner(workingDirs, &FakeServerDetails, "")
	assert.NoError(t, err)
	return scanner, func() {
		assert.NoError(t, scanner.ScannerDirCleanupFunc())
	}
}

func GetTestDataPath() string {
	return filepath.Join("..", "..", "..", "testdata")
}

func ShouldSkipScanner(module jfrogappsconfig.Module, scanType utils.JasScanType) bool {
	lowerScanType := strings.ToLower(string(scanType))
	if slices.Contains(module.ExcludeScanners, lowerScanType) {
		log.Info(fmt.Sprintf("Skipping %s scanning", scanType))
		return true
	}
	return false
}

func GetSourceRoots(module jfrogappsconfig.Module, scanner *jfrogappsconfig.Scanner) ([]string, error) {
	root, err := filepath.Abs(module.SourceRoot)
	if err != nil {
		return []string{}, errorutils.CheckError(err)
	}
	if scanner == nil || len(scanner.WorkingDirs) == 0 {
		return []string{root}, errorutils.CheckError(err)
	}
	var roots []string
	for _, workingDir := range scanner.WorkingDirs {
		roots = append(roots, filepath.Join(root, workingDir))
	}
	return roots, nil
}

func GetExcludePatterns(module jfrogappsconfig.Module, scanner *jfrogappsconfig.Scanner) []string {
	excludePatterns := module.ExcludePatterns
	if scanner != nil {
		excludePatterns = append(excludePatterns, scanner.ExcludePatterns...)
	}
	if len(excludePatterns) == 0 {
		return DefaultExcludePatterns
	}
	return excludePatterns
}
