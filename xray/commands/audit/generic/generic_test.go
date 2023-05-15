package audit

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/generic/jas"
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

func (am *analyzerManagerMock) RunAnalyzerManager(string, string) error {
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

func TestGetExtendedScanResults_AnalyzerManagerDoesntExist(t *testing.T) {
	// Arrange
	analyzerManagerExist = false
	audit.analyzerManagerExecuter = &analyzerManagerMock{}

	// Act
	extendedResults, err := jas.GetExtendedScanResults(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails)

	// Assert
	assert.NoError(t, err)
	assert.False(t, extendedResults.EntitledForJas)
	assert.Equal(t, 1, len(extendedResults.XrayResults))
	assert.Nil(t, extendedResults.ApplicabilityScannerResults)

	// Cleanup
	analyzerManagerExist = true
}
