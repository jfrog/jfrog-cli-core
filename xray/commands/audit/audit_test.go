package audit

import (
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

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
		result := getDirectDependenciesFromTree(test.dependenciesTrees)
		assert.ElementsMatch(t, test.expectedResult, result)
	}
}
