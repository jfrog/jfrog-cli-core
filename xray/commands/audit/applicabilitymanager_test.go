package audit

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestNewApplicabilityScanManager_InputIsValid(t *testing.T) {
	// Act
	applicabilityManager, _, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)

	// Assert
	assert.NotEmpty(t, applicabilityManager)
	assert.NotEmpty(t, applicabilityManager.configFileName)
	assert.NotEmpty(t, applicabilityManager.resultsFileName)
	assert.Equal(t, 1, len(applicabilityManager.xrayVulnerabilities))
	assert.Equal(t, 1, len(applicabilityManager.xrayViolations))
}

func TestNewApplicabilityScanManager_DependencyTreeDoesntExist(t *testing.T) {
	// Act
	applicabilityManager, _, _ := NewApplicabilityScanManager(fakeBasicXrayResults, nil, &fakeServerDetails)

	// Assert
	assert.NotEmpty(t, applicabilityManager)
	assert.NotEmpty(t, applicabilityManager.configFileName)
	assert.NotEmpty(t, applicabilityManager.resultsFileName)
	assert.Empty(t, applicabilityManager.xrayVulnerabilities)
	assert.Empty(t, applicabilityManager.xrayViolations)
}

func TestNewApplicabilityScanManager_NoDirectDependenciesInTree(t *testing.T) {
	// Arrange
	var noDirectDependenciesResults = []services.ScanResponse{
		{
			ScanId: "scanId_1",
			Vulnerabilities: []services.Vulnerability{
				{IssueId: "issueId_1", Technology: coreutils.Pipenv.ToString(),
					Cves: []services.Cve{{Id: "testCve1"}, {Id: "testCve2"}, {Id: "testCve3"}},
					Components: map[string]services.Component{
						"issueId_1_direct_dependency":     {},
						"issueId_1_non_direct_dependency": {}}},
			},
			Violations: []services.Violation{
				{IssueId: "issueId_2", Technology: coreutils.Pipenv.ToString(),
					Cves: []services.Cve{{Id: "testCve4"}, {Id: "testCve5"}},
					Components: map[string]services.Component{
						"issueId_2_direct_dependency":     {},
						"issueId_2_non_direct_dependency": {}}},
			},
		},
	}
	fakeBasicXrayResults[0].Vulnerabilities[0].Components["issueId_1_non_direct_dependency"] = services.Component{}
	fakeBasicXrayResults[0].Violations[0].Components["issueId_2_non_direct_dependency"] = services.Component{}

	// Act
	applicabilityManager, _, _ := NewApplicabilityScanManager(noDirectDependenciesResults, fakeBasicDependencyGraph, &fakeServerDetails)

	// Assert
	assert.NotEmpty(t, applicabilityManager)
	assert.NotEmpty(t, applicabilityManager.configFileName)
	assert.NotEmpty(t, applicabilityManager.resultsFileName)
	assert.Equal(t, 1, len(applicabilityManager.xrayVulnerabilities)) // non-direct dependency should not be added
	assert.Equal(t, 1, len(applicabilityManager.xrayViolations))      // non-direct dependency should not be added
}

func TestNewApplicabilityScanManager_MultipleDependencyTrees(t *testing.T) {
	// Arrange
	multipleDependencyTrees := []*services.GraphNode{fakeBasicDependencyGraph[0], fakeBasicDependencyGraph[0]}

	// Act
	applicabilityManager, _, _ := NewApplicabilityScanManager(fakeBasicXrayResults, multipleDependencyTrees, &fakeServerDetails)

	// Assert
	assert.NotEmpty(t, applicabilityManager)
	assert.NotEmpty(t, applicabilityManager.configFileName)
	assert.NotEmpty(t, applicabilityManager.resultsFileName)
	assert.Equal(t, 2, len(applicabilityManager.xrayVulnerabilities))
	assert.Equal(t, 2, len(applicabilityManager.xrayViolations))
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
	applicabilityManager, _, _ := NewApplicabilityScanManager(noViolationScanResponse, fakeBasicDependencyGraph, &fakeServerDetails)

	// Assert
	assert.NotEmpty(t, applicabilityManager)
	assert.NotEmpty(t, applicabilityManager.configFileName)
	assert.NotEmpty(t, applicabilityManager.resultsFileName)
	assert.Equal(t, 1, len(applicabilityManager.xrayVulnerabilities))
	assert.Empty(t, applicabilityManager.xrayViolations)
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
	applicabilityManager, _, _ := NewApplicabilityScanManager(noVulnerabilitiesScanResponse, fakeBasicDependencyGraph, &fakeServerDetails)

	// Assert
	assert.NotEmpty(t, applicabilityManager)
	assert.NotEmpty(t, applicabilityManager.configFileName)
	assert.NotEmpty(t, applicabilityManager.resultsFileName)
	assert.Equal(t, 1, len(applicabilityManager.xrayViolations))
	assert.Empty(t, applicabilityManager.xrayVulnerabilities)
}

func TestApplicabilityScanManager_ShouldRun_AllConditionsMet(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, _, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)

	// Act
	shouldRun, _ := applicabilityManager.shouldRun()

	// Assert
	assert.True(t, shouldRun)
}

func TestApplicabilityScanManager_ShouldRun_AnalyzerManagerDoesntExist(t *testing.T) {
	// Arrange
	analyzerManagerExist = false
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, _, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)

	// Act
	shouldRun, _ := applicabilityManager.shouldRun()

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
	applicabilityManager, _, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph,
		&fakeServerDetails)

	// Act
	shouldRun, _ := applicabilityManager.shouldRun()

	// Assert
	assert.False(t, shouldRun)

	// Cleanup
	fakeBasicXrayResults[0].Vulnerabilities[0].Technology = coreutils.Pipenv.ToString()
	fakeBasicXrayResults[0].Violations[0].Technology = coreutils.Pipenv.ToString()
}

func TestApplicabilityScanManager_ShouldRun_ScanResultsAreEmpty(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, _, _ := NewApplicabilityScanManager(nil, fakeBasicDependencyGraph, &fakeServerDetails)

	// Act
	shouldRun, _ := applicabilityManager.shouldRun()

	// Assert
	assert.False(t, shouldRun)
}

func TestResultsIncludeEligibleTechnologies(t *testing.T) {
	tests := []struct {
		vulnerabilities []services.Vulnerability
		violations      []services.Violation
		expectedResult  bool
	}{
		{vulnerabilities: []services.Vulnerability{{Technology: "npm"}}, violations: []services.Violation{{Technology: "go"}}, expectedResult: true},
		{vulnerabilities: []services.Vulnerability{{Technology: "go"}}, violations: []services.Violation{{Technology: "npm"}}, expectedResult: true},
		{vulnerabilities: []services.Vulnerability{{Technology: "npm"}}, violations: []services.Violation{{Technology: "npm"}}, expectedResult: true},
		{vulnerabilities: []services.Vulnerability{{Technology: "go"}}, violations: []services.Violation{{Technology: "go"}}, expectedResult: false},
	}
	for _, test := range tests {
		assert.Equal(t, test.expectedResult, resultsIncludeEligibleTechnologies(test.vulnerabilities, test.violations))
	}
}

func TestExtractXrayDirectViolations(t *testing.T) {
	var xrayResponseForDirectViolationsTest = []services.ScanResponse{
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
	tests := []struct {
		directDependencies []string
		expectedResult     []services.Violation
	}{
		{directDependencies: []string{"issueId_2_direct_dependency", "issueId_1_direct_dependency"},
			expectedResult: []services.Violation{
				{IssueId: "issueId_2", Technology: coreutils.Pipenv.ToString(),
					Cves:       []services.Cve{{Id: "testCve4"}, {Id: "testCve5"}},
					Components: map[string]services.Component{"issueId_2_direct_dependency": {}}},
			},
		},
		{directDependencies: []string{"issueId_1_direct_dependency"}, // vulnerability dependency, should be ignored by function
			expectedResult: []services.Violation{},
		},
		{directDependencies: []string{},
			expectedResult: []services.Violation{},
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedResult, extractXrayDirectViolations(xrayResponseForDirectViolationsTest, test.directDependencies))
	}
}

func TestExtractXrayDirectVulnerabilities(t *testing.T) {
	var xrayResponseForDirectVulnerabilitiesTest = []services.ScanResponse{
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
	tests := []struct {
		directDependencies []string
		expectedResult     []services.Vulnerability
	}{
		{directDependencies: []string{"issueId_2_direct_dependency", "issueId_1_direct_dependency"},
			expectedResult: []services.Vulnerability{
				{IssueId: "issueId_1", Technology: coreutils.Pipenv.ToString(),
					Cves:       []services.Cve{{Id: "testCve1"}, {Id: "testCve2"}, {Id: "testCve3"}},
					Components: map[string]services.Component{"issueId_1_direct_dependency": {}}},
			},
		},
		{directDependencies: []string{"issueId_2_direct_dependency"}, // violation dependency, should be ignored by function
			expectedResult: []services.Vulnerability{},
		},
		{directDependencies: []string{},
			expectedResult: []services.Vulnerability{},
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedResult, extractXrayDirectVulnerabilities(xrayResponseForDirectVulnerabilitiesTest, test.directDependencies))
	}
}

func TestGetDirectDependenciesList(t *testing.T) {
	tests := []struct {
		dependenciesTrees []*services.GraphNode
		expectedResult    []string
	}{
		{
			dependenciesTrees: nil,
			expectedResult:    []string{},
		},
		{
			dependenciesTrees: []*services.GraphNode{
				{Id: "parent_node_id", Nodes: []*services.GraphNode{
					{Id: "issueId_1_direct_dependency", Nodes: []*services.GraphNode{{Id: "issueId_1_non_direct_dependency"}}},
					{Id: "issueId_2_direct_dependency", Nodes: nil},
				},
				},
			},
			expectedResult: []string{"issueId_1_direct_dependency", "issueId_2_direct_dependency"},
		},
		{
			dependenciesTrees: []*services.GraphNode{
				{Id: "parent_node_id", Nodes: []*services.GraphNode{
					{Id: "issueId_1_direct_dependency", Nodes: nil},
					{Id: "issueId_2_direct_dependency", Nodes: nil},
				},
				},
			},
			expectedResult: []string{"issueId_1_direct_dependency", "issueId_2_direct_dependency"},
		},
	}

	for _, test := range tests {
		assert.ElementsMatch(t, test.expectedResult, getDirectDependenciesList(test.dependenciesTrees))
	}
}

func TestCreateConfigFile_VerifyFileWasCreated(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, _, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)

	// Act
	err := applicabilityManager.createConfigFile()

	// Assert
	assert.NoError(t, err)
	_, fileNotExistError := os.Stat(applicabilityManager.configFileName)
	assert.NoError(t, fileNotExistError)
	fileContent, _ := os.ReadFile(applicabilityManager.configFileName)
	assert.True(t, len(fileContent) > 0)

	// Cleanup
	err = os.Remove(applicabilityManager.configFileName)
	assert.NoError(t, err)
}

func TestParseResults_EmptyResults_AllCvesShouldGetUnknown(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, _, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)
	applicabilityManager.resultsFileName = filepath.Join("..", "testdata", "applicability-scan", "empty-results.sarif")

	// Act
	err := applicabilityManager.parseResults()

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityManager.applicabilityScanResults)
	assert.Equal(t, 5, len(applicabilityManager.applicabilityScanResults))
	for _, cveResult := range applicabilityManager.applicabilityScanResults {
		assert.Equal(t, utils.ApplicabilityUndeterminedStringValue, cveResult)
	}
}

func TestParseResults_ApplicableCveExist(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, _, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)
	applicabilityManager.resultsFileName = filepath.Join("..", "testdata", "applicability-scan", "applicable-cve-results.sarif")

	// Act
	err := applicabilityManager.parseResults()

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityManager.applicabilityScanResults)
	assert.Equal(t, 5, len(applicabilityManager.applicabilityScanResults))
	assert.Equal(t, utils.ApplicableStringValue, applicabilityManager.applicabilityScanResults["testCve1"])
	assert.Equal(t, utils.NotApplicableStringValue, applicabilityManager.applicabilityScanResults["testCve3"])

}

func TestParseResults_AllCvesNotApplicable(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, _, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)
	applicabilityManager.resultsFileName = filepath.Join("..", "testdata", "applicability-scan", "no-applicable-cves-results.sarif")

	// Act
	err := applicabilityManager.parseResults()

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityManager.applicabilityScanResults)
	assert.Equal(t, 5, len(applicabilityManager.applicabilityScanResults))
	for _, cveResult := range applicabilityManager.applicabilityScanResults {
		assert.Equal(t, utils.NotApplicableStringValue, cveResult)
	}
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
	assert.Equal(t, fmt.Sprintf(applicabilityScanFailureMessage, analyzerManagerErrorMessage), err.Error())
	assert.Nil(t, extendedResults)

	// Cleanup
	analyzerManagerExecutionError = nil
}
