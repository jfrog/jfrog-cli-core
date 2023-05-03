package jas

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

var (
	analyzerManagerExecutionError error = nil
	analyzerManagerExist                = true
)

type analyzerManagerMock struct {
}

func (am *analyzerManagerMock) RunAnalyzerManager(string) error {
	return analyzerManagerExecutionError
}

func (am *analyzerManagerMock) DoesAnalyzerManagerExecutableExist() bool {
	return analyzerManagerExist
}

var fakeBasicXrayResults = []services.ScanResponse{
	{
		ScanId: "scanId_1",
		Vulnerabilities: []services.Vulnerability{
			{IssueId: "issueId_1", Technology: coreutils.Pipenv.ToString(),
				Cves:       []services.Cve{{Id: "testCve1"}, {Id: "testCve2"}, {Id: "testCve3"}},
				Components: map[string]services.Component{"issueId_1_direct_dependency": {}}},
		},
		Violations: []services.Violation{
			{IssueId: "issueId_2", Technology: coreutils.Pipenv.ToString(),
				Cves:       []services.Cve{{Id: "testCve4"}, {Id: "testCve5"}},
				Components: map[string]services.Component{"issueId_2_direct_dependency": {}}},
		},
	},
}

var fakeBasicDependencyGraph = []*services.GraphNode{
	{
		Id: "parent_node_id",
		Nodes: []*services.GraphNode{
			{Id: "issueId_1_direct_dependency", Nodes: []*services.GraphNode{{Id: "issueId_1_non_direct_dependency"}}},
			{Id: "issueId_2_direct_dependency", Nodes: nil},
		},
	},
}

var fakeServerDetails = config.ServerDetails{
	Url:      "platformUrl",
	Password: "password",
	User:     "user",
}

func TestNewApplicabilityScanManager_InputIsValid(t *testing.T) {
	// Act
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)

	// Assert
	assert.NotEmpty(t, applicabilityScanner)
	assert.NotEmpty(t, applicabilityScanner.configFileName)
	assert.NotEmpty(t, applicabilityScanner.resultsFileName)
	assert.Equal(t, 1, len(applicabilityScanner.xrayVulnerabilities))
	assert.Equal(t, 1, len(applicabilityScanner.xrayViolations))
}

func TestNewApplicabilityScanManager_DependencyTreeDoesntExist(t *testing.T) {
	// Act
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, nil, &fakeServerDetails)

	// Assert
	assert.NotEmpty(t, applicabilityScanner)
	assert.NotEmpty(t, applicabilityScanner.configFileName)
	assert.NotEmpty(t, applicabilityScanner.resultsFileName)
	assert.Empty(t, applicabilityScanner.xrayVulnerabilities)
	assert.Empty(t, applicabilityScanner.xrayViolations)
}

func TestNewApplicabilityScanManager_NoDirectDependenciesInTree(t *testing.T) {
	// Arrange
	fakeBasicXrayResults[0].Vulnerabilities[0].Components["issueId_1_non_direct_dependency"] = services.Component{}
	fakeBasicXrayResults[0].Violations[0].Components["issueId_2_non_direct_dependency"] = services.Component{}

	// Act
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)

	// Assert
	assert.NotEmpty(t, applicabilityScanner)
	assert.NotEmpty(t, applicabilityScanner.configFileName)
	assert.NotEmpty(t, applicabilityScanner.resultsFileName)
	assert.Equal(t, 1, len(applicabilityScanner.xrayVulnerabilities)) // non-direct dependency should not be added
	assert.Equal(t, 1, len(applicabilityScanner.xrayViolations))      // non-direct dependency should not be added
}

func TestNewApplicabilityScanManager_MultipleDependencyTrees(t *testing.T) {
	// Arrange
	multipleDependencyTrees := []*services.GraphNode{fakeBasicDependencyGraph[0], fakeBasicDependencyGraph[0]}

	// Act
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, multipleDependencyTrees, &fakeServerDetails)

	// Assert
	assert.NotEmpty(t, applicabilityScanner)
	assert.NotEmpty(t, applicabilityScanner.configFileName)
	assert.NotEmpty(t, applicabilityScanner.resultsFileName)
	assert.Equal(t, 2, len(applicabilityScanner.xrayVulnerabilities))
	assert.Equal(t, 2, len(applicabilityScanner.xrayViolations))
}

func TestNewApplicabilityScanManager_ViolationsDontExistInResults(t *testing.T) {
	// Arrange
	noViolationScanResponse := []services.ScanResponse{
		{
			ScanId: "scanId_1",
			Vulnerabilities: []services.Vulnerability{
				{IssueId: "issueId_1", Technology: coreutils.Pipenv.ToString(),
					Cves:       []services.Cve{{Id: "test_cve_1"}, {Id: "test_cve_2"}, {Id: "test_cve_3"}},
					Components: map[string]services.Component{"issueId_1_direct_dependency": {}}},
			},
		},
	}

	// Act
	applicabilityScanner, _ := NewApplicabilityScanManager(noViolationScanResponse, fakeBasicDependencyGraph, &fakeServerDetails)

	// Assert
	assert.NotEmpty(t, applicabilityScanner)
	assert.NotEmpty(t, applicabilityScanner.configFileName)
	assert.NotEmpty(t, applicabilityScanner.resultsFileName)
	assert.Equal(t, 1, len(applicabilityScanner.xrayVulnerabilities))
	assert.Empty(t, applicabilityScanner.xrayViolations)
}

func TestNewApplicabilityScanManager_VulnerabilitiesDontExist(t *testing.T) {
	// Arrange
	noVulnerabilitiesScanResponse := []services.ScanResponse{
		{
			ScanId: "scanId_1",
			Violations: []services.Violation{
				{IssueId: "issueId_2", Technology: coreutils.Pipenv.ToString(),
					Cves:       []services.Cve{{Id: "test_cve_3"}, {Id: "test_cve_4"}},
					Components: map[string]services.Component{"issueId_2_direct_dependency": {}}},
			},
		},
	}

	// Act
	applicabilityScanner, _ := NewApplicabilityScanManager(noVulnerabilitiesScanResponse, fakeBasicDependencyGraph, &fakeServerDetails)

	// Assert
	assert.NotEmpty(t, applicabilityScanner)
	assert.NotEmpty(t, applicabilityScanner.configFileName)
	assert.NotEmpty(t, applicabilityScanner.resultsFileName)
	assert.Equal(t, 1, len(applicabilityScanner.xrayViolations))
	assert.Empty(t, applicabilityScanner.xrayVulnerabilities)
}

func TestApplicabilityScanManager_ShouldRun_AllConditionsMet(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)

	// Act
	shouldRun := applicabilityScanner.shouldRun()

	// Assert
	assert.True(t, shouldRun)
}

func TestApplicabilityScanManager_ShouldRun_AnalyzerManagerDoesntExist(t *testing.T) {
	// Arrange
	analyzerManagerExist = false
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)

	// Act
	shouldRun := applicabilityScanner.shouldRun()

	// Assert
	assert.False(t, shouldRun)

	// Cleanup
	analyzerManagerExist = true
}

func TestApplicabilityScanManager_ShouldRun_TechnologiesNotEligibleForScan(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	fakeBasicXrayResults[0].Vulnerabilities[0].Technology = coreutils.Nuget.ToString()
	fakeBasicXrayResults[0].Violations[0].Technology = coreutils.Go.ToString()
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph,
		&fakeServerDetails)

	// Act
	shouldRun := applicabilityScanner.shouldRun()

	// Assert
	assert.False(t, shouldRun)

	// Cleanup
	fakeBasicXrayResults[0].Vulnerabilities[0].Technology = coreutils.Pipenv.ToString()
	fakeBasicXrayResults[0].Violations[0].Technology = coreutils.Pipenv.ToString()
}

func TestApplicabilityScanManager_ShouldRun_ScanResultsAreEmpty(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityScanner, _ := NewApplicabilityScanManager(nil, fakeBasicDependencyGraph, &fakeServerDetails)

	// Act
	shouldRun := applicabilityScanner.shouldRun()

	// Assert
	assert.False(t, shouldRun)
}

func TestCreateConfigFile_VerifyFileWasCreated(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)

	// Act
	err := applicabilityScanner.createConfigFile()

	// Assert
	assert.NoError(t, err)
	_, fileNotExistError := os.Stat(applicabilityScanner.configFileName)
	assert.NoError(t, fileNotExistError)
	fileContent, _ := os.ReadFile(applicabilityScanner.configFileName)
	assert.True(t, len(fileContent) > 0)

	// Cleanup
	err = os.Remove(applicabilityScanner.configFileName)
	assert.NoError(t, err)
}

func TestParseResults_EmptyResults_AllCvesShouldGetUnknown(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)
	applicabilityScanner.resultsFileName = filepath.Join("..", "..", "..", "testdata", "applicability-scan", "empty-results.sarif")

	// Act
	err := applicabilityScanner.parseResults()

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityScanner.applicabilityScannerResults)
	assert.Equal(t, 5, len(applicabilityScanner.applicabilityScannerResults))
	for _, cveResult := range applicabilityScanner.applicabilityScannerResults {
		assert.Equal(t, UndeterminedStringValue, cveResult)
	}
}

func TestParseResults_ApplicableCveExist(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)
	applicabilityScanner.resultsFileName = filepath.Join("..", "..", "..", "testdata", "applicability-scan", "applicable-cve-results.sarif")

	// Act
	err := applicabilityScanner.parseResults()

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityScanner.applicabilityScannerResults)
	assert.Equal(t, 5, len(applicabilityScanner.applicabilityScannerResults))
	assert.Equal(t, ApplicableStringValue, applicabilityScanner.applicabilityScannerResults["testCve1"])
	assert.Equal(t, NotApplicableStringValue, applicabilityScanner.applicabilityScannerResults["testCve3"])

}

func TestParseResults_AllCvesNotApplicable(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)
	applicabilityScanner.resultsFileName = filepath.Join("..", "..", "..", "testdata", "applicability-scan", "no-applicable-cves-results.sarif")

	// Act
	err := applicabilityScanner.parseResults()

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityScanner.applicabilityScannerResults)
	assert.Equal(t, 5, len(applicabilityScanner.applicabilityScannerResults))
	for _, cveResult := range applicabilityScanner.applicabilityScannerResults {
		assert.Equal(t, NotApplicableStringValue, cveResult)
	}
}

func TestGetExtendedScanResults_AnalyzerManagerDoesntExist(t *testing.T) {
	// Arrange
	analyzerManagerExist = false
	analyzerManagerExecuter = &analyzerManagerMock{}

	// Act
	extendedResults, err := GetExtendedScanResults(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)

	// Assert
	assert.NoError(t, err)
	assert.False(t, extendedResults.EntitledForJas)
	assert.Equal(t, 1, len(extendedResults.XrayResults))
	assert.Nil(t, extendedResults.ApplicabilityScannerResults)

	// Cleanup
	analyzerManagerExist = true
}

func TestGetExtendedScanResults_AnalyzerManagerReturnsError(t *testing.T) {
	// Arrange
	analyzerManagerErrorMessage := "analyzer manager failure message"
	analyzerManagerExecutionError = errors.New(analyzerManagerErrorMessage)
	analyzerManagerExecuter = &analyzerManagerMock{}

	// Act
	extendedResults, err := GetExtendedScanResults(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, analyzerManagerErrorMessage, err.Error())
	assert.Nil(t, extendedResults)

	// Cleanup
	analyzerManagerExecutionError = nil
}
