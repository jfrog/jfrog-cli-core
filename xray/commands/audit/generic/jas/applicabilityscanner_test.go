package jas

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
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
				Cves:       []services.Cve{{Id: "test_cve_1"}, {Id: "test_cve_2"}, {Id: "test_cve_3"}},
				Components: map[string]services.Component{"issueId_1_direct_dependency": {}, "issueId_1_non_direct_dependency": {}}},
		},
		Violations: []services.Violation{
			{IssueId: "issueId_2", Technology: coreutils.Pipenv.ToString(),
				Cves:       []services.Cve{{Id: "test_cve_3"}, {Id: "test_cve_4"}},
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

//not entitled for jas

//TestNewApplicabilityScanManager_InputIsValid
//
//TestNewApplicabilityScanManager_DependencyTreeDoesntExist
//
//TestNewApplicabilityScanManager_NoDirectDependenciesInTree\
//
//TestNewApplicabilityScanManager_MultipleDependencyTrees
//
//TestNewApplicabilityScanManager_ViolationsdontExist
//
//TestNewApplicabilityScanManager_VulnerabilitiesDontExist
//
//TestApplicabilityScanManager_ShouldRun_AllConditionsMet
//
//TestApplicabilityScanManager_ShouldRun_AnalyzerManagerDoesntExist
//
//TestApplicabilityScanManager_ShouldRun_TechnologoesNotEligibleForScan
//
//TestApplicabilityScanManager_ShouldRun_ScanResultsAreEmpty

func TestCreateConfigFile_VerifyFileContent(t *testing.T) {

}

func TestParseResults_EmptyResults(t *testing.T) {

}

func TestParseResults_ApplicableCveExist(t *testing.T) {

}

func TestParseResults_AllCvesNotApplicable(t *testing.T) {

}

func TestParseResults_UnknownCves(t *testing.T) {

}

func TestGetExtendedScanResults_AnalyzerManagerDoesntExist(t *testing.T) {
	// arrange
	analyzerManagerExist = false
	analyzerManagerExecuter = &analyzerManagerMock{}

	// act
	extendedResults, err := GetExtendedScanResults(fakeBasicXrayResults, fakeBasicDependencyGraph)

	// assert
	assert.NoError(t, err)
	assert.False(t, extendedResults.EntitledForJas)
	assert.Equal(t, 1, len(extendedResults.XrayResults))
	assert.Nil(t, extendedResults.ApplicabilityScannerResults)
}

func TestGetExtendedScanResults_AnalyzerManagerReturnsError(t *testing.T) {
	// arrange
	analyzerManagerErrorMessage := "analyzer manager failure message"
	analyzerManagerExecutionError = errors.New(analyzerManagerErrorMessage)
	analyzerManagerExecuter = &analyzerManagerMock{}

	// act
	extendedResults, err := GetExtendedScanResults(fakeBasicXrayResults, fakeBasicDependencyGraph)

	// assert
	assert.Error(t, err)
	assert.Equal(t, analyzerManagerErrorMessage, err.Error())
	assert.Nil(t, extendedResults)
}
