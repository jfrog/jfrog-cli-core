package jas

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestNewApplicabilityScanManager_InputIsValid(t *testing.T) {
	// Act
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})

	// Assert
	assert.NotEmpty(t, applicabilityScanner)
	assert.NotEmpty(t, applicabilityScanner.configFileName)
	assert.NotEmpty(t, applicabilityScanner.resultsFileName)
	assert.Equal(t, 1, len(applicabilityScanner.xrayVulnerabilities))
	assert.Equal(t, 1, len(applicabilityScanner.xrayViolations))
}

func TestNewApplicabilityScanManager_DependencyTreeDoesntExist(t *testing.T) {
	// Act
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, nil, &fakeServerDetails, &analyzerManagerMock{})

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
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})

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
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, multipleDependencyTrees, &fakeServerDetails, &analyzerManagerMock{})

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
	applicabilityScanner, _ := NewApplicabilityScanManager(noViolationScanResponse, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})

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
	applicabilityScanner, _ := NewApplicabilityScanManager(noVulnerabilitiesScanResponse, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})

	// Assert
	assert.NotEmpty(t, applicabilityScanner)
	assert.NotEmpty(t, applicabilityScanner.configFileName)
	assert.NotEmpty(t, applicabilityScanner.resultsFileName)
	assert.Equal(t, 1, len(applicabilityScanner.xrayViolations))
	assert.Empty(t, applicabilityScanner.xrayVulnerabilities)
}

func TestApplicabilityScanManager_IsEntitled_AllConditionsMet(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})

	// Act
	entitled := applicabilityScanner.entitledForAppScan()

	// Assert
	assert.True(t, entitled)
}

func TestApplicabilityScanManager_IsEntitled_TechnologiesNotEligibleForScan(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	fakeBasicXrayResults[0].Vulnerabilities[0].Technology = coreutils.Nuget.ToString()
	fakeBasicXrayResults[0].Violations[0].Technology = coreutils.Go.ToString()
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph,
		&fakeServerDetails, &analyzerManagerMock{})

	// Act
	entitled := applicabilityScanner.entitledForAppScan()

	// Assert
	assert.False(t, entitled)

	// Cleanup
	fakeBasicXrayResults[0].Vulnerabilities[0].Technology = coreutils.Pipenv.ToString()
	fakeBasicXrayResults[0].Violations[0].Technology = coreutils.Pipenv.ToString()
}

func TestApplicabilityScanManager_IsEntitled_ScanResultsAreEmpty(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityScanner, _ := NewApplicabilityScanManager(nil, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})

	// Act
	entitled := applicabilityScanner.entitledForAppScan()

	// Assert
	assert.False(t, entitled)
}

func TestCreateConfigFile_VerifyFileWasCreated(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})

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
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})
	applicabilityScanner.resultsFileName = filepath.Join("..", "..", "..", "testdata", "applicability-scan", "empty-results.sarif")

	// Act
	err := applicabilityScanner.parseResults()

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityScanner.applicabilityScannerResults)
	assert.Equal(t, 5, len(applicabilityScanner.applicabilityScannerResults))
	for _, cveResult := range applicabilityScanner.applicabilityScannerResults {
		assert.Equal(t, "Unknown", cveResult)
	}
}

func TestParseResults_ApplicableCveExist(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})
	applicabilityScanner.resultsFileName = filepath.Join("..", "..", "..", "testdata", "applicability-scan", "applicable-cve-results.sarif")

	// Act
	err := applicabilityScanner.parseResults()

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityScanner.applicabilityScannerResults)
	assert.Equal(t, 5, len(applicabilityScanner.applicabilityScannerResults))
	assert.Equal(t, "Yes", applicabilityScanner.applicabilityScannerResults["testCve1"])
	assert.Equal(t, "No", applicabilityScanner.applicabilityScannerResults["testCve3"])

}

func TestParseResults_AllCvesNotApplicable(t *testing.T) {
	// Arrange
	analyzerManagerExecuter = &analyzerManagerMock{}
	applicabilityScanner, _ := NewApplicabilityScanManager(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, &analyzerManagerMock{})
	applicabilityScanner.resultsFileName = filepath.Join("..", "..", "..", "testdata", "applicability-scan", "no-applicable-cves-results.sarif")

	// Act
	err := applicabilityScanner.parseResults()

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, applicabilityScanner.applicabilityScannerResults)
	assert.Equal(t, 5, len(applicabilityScanner.applicabilityScannerResults))
	for _, cveResult := range applicabilityScanner.applicabilityScannerResults {
		assert.Equal(t, "No", cveResult)
	}
}

func TestApplicabilityScan_GetExtendedScanResults_AnalyzerManagerReturnsError(t *testing.T) {
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
