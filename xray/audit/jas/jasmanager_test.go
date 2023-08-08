package jas

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	analyzerManagerExecutionError error = nil
	analyzerManagerExists               = true
)

type analyzerManagerMock struct {
}

func (am *analyzerManagerMock) Exec(string, string, *config.ServerDetails) error {
	return analyzerManagerExecutionError
}

func (am *analyzerManagerMock) ExistLocally() (bool, error) {
	return analyzerManagerExists, nil
}

var fakeBasicXrayResults = []services.ScanResponse{
	{
		ScanId: "scanId_1",
		Vulnerabilities: []services.Vulnerability{
			{IssueId: "issueId_1", Technology: coreutils.Pipenv.ToString(),
				Cves:       []services.Cve{{Id: "testCve1"}, {Id: "testCve2"}, {Id: "testCve3"}},
				Components: map[string]services.Component{"issueId_1_direct_dependency": {}, "issueId_3_direct_dependency": {}}},
		},
		Violations: []services.Violation{
			{IssueId: "issueId_2", Technology: coreutils.Pipenv.ToString(),
				Cves:       []services.Cve{{Id: "testCve4"}, {Id: "testCve5"}},
				Components: map[string]services.Component{"issueId_2_direct_dependency": {}, "issueId_4_direct_dependency": {}}},
		},
	},
}

var fakeBasicDependencyGraph = []*xrayUtils.GraphNode{
	{
		Id: "parent_node_id",
		Nodes: []*xrayUtils.GraphNode{
			{Id: "issueId_1_direct_dependency", Nodes: []*xrayUtils.GraphNode{{Id: "issueId_1_non_direct_dependency"}}},
			{Id: "issueId_2_direct_dependency", Nodes: nil},
		},
	},
}

var multipleFakeBasicDependencyGraph = []*xrayUtils.GraphNode{
	{
		Id: "parent_node_id",
		Nodes: []*xrayUtils.GraphNode{
			{Id: "issueId_1_direct_dependency", Nodes: []*xrayUtils.GraphNode{{Id: "issueId_1_non_direct_dependency"}}},
			{Id: "issueId_2_direct_dependency", Nodes: nil},
		},
	},
	{
		Id: "parent_node_id",
		Nodes: []*xrayUtils.GraphNode{
			{Id: "issueId_3_direct_dependency", Nodes: []*xrayUtils.GraphNode{{Id: "issueId_2_non_direct_dependency"}}},
			{Id: "issueId_4_direct_dependency", Nodes: nil},
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
	analyzerManagerExists = false
	analyzerManagerExecuter = &analyzerManagerMock{}

	// Act
	extendedResults, err := GetExtendedScanResults(fakeBasicXrayResults, fakeBasicDependencyGraph, &fakeServerDetails, []coreutils.Technology{coreutils.Yarn}, nil)

	// Assert
	assert.NoError(t, err)
	assert.False(t, extendedResults.EntitledForJas)
	assert.Equal(t, 1, len(extendedResults.XrayResults))
	assert.Nil(t, extendedResults.ApplicabilityScanResults)
}

func TestGetExtendedScanResults_ServerNotValid(t *testing.T) {
	// Act
	extendedResults, err := GetExtendedScanResults(fakeBasicXrayResults, fakeBasicDependencyGraph, nil, []coreutils.Technology{coreutils.Pip}, nil)

	// Assert
	assert.NotNil(t, extendedResults)
	assert.NoError(t, err)
}
