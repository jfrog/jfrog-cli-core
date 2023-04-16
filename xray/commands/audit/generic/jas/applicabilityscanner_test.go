package jas

import (
	"errors"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	analyzerManagerExecutionError error = nil
	analyzerManagerNotExistError  error = nil
)

type analyzerManagerMock struct {
}

func (am *analyzerManagerMock) RunAnalyzerManager(string) error {
	return analyzerManagerExecutionError
}

func (am *analyzerManagerMock) DoesAnalyzerManagerExecutableExist() error {
	return analyzerManagerExecutionError
}

var fakeXrayResults = []services.ScanResponse{
	{
		ScanId: "scanId_1",
		Vulnerabilities: []services.Vulnerability{
			{IssueId: "issueId_1", Cves: []services.Cve{{Id: "test_cve_1"}, {Id: "test_cve_2"}, {Id: "test_cve_3"}}},
		},
	},
	{
		ScanId: "scanId_2",
		Vulnerabilities: []services.Vulnerability{
			{IssueId: "issueId_2", Cves: []services.Cve{{Id: "test_cve_4"}, {Id: "test_cve_5"}}},
		},
	},
}

func TestGetExtendedScanResults_SuccessfulScan(t *testing.T) {
	// arrange
	analyzerManagerExecuter = &analyzerManagerMock{}

	// act
	extendedResults, err := GetExtendedScanResults(fakeXrayResults)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, 2, len(extendedResults.XrayResults))
	assert.Nil(t, extendedResults.ApplicableCves)
}

func TestGetExtendedScanResults_AnalyzerManagerDoesntExist(t *testing.T) {
	// arrange
	analyzerManagerNotExistError = errors.New("file does not exist error")
	analyzerManagerExecuter = &analyzerManagerMock{}

	// act
	extendedResults, err := GetExtendedScanResults(fakeXrayResults)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, 2, len(extendedResults.XrayResults))
	assert.Nil(t, extendedResults.ApplicableCves)
}

func TestGetExtendedScanResults_AnalyzerManagerReturnsError(t *testing.T) {

}

func TestRun_SuccessfulScan(t *testing.T) {

}

func TestRun_CreateConfigFileFailure(t *testing.T) {

}

func TestRun_ParseResultsFailure(t *testing.T) {

}

func TestCreateConfigFile_VerifyFileContent(t *testing.T) {

}

func TestParseResults_EmptyResults(t *testing.T) {

}

func TestParseResults_ApplicableCveExist(t *testing.T) {

}

func TestParseResults_AllCvesNotApplicable(t *testing.T) {

}
