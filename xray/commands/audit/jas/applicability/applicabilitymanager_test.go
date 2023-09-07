package applicability

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

var mockDirectDependencies = []string{"issueId_2_direct_dependency", "issueId_1_direct_dependency"}
var mockMultiRootDirectDependencies = []string{"issueId_2_direct_dependency", "issueId_1_direct_dependency", "issueId_3_direct_dependency", "issueId_4_direct_dependency"}

func TestNewApplicabilityScanManager_InputIsValid(t *testing.T) {
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()
	// Act
	applicabilityManager := newApplicabilityScanManager(jas.FakeBasicXrayResults, mockDirectDependencies, scanner, false)

	// Assert
	if assert.NotNil(t, applicabilityManager) {
		assert.NotEmpty(t, applicabilityManager.scanner.ConfigFileName)
		assert.NotEmpty(t, applicabilityManager.scanner.ResultsFileName)
		assert.Len(t, applicabilityManager.dependencyWhitelist, 5)
	}
}

func TestNewApplicabilityScanManager_DependencyTreeDoesntExist(t *testing.T) {
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()
	// Act
	applicabilityManager := newApplicabilityScanManager(jas.FakeBasicXrayResults, nil, scanner, false)

	// Assert
	if assert.NotNil(t, applicabilityManager) {
		assert.NotNil(t, applicabilityManager.scanner.ScannerDirCleanupFunc)
		assert.Len(t, applicabilityManager.scanner.WorkingDirs, 1)
		assert.NotEmpty(t, applicabilityManager.scanner.ConfigFileName)
		assert.NotEmpty(t, applicabilityManager.scanner.ResultsFileName)
		assert.Empty(t, applicabilityManager.dependencyWhitelist)
	}
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
	jas.FakeBasicXrayResults[0].Vulnerabilities[0].Components["issueId_1_non_direct_dependency"] = services.Component{}
	jas.FakeBasicXrayResults[0].Violations[0].Components["issueId_2_non_direct_dependency"] = services.Component{}

	// Act
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()
	applicabilityManager := newApplicabilityScanManager(noDirectDependenciesResults, mockDirectDependencies, scanner, false)

	// Assert
	if assert.NotNil(t, applicabilityManager) {
		assert.NotEmpty(t, applicabilityManager.scanner.ConfigFileName)
		assert.NotEmpty(t, applicabilityManager.scanner.ResultsFileName)
		// Non-direct dependencies should not be added
		assert.Empty(t, applicabilityManager.dependencyWhitelist)
	}
}

func TestNewApplicabilityScanManager_MultipleDependencyTrees(t *testing.T) {
	// Arrange
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()
	// Act
	applicabilityManager := newApplicabilityScanManager(jas.FakeBasicXrayResults, mockMultiRootDirectDependencies, scanner, false)

	// Assert
	if assert.NotNil(t, applicabilityManager) {
		assert.NotEmpty(t, applicabilityManager.scanner.ConfigFileName)
		assert.NotEmpty(t, applicabilityManager.scanner.ResultsFileName)
		assert.Len(t, applicabilityManager.dependencyWhitelist, 5)
	}
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
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()

	// Act
	applicabilityManager := newApplicabilityScanManager(noViolationScanResponse, mockDirectDependencies, scanner, false)

	// Assert
	if assert.NotNil(t, applicabilityManager) {
		assert.NotEmpty(t, applicabilityManager.scanner.ConfigFileName)
		assert.NotEmpty(t, applicabilityManager.scanner.ResultsFileName)
		assert.Len(t, applicabilityManager.dependencyWhitelist, 3)
	}
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
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()

	// Act
	applicabilityManager := newApplicabilityScanManager(noVulnerabilitiesScanResponse, mockDirectDependencies, scanner, false)

	// Assert
	if assert.NotNil(t, applicabilityManager) {
		assert.NotEmpty(t, applicabilityManager.scanner.ConfigFileName)
		assert.NotEmpty(t, applicabilityManager.scanner.ResultsFileName)
		assert.Len(t, applicabilityManager.dependencyWhitelist, 2)
	}
}

func TestApplicabilityScanManager_ShouldRun_TechnologiesNotEligibleForScan(t *testing.T) {
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()

	results, err := RunApplicabilityScan(jas.FakeBasicXrayResults, mockDirectDependencies, []coreutils.Technology{coreutils.Nuget, coreutils.Go}, scanner, false)

	// Assert
	assert.Nil(t, results)
	assert.NoError(t, err)
}

func TestApplicabilityScanManager_ShouldRun_ScanResultsAreEmpty(t *testing.T) {
	// Arrange
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()

	applicabilityManager := newApplicabilityScanManager(nil, mockDirectDependencies, scanner, false)

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
		cves := prepareDependenciesCvesWhitelist(xrayResponseForDirectViolationsTest, test.directDependencies)
		assert.Len(t, cves, test.cvesCount)
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
		assert.Len(t, prepareDependenciesCvesWhitelist(xrayResponseForDirectVulnerabilitiesTest, test.directDependencies), test.cvesCount)
	}
}

func TestCreateConfigFile_VerifyFileWasCreated(t *testing.T) {
	// Arrange
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()

	applicabilityManager := newApplicabilityScanManager(jas.FakeBasicXrayResults, []string{"issueId_1_direct_dependency", "issueId_2_direct_dependency"}, scanner, false)

	currWd, err := coreutils.GetWorkingDirectory()
	assert.NoError(t, err)
	err = applicabilityManager.createConfigFile(currWd, false)
	assert.NoError(t, err)

	defer func() {
		err = os.Remove(applicabilityManager.scanner.ConfigFileName)
		assert.NoError(t, err)
	}()

	_, fileNotExistError := os.Stat(applicabilityManager.scanner.ConfigFileName)
	assert.NoError(t, fileNotExistError)
	fileContent, err := os.ReadFile(applicabilityManager.scanner.ConfigFileName)
	assert.NoError(t, err)
	assert.True(t, len(fileContent) > 0)
}

func TestParseResults_EmptyResults_AllCvesShouldGetUnknown(t *testing.T) {
	// Arrange
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()

	applicabilityManager := newApplicabilityScanManager(jas.FakeBasicXrayResults, mockDirectDependencies, scanner, false)
	applicabilityManager.scanner.ResultsFileName = filepath.Join(jas.GetTestDataPath(), "applicability-scan", "empty-results.sarif")

	// Act
	var err error
	applicabilityManager.applicabilityScanResults, err = utils.ReadScanRunsFromFile(applicabilityManager.scanner.ResultsFileName)

	if assert.NoError(t, err) {
		assert.Len(t, applicabilityManager.applicabilityScanResults, 1)
		assert.Empty(t, applicabilityManager.applicabilityScanResults[0].Results)
		processApplicabilityScanResults(applicabilityManager.applicabilityScanResults, scanner.WorkingDirs[0], false)
		assert.Empty(t, applicabilityManager.applicabilityScanResults[0].Results)
	}
}

func TestParseResults_ApplicableCveExist(t *testing.T) {
	// Arrange
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()
	applicabilityManager := newApplicabilityScanManager(jas.FakeBasicXrayResults, mockDirectDependencies, scanner, false)
	applicabilityManager.scanner.ResultsFileName = filepath.Join(jas.GetTestDataPath(), "applicability-scan", "applicable-cve-results.sarif")

	// Act
	var err error
	applicabilityManager.applicabilityScanResults, err = utils.ReadScanRunsFromFile(applicabilityManager.scanner.ResultsFileName)

	if assert.NoError(t, err) {
		assert.Len(t, applicabilityManager.applicabilityScanResults, 1)
		processApplicabilityScanResults(applicabilityManager.applicabilityScanResults, scanner.WorkingDirs[0], false)
		assert.Len(t, applicabilityManager.applicabilityScanResults[0].Results, 5)
	}

	// // Assert
	// assert.NoError(t, err)
	// assert.Equal(t, 5, len(results))
	// assert.Equal(t, utils.Applicable, results["testCve1"])
	// assert.Equal(t, utils.NotApplicable, results["testCve3"])
}

func TestParseResults_AllCvesNotApplicable(t *testing.T) {
	// Arrange
	scanner, cleanUp := jas.InitJasTest(t)
	defer cleanUp()
	applicabilityManager := newApplicabilityScanManager(jas.FakeBasicXrayResults, mockDirectDependencies, scanner, false)
	applicabilityManager.scanner.ResultsFileName = filepath.Join(jas.GetTestDataPath(), "applicability-scan", "no-applicable-cves-results.sarif")

	// Act
	var err error
	applicabilityManager.applicabilityScanResults, err = utils.ReadScanRunsFromFile(applicabilityManager.scanner.ResultsFileName)

	if assert.NoError(t, err) {
		assert.Len(t, applicabilityManager.applicabilityScanResults, 1)
		processApplicabilityScanResults(applicabilityManager.applicabilityScanResults, scanner.WorkingDirs[0], false)
		assert.Len(t, applicabilityManager.applicabilityScanResults[0].Results, 5)
	}

	// // Assert
	// assert.NoError(t, err)
	// assert.Equal(t, 5, len(results))
	// for _, cveResult := range results {
	// 	assert.Equal(t, utils.NotApplicable, cveResult)
	// }
}
