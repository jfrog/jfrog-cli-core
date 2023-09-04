package jas

import (
	rtutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/xray/scan"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestNewApplicabilityScanManager_InputIsValid(t *testing.T) {
	scanner, cleanUp := initJasTest(t)
	defer cleanUp()
	// Act
	applicabilityManager := newApplicabilityScanManager(fakeBasicXrayResults, mockDirectDependencies, scanner)

	// Assert
	if assert.NotNil(t, applicabilityManager) {
		assert.NotEmpty(t, applicabilityManager.scanner.configFileName)
		assert.NotEmpty(t, applicabilityManager.scanner.resultsFileName)
		assert.Len(t, applicabilityManager.directDependenciesCves, 5)
	}
}

func TestNewApplicabilityScanManager_DependencyTreeDoesntExist(t *testing.T) {
	scanner, cleanUp := initJasTest(t)
	defer cleanUp()
	// Act
	applicabilityManager := newApplicabilityScanManager(fakeBasicXrayResults, nil, scanner)

	// Assert
	if assert.NotNil(t, applicabilityManager) {
		assert.NotNil(t, applicabilityManager.scanner.scannerDirCleanupFunc)
		assert.Len(t, applicabilityManager.scanner.workingDirs, 1)
		assert.NotEmpty(t, applicabilityManager.scanner.configFileName)
		assert.NotEmpty(t, applicabilityManager.scanner.resultsFileName)
		assert.Empty(t, applicabilityManager.directDependenciesCves)
	}
}

func TestNewApplicabilityScanManager_NoDirectDependenciesInScan(t *testing.T) {
	// Arrange
	var noDirectDependenciesResults = []scan.ScanResponse{
		{
			ScanId: "scanId_1",
			Vulnerabilities: []scan.Vulnerability{
				{IssueId: "issueId_1", Technology: coreutils.Pipenv.ToString(),
					Cves: []scan.Cve{{Id: "testCve1"}, {Id: "testCve2"}, {Id: "testCve3"}},
					Components: map[string]scan.Component{
						"issueId_1_non_direct_dependency": {}}},
			},
			Violations: []scan.Violation{
				{IssueId: "issueId_2", Technology: coreutils.Pipenv.ToString(),
					Cves: []scan.Cve{{Id: "testCve4"}, {Id: "testCve5"}},
					Components: map[string]scan.Component{
						"issueId_2_non_direct_dependency": {}}},
			},
		},
	}
	fakeBasicXrayResults[0].Vulnerabilities[0].Components["issueId_1_non_direct_dependency"] = scan.Component{}
	fakeBasicXrayResults[0].Violations[0].Components["issueId_2_non_direct_dependency"] = scan.Component{}

	// Act
	scanner, cleanUp := initJasTest(t)
	defer cleanUp()
	applicabilityManager := newApplicabilityScanManager(noDirectDependenciesResults, mockDirectDependencies, scanner)

	// Assert
	if assert.NotNil(t, applicabilityManager) {
		assert.NotEmpty(t, applicabilityManager.scanner.configFileName)
		assert.NotEmpty(t, applicabilityManager.scanner.resultsFileName)
		// Non-direct dependencies should not be added
		assert.Empty(t, applicabilityManager.directDependenciesCves)
	}
}

func TestNewApplicabilityScanManager_MultipleDependencyTrees(t *testing.T) {
	// Arrange
	scanner, cleanUp := initJasTest(t)
	defer cleanUp()
	// Act
	applicabilityManager := newApplicabilityScanManager(fakeBasicXrayResults, mockMultiRootDirectDependencies, scanner)

	// Assert
	if assert.NotNil(t, applicabilityManager) {
		assert.NotEmpty(t, applicabilityManager.scanner.configFileName)
		assert.NotEmpty(t, applicabilityManager.scanner.resultsFileName)
		assert.Len(t, applicabilityManager.directDependenciesCves, 5)
	}
}

func TestNewApplicabilityScanManager_ViolationsDontExistInResults(t *testing.T) {
	// Arrange
	noViolationScanResponse := []scan.ScanResponse{
		{
			ScanId: "scanId_1",
			Vulnerabilities: []scan.Vulnerability{
				{IssueId: "issueId_1", Technology: coreutils.Pipenv.ToString(),
					Cves:       []scan.Cve{{Id: "test_cve_1"}, {Id: "test_cve_2"}, {Id: "test_cve_3"}},
					Components: map[string]scan.Component{"issueId_1_direct_dependency": {}}},
			},
		},
	}
	scanner, cleanUp := initJasTest(t)
	defer cleanUp()

	// Act
	applicabilityManager := newApplicabilityScanManager(noViolationScanResponse, mockDirectDependencies, scanner)

	// Assert
	if assert.NotNil(t, applicabilityManager) {
		assert.NotEmpty(t, applicabilityManager.scanner.configFileName)
		assert.NotEmpty(t, applicabilityManager.scanner.resultsFileName)
		assert.Len(t, applicabilityManager.directDependenciesCves, 3)
	}
}

func TestNewApplicabilityScanManager_VulnerabilitiesDontExist(t *testing.T) {
	// Arrange
	noVulnerabilitiesScanResponse := []scan.ScanResponse{
		{
			ScanId: "scanId_1",
			Violations: []scan.Violation{
				{IssueId: "issueId_2", Technology: coreutils.Pipenv.ToString(),
					Cves:       []scan.Cve{{Id: "test_cve_3"}, {Id: "test_cve_4"}},
					Components: map[string]scan.Component{"issueId_2_direct_dependency": {}}},
			},
		},
	}
	scanner, cleanUp := initJasTest(t)
	defer cleanUp()

	// Act
	applicabilityManager := newApplicabilityScanManager(noVulnerabilitiesScanResponse, mockDirectDependencies, scanner)

	// Assert
	if assert.NotNil(t, applicabilityManager) {
		assert.NotEmpty(t, applicabilityManager.scanner.configFileName)
		assert.NotEmpty(t, applicabilityManager.scanner.resultsFileName)
		assert.Len(t, applicabilityManager.directDependenciesCves, 2)
	}
}

func TestApplicabilityScanManager_ShouldRun_TechnologiesNotEligibleForScan(t *testing.T) {
	scanner, cleanUp := initJasTest(t)
	defer cleanUp()

	results, err := getApplicabilityScanResults(fakeBasicXrayResults, mockDirectDependencies,
		[]coreutils.Technology{coreutils.Nuget, coreutils.Go}, scanner)

	// Assert
	assert.Nil(t, results)
	assert.NoError(t, err)
}

func TestApplicabilityScanManager_ShouldRun_ScanResultsAreEmpty(t *testing.T) {
	// Arrange
	scanner, cleanUp := initJasTest(t)
	defer cleanUp()

	applicabilityManager := newApplicabilityScanManager(nil, mockDirectDependencies, scanner)

	// Assert
	eligible := applicabilityManager.shouldRunApplicabilityScan([]coreutils.Technology{coreutils.Npm})
	assert.False(t, eligible)
}

func TestExtractXrayDirectViolations(t *testing.T) {
	var xrayResponseForDirectViolationsTest = []scan.ScanResponse{
		{
			Violations: []scan.Violation{
				{IssueId: "issueId_2", Technology: coreutils.Pipenv.ToString(),
					Cves:       []scan.Cve{{Id: "testCve4"}, {Id: "testCve5"}},
					Components: map[string]scan.Component{"issueId_2_direct_dependency": {}}},
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
		cves := extractDirectDependenciesCvesFromScan(xrayResponseForDirectViolationsTest, test.directDependencies)
		assert.Len(t, cves, test.cvesCount)
	}
}

func TestExtractXrayDirectVulnerabilities(t *testing.T) {
	var xrayResponseForDirectVulnerabilitiesTest = []scan.ScanResponse{
		{
			ScanId: "scanId_1",
			Vulnerabilities: []scan.Vulnerability{
				{
					IssueId: "issueId_1", Technology: coreutils.Pipenv.ToString(),
					Cves:       []scan.Cve{{Id: "testCve1"}, {Id: "testCve2"}, {Id: "testCve3"}},
					Components: map[string]scan.Component{"issueId_1_direct_dependency": {}},
				},
				{
					IssueId: "issueId_2", Technology: coreutils.Pipenv.ToString(),
					Cves:       []scan.Cve{{Id: "testCve4"}, {Id: "testCve5"}},
					Components: map[string]scan.Component{"issueId_2_direct_dependency": {}},
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
		assert.Len(t, extractDirectDependenciesCvesFromScan(xrayResponseForDirectVulnerabilitiesTest, test.directDependencies), test.cvesCount)
	}
}

func TestCreateConfigFile_VerifyFileWasCreated(t *testing.T) {
	// Arrange
	scanner, cleanUp := initJasTest(t)
	defer cleanUp()

	applicabilityManager := newApplicabilityScanManager(fakeBasicXrayResults, []string{"issueId_1_direct_dependency", "issueId_2_direct_dependency"}, scanner)

	currWd, err := coreutils.GetWorkingDirectory()
	assert.NoError(t, err)
	err = applicabilityManager.createConfigFile(currWd)
	assert.NoError(t, err)

	defer func() {
		err = os.Remove(applicabilityManager.scanner.configFileName)
		assert.NoError(t, err)
	}()

	_, fileNotExistError := os.Stat(applicabilityManager.scanner.configFileName)
	assert.NoError(t, fileNotExistError)
	fileContent, err := os.ReadFile(applicabilityManager.scanner.configFileName)
	assert.NoError(t, err)
	assert.True(t, len(fileContent) > 0)
}

func TestParseResults_EmptyResults_AllCvesShouldGetUnknown(t *testing.T) {
	// Arrange
	scanner, cleanUp := initJasTest(t)
	defer cleanUp()

	applicabilityManager := newApplicabilityScanManager(fakeBasicXrayResults, mockDirectDependencies, scanner)
	applicabilityManager.scanner.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "applicability-scan", "empty-results.sarif")

	// Act
	results, err := applicabilityManager.getScanResults()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 5, len(results))
	for _, cveResult := range results {
		assert.Equal(t, utils.ApplicabilityUndetermined, cveResult)
	}
}

func TestParseResults_ApplicableCveExist(t *testing.T) {
	// Arrange
	scanner, cleanUp := initJasTest(t)
	defer cleanUp()
	applicabilityManager := newApplicabilityScanManager(fakeBasicXrayResults, mockDirectDependencies, scanner)
	applicabilityManager.scanner.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "applicability-scan", "applicable-cve-results.sarif")

	// Act
	results, err := applicabilityManager.getScanResults()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 5, len(results))
	assert.Equal(t, utils.Applicable, results["testCve1"])
	assert.Equal(t, utils.NotApplicable, results["testCve3"])
}

func TestParseResults_AllCvesNotApplicable(t *testing.T) {
	// Arrange
	scanner, cleanUp := initJasTest(t)
	defer cleanUp()
	applicabilityManager := newApplicabilityScanManager(fakeBasicXrayResults, mockDirectDependencies, scanner)
	applicabilityManager.scanner.resultsFileName = filepath.Join("..", "..", "commands", "testdata", "applicability-scan", "no-applicable-cves-results.sarif")

	// Act
	results, err := applicabilityManager.getScanResults()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 5, len(results))
	for _, cveResult := range results {
		assert.Equal(t, utils.NotApplicable, cveResult)
	}
}

func TestGetExtendedScanResults_AnalyzerManagerReturnsError(t *testing.T) {
	assert.NoError(t, rtutils.DownloadAnalyzerManagerIfNeeded())
	scanResults := &utils.ExtendedScanResults{XrayResults: fakeBasicXrayResults, ScannedTechnologies: []coreutils.Technology{coreutils.Yarn}}
	err := RunScannersAndSetResults(scanResults, mockDirectDependencies, &fakeServerDetails, nil, nil, "")

	// Expect error:
	assert.ErrorContains(t, err, "failed to run Applicability scan")
}
