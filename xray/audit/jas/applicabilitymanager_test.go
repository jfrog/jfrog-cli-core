package jas

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestNewApplicabilityScanManager_InputIsValid(t *testing.T) {
	// Act
	applicabilityManager, _, err := newApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityManager)
	assert.NotEmpty(t, applicabilityManager.configFileName)
	assert.NotEmpty(t, applicabilityManager.resultsFileName)
	assert.Equal(t, 1, len(applicabilityManager.xrayVulnerabilities))
	assert.Equal(t, 1, len(applicabilityManager.xrayViolations))
}

func TestNewApplicabilityScanManager_DependencyTreeDoesntExist(t *testing.T) {
	// Act
	applicabilityManager, _, err := newApplicabilityScanManager(fakeBasicXrayResults, nil, &fakeServerDetails, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
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
	applicabilityManager, _, err := newApplicabilityScanManager(noDirectDependenciesResults, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityManager)
	assert.NotEmpty(t, applicabilityManager.configFileName)
	assert.NotEmpty(t, applicabilityManager.resultsFileName)
	// Non-direct dependencies should not be added
	assert.Equal(t, 1, len(applicabilityManager.xrayVulnerabilities))
	assert.Equal(t, 1, len(applicabilityManager.xrayViolations))
}

func TestNewApplicabilityScanManager_MultipleDependencyTrees(t *testing.T) {
	// Arrange
	multipleDependencyTrees := []*xrayUtils.GraphNode{fakeBasicDependencyGraph[0], fakeBasicDependencyGraph[0]}

	// Act
	applicabilityManager, _, err := newApplicabilityScanManager(fakeBasicXrayResults, multipleDependencyTrees, &fakeServerDetails, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
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
	applicabilityManager, _, err := newApplicabilityScanManager(noViolationScanResponse, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
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
	applicabilityManager, _, err := newApplicabilityScanManager(noVulnerabilitiesScanResponse, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityManager)
	assert.NotEmpty(t, applicabilityManager.configFileName)
	assert.NotEmpty(t, applicabilityManager.resultsFileName)
	assert.Equal(t, 1, len(applicabilityManager.xrayViolations))
	assert.Empty(t, applicabilityManager.xrayVulnerabilities)
}

func TestApplicabilityScanManager_ShouldRun_AllConditionsMet(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, _, err := newApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})

	// Act
	eligible := applicabilityManager.eligibleForApplicabilityScan()

	// Assert
	assert.NoError(t, err)
	assert.True(t, eligible)
}

func TestApplicabilityScanManager_ShouldRun_TechnologiesNotEligibleForScan(t *testing.T) {
	defer func() {
		fakeBasicXrayResults[0].Vulnerabilities[0].Technology = coreutils.Pipenv.ToString()
		fakeBasicXrayResults[0].Violations[0].Technology = coreutils.Pipenv.ToString()
	}()

	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	fakeBasicXrayResults[0].Vulnerabilities[0].Technology = coreutils.Nuget.ToString()
	fakeBasicXrayResults[0].Violations[0].Technology = coreutils.Go.ToString()
	applicabilityManager, _, err := newApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph,
		&fakeServerDetails, &analyzerManagerMock{})

	// Act
	eligible := applicabilityManager.eligibleForApplicabilityScan()

	// Assert
	assert.NoError(t, err)
	assert.False(t, eligible)
}

func TestApplicabilityScanManager_ShouldRun_ScanResultsAreEmpty(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, _, err := newApplicabilityScanManager(nil, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})

	// Act
	eligible := applicabilityManager.eligibleForApplicabilityScan()

	// Assert
	assert.NoError(t, err)
	assert.False(t, eligible)
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
		// Vulnerability dependency, should be ignored by function
		{directDependencies: []string{"issueId_1_direct_dependency"},
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
		dependenciesTrees []*xrayUtils.GraphNode
		expectedResult    []string
	}{
		{
			dependenciesTrees: nil,
			expectedResult:    []string{},
		},
		{
			dependenciesTrees: []*xrayUtils.GraphNode{
				{Id: "parent_node_id", Nodes: []*xrayUtils.GraphNode{
					{Id: "issueId_1_direct_dependency", Nodes: []*xrayUtils.GraphNode{{Id: "issueId_1_non_direct_dependency"}}},
					{Id: "issueId_2_direct_dependency", Nodes: nil},
				},
				},
			},
			expectedResult: []string{"issueId_1_direct_dependency", "issueId_2_direct_dependency"},
		},
		{
			dependenciesTrees: []*xrayUtils.GraphNode{
				{Id: "parent_node_id", Nodes: []*xrayUtils.GraphNode{
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
	applicabilityManager, _, applicabilityManagerError := newApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})

	// Act
	err := applicabilityManager.createConfigFile()

	defer func() {
		err = os.Remove(applicabilityManager.configFileName)
		assert.NoError(t, err)
	}()

	// Assert
	assert.NoError(t, applicabilityManagerError)
	assert.NoError(t, err)
	_, fileNotExistError := os.Stat(applicabilityManager.configFileName)
	assert.NoError(t, fileNotExistError)
	fileContent, err := os.ReadFile(applicabilityManager.configFileName)
	assert.NoError(t, err)
	assert.True(t, len(fileContent) > 0)
}

func TestParseResults_EmptyResults_AllCvesShouldGetUnknown(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, _, applicabilityManagerError := newApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})
	applicabilityManager.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "applicability-scan", "empty-results.sarif")

	// Act
	err := applicabilityManager.setScanResults()

	// Assert
	assert.NoError(t, applicabilityManagerError)
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
	applicabilityManager, _, applicabilityManagerError := newApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})
	applicabilityManager.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "applicability-scan", "applicable-cve-results.sarif")

	// Act
	err := applicabilityManager.setScanResults()

	// Assert
	assert.NoError(t, applicabilityManagerError)
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityManager.applicabilityScanResults)
	assert.Equal(t, 5, len(applicabilityManager.applicabilityScanResults))
	assert.Equal(t, utils.ApplicableStringValue, applicabilityManager.applicabilityScanResults["testCve1"])
	assert.Equal(t, utils.NotApplicableStringValue, applicabilityManager.applicabilityScanResults["testCve3"])
}

func TestParseResults_AllCvesNotApplicable(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, _, applicabilityManagerError := newApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})
	applicabilityManager.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "applicability-scan", "no-applicable-cves-results.sarif")

	// Act
	err := applicabilityManager.setScanResults()

	// Assert
	assert.NoError(t, applicabilityManagerError)
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityManager.applicabilityScanResults)
	assert.Equal(t, 5, len(applicabilityManager.applicabilityScanResults))
	for _, cveResult := range applicabilityManager.applicabilityScanResults {
		assert.Equal(t, utils.NotApplicableStringValue, cveResult)
	}
}

func TestGetExtendedScanResults_AnalyzerManagerReturnsError(t *testing.T) {
	defer func() {
		analyzerManagerExecutionError = nil
	}()
	// Arrange
	analyzerManagerErrorMessage := "analyzer manager failure message"
	analyzerManagerExecutionError = errors.New(analyzerManagerErrorMessage)
	analyzerManagerExecuter = &analyzerManagerMock{}

	// Act
	extendedResults, err := GetExtendedScanResults(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, "")

	// Assert
	assert.Error(t, err)
	assert.Equal(t, fmt.Sprintf(applicabilityScanFailureMessage, analyzerManagerErrorMessage), err.Error())
	assert.Nil(t, extendedResults)
}
