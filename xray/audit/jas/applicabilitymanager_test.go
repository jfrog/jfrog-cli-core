package jas

import (
	"errors"
	"github.com/jfrog/gofrog/datastructures"
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
	applicabilityManager, _, err := newApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, nil, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityManager)
	assert.NotEmpty(t, applicabilityManager.configFileName)
	assert.NotEmpty(t, applicabilityManager.resultsFileName)
	assert.Equal(t, applicabilityManager.directDependenciesCves.Size(), 5)
}

func TestNewApplicabilityScanManager_DependencyTreeDoesntExist(t *testing.T) {
	// Act
	applicabilityManager, _, err := newApplicabilityScanManager(fakeBasicXrayResults, nil, &fakeServerDetails, nil, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityManager)
	assert.NotEmpty(t, applicabilityManager.configFileName)
	assert.NotEmpty(t, applicabilityManager.resultsFileName)
	assert.Equal(t, applicabilityManager.directDependenciesCves.Size(), 0)
}

func TestNewApplicabilityScanManager_NoDirectDependenciesInScan(t *testing.T) {
	// Arrange
	var noDirectDependenciesResults = []services.ScanResponse{
		{
			ScanId: "scanId_1",
			Vulnerabilities: []services.Vulnerability{
				{IssueId: "issueId_1", Technology: coreutils.Pipenv.ToString(),
					Cves: []services.Cve{{Id: "testCve1"}, {Id: "testCve2"}, {Id: "testCve3"}},
					Components: map[string]services.Component{
						"issueId_1_non_direct_dependency": {}}},
			},
			Violations: []services.Violation{
				{IssueId: "issueId_2", Technology: coreutils.Pipenv.ToString(),
					Cves: []services.Cve{{Id: "testCve4"}, {Id: "testCve5"}},
					Components: map[string]services.Component{
						"issueId_2_non_direct_dependency": {}}},
			},
		},
	}
	fakeBasicXrayResults[0].Vulnerabilities[0].Components["issueId_1_non_direct_dependency"] = services.Component{}
	fakeBasicXrayResults[0].Violations[0].Components["issueId_2_non_direct_dependency"] = services.Component{}

	// Act
	applicabilityManager, _, err := newApplicabilityScanManager(noDirectDependenciesResults, fakeBasicDependencyGraph, &fakeServerDetails, nil, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityManager)
	assert.NotEmpty(t, applicabilityManager.configFileName)
	assert.NotEmpty(t, applicabilityManager.resultsFileName)
	// Non-direct dependencies should not be added
	assert.Equal(t, 0, applicabilityManager.directDependenciesCves.Size())
}

func TestNewApplicabilityScanManager_MultipleDependencyTrees(t *testing.T) {
	// Arrange
	multipleDependencyTrees := []*xrayUtils.GraphNode{multipleFakeBasicDependencyGraph[0], multipleFakeBasicDependencyGraph[1]}

	// Act
	applicabilityManager, _, err := newApplicabilityScanManager(fakeBasicXrayResults, multipleDependencyTrees, &fakeServerDetails, nil, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityManager)
	assert.NotEmpty(t, applicabilityManager.configFileName)
	assert.NotEmpty(t, applicabilityManager.resultsFileName)
	assert.Equal(t, 5, applicabilityManager.directDependenciesCves.Size())
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
	applicabilityManager, _, err := newApplicabilityScanManager(noViolationScanResponse, fakeBasicDependencyGraph, &fakeServerDetails, nil, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityManager)
	assert.NotEmpty(t, applicabilityManager.configFileName)
	assert.NotEmpty(t, applicabilityManager.resultsFileName)
	assert.Equal(t, 3, applicabilityManager.directDependenciesCves.Size())
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
	applicabilityManager, _, err := newApplicabilityScanManager(noVulnerabilitiesScanResponse, fakeBasicDependencyGraph, &fakeServerDetails, nil, &analyzerManagerMock{})

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityManager)
	assert.NotEmpty(t, applicabilityManager.configFileName)
	assert.NotEmpty(t, applicabilityManager.resultsFileName)
	assert.Equal(t, 2, applicabilityManager.directDependenciesCves.Size())
}

func TestApplicabilityScanManager_ShouldRun_TechnologiesNotEligibleForScan(t *testing.T) {
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, eligible, err := getApplicabilityScanResults(fakeBasicXrayResults, fakeBasicDependencyGraph,
		&fakeServerDetails, []coreutils.Technology{coreutils.Nuget, coreutils.Go}, nil, &analyzerManagerMock{})

	// Assert
	assert.Nil(t, applicabilityManager)
	assert.NoError(t, err)
	assert.False(t, eligible)
}

func TestApplicabilityScanManager_ShouldRun_ScanResultsAreEmpty(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, _, err := newApplicabilityScanManager(nil, fakeBasicDependencyGraph, &fakeServerDetails, nil, &analyzerManagerMock{})
	assert.NoError(t, err)
	// Assert
	eligible := applicabilityManager.shouldRunApplicabilityScan([]coreutils.Technology{coreutils.Npm})
	assert.False(t, eligible)
}

func TestExtractXrayDirectViolations(t *testing.T) {
	var xrayResponseForDirectViolationsTest = []services.ScanResponse{
		{
			Violations: []services.Violation{
				{IssueId: "issueId_2", Technology: coreutils.Pipenv.ToString(),
					Cves:       []services.Cve{{Id: "testCve4"}, {Id: "testCve5"}},
					Components: map[string]services.Component{"issueId_2_direct_dependency": {}}},
			},
		},
	}
	tests := []struct {
		directDependencies []string
		cvesCount          int
	}{
		{directDependencies: []string{"issueId_2_direct_dependency", "issueId_1_direct_dependency"},
			cvesCount: 2,
		},
		// Vulnerability dependency, should be ignored by function
		{directDependencies: []string{"issueId_1_direct_dependency"},
			cvesCount: 0,
		},
		{directDependencies: []string{},
			cvesCount: 0,
		},
	}

	for _, test := range tests {
		directDependenciesSet := datastructures.MakeSet[string]()
		for _, direct := range test.directDependencies {
			directDependenciesSet.Add(direct)
		}
		cves := extractDirectDependenciesCvesFromScan(xrayResponseForDirectViolationsTest, directDependenciesSet)
		assert.Equal(t, test.cvesCount, cves.Size())
	}
}

func TestExtractXrayDirectVulnerabilities(t *testing.T) {
	var xrayResponseForDirectVulnerabilitiesTest = []services.ScanResponse{
		{
			ScanId: "scanId_1",
			Vulnerabilities: []services.Vulnerability{
				{
					IssueId: "issueId_1", Technology: coreutils.Pipenv.ToString(),
					Cves:       []services.Cve{{Id: "testCve1"}, {Id: "testCve2"}, {Id: "testCve3"}},
					Components: map[string]services.Component{"issueId_1_direct_dependency": {}},
				},
				{
					IssueId: "issueId_2", Technology: coreutils.Pipenv.ToString(),
					Cves:       []services.Cve{{Id: "testCve4"}, {Id: "testCve5"}},
					Components: map[string]services.Component{"issueId_2_direct_dependency": {}},
				},
			},
		},
	}
	tests := []struct {
		directDependencies []string
		cvesCount          int
	}{
		{
			directDependencies: []string{"issueId_1_direct_dependency"},
			cvesCount:          3,
		},
		{
			directDependencies: []string{"issueId_2_direct_dependency"},
			cvesCount:          2,
		},
		{directDependencies: []string{},
			cvesCount: 0,
		},
	}

	for _, test := range tests {
		directDependenciesSet := datastructures.MakeSet[string]()
		for _, direct := range test.directDependencies {
			directDependenciesSet.Add(direct)
		}
		assert.Equal(t, test.cvesCount, extractDirectDependenciesCvesFromScan(xrayResponseForDirectVulnerabilitiesTest, directDependenciesSet).Size())
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
		result := getDirectDependenciesSet(test.dependenciesTrees)
		assert.ElementsMatch(t, test.expectedResult, result.ToSlice())
	}
}

func TestCreateConfigFile_VerifyFileWasCreated(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, _, applicabilityManagerError := newApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, nil, &analyzerManagerMock{})
	assert.NoError(t, applicabilityManagerError)

	currWd, err := coreutils.GetWorkingDirectory()
	assert.NoError(t, err)
	err = applicabilityManager.createConfigFile(currWd)
	assert.NoError(t, err)

	defer func() {
		err = os.Remove(applicabilityManager.configFileName)
		assert.NoError(t, err)
	}()

	_, fileNotExistError := os.Stat(applicabilityManager.configFileName)
	assert.NoError(t, fileNotExistError)
	fileContent, err := os.ReadFile(applicabilityManager.configFileName)
	assert.NoError(t, err)
	assert.True(t, len(fileContent) > 0)
}

func TestParseResults_EmptyResults_AllCvesShouldGetUnknown(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, _, applicabilityManagerError := newApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, nil, &analyzerManagerMock{})
	applicabilityManager.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "applicability-scan", "empty-results.sarif")

	// Act
	results, err := applicabilityManager.getScanResults()

	// Assert
	assert.NoError(t, applicabilityManagerError)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(results))
	for _, cveResult := range results {
		assert.Equal(t, utils.ApplicabilityUndeterminedStringValue, cveResult)
	}
}

func TestParseResults_ApplicableCveExist(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, _, applicabilityManagerError := newApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, nil, &analyzerManagerMock{})
	applicabilityManager.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "applicability-scan", "applicable-cve-results.sarif")

	// Act
	results, err := applicabilityManager.getScanResults()

	// Assert
	assert.NoError(t, applicabilityManagerError)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(results))
	assert.Equal(t, utils.ApplicableStringValue, results["testCve1"])
	assert.Equal(t, utils.NotApplicableStringValue, results["testCve3"])
}

func TestParseResults_AllCvesNotApplicable(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityManager, _, applicabilityManagerError := newApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, nil, &analyzerManagerMock{})
	applicabilityManager.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "applicability-scan", "no-applicable-cves-results.sarif")

	// Act
	results, err := applicabilityManager.getScanResults()

	// Assert
	assert.NoError(t, applicabilityManagerError)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(results))
	for _, cveResult := range results {
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
	extendedResults, err := GetExtendedScanResults(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, []coreutils.Technology{coreutils.Npm}, nil)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "failed to run Applicability scan. Exit code received: analyzer manager failure message", err.Error())
	assert.Nil(t, extendedResults)
}
