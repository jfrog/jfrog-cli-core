package sca

import (
	"github.com/jfrog/jfrog-client-go/xray/scan"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSetPathsForIssues(t *testing.T) {
	// Create a test dependency tree
	rootNode := &xrayUtils.GraphNode{Id: "root"}
	childNode1 := &xrayUtils.GraphNode{Id: "child1"}
	childNode2 := &xrayUtils.GraphNode{Id: "child2"}
	childNode3 := &xrayUtils.GraphNode{Id: "child3"}
	childNode4 := &xrayUtils.GraphNode{Id: "child4"}
	childNode5 := &xrayUtils.GraphNode{Id: "child5"}
	rootNode.Nodes = []*xrayUtils.GraphNode{childNode1, childNode2, childNode3}
	childNode2.Nodes = []*xrayUtils.GraphNode{childNode4}
	childNode3.Nodes = []*xrayUtils.GraphNode{childNode5}

	// Create a test issues map
	issuesMap := make(map[string]*scan.Component)
	issuesMap["child1"] = &scan.Component{ImpactPaths: [][]scan.ImpactPathNode{}}
	issuesMap["child4"] = &scan.Component{ImpactPaths: [][]scan.ImpactPathNode{}}
	issuesMap["child5"] = &scan.Component{ImpactPaths: [][]scan.ImpactPathNode{}}

	// Call setPathsForIssues with the test data
	setPathsForIssues(rootNode, issuesMap, []scan.ImpactPathNode{})

	// Check the results
	assert.Equal(t, issuesMap["child1"].ImpactPaths[0][0].ComponentId, "root")
	assert.Equal(t, issuesMap["child1"].ImpactPaths[0][1].ComponentId, "child1")

	assert.Equal(t, issuesMap["child4"].ImpactPaths[0][0].ComponentId, "root")
	assert.Equal(t, issuesMap["child4"].ImpactPaths[0][1].ComponentId, "child2")
	assert.Equal(t, issuesMap["child4"].ImpactPaths[0][2].ComponentId, "child4")

	assert.Equal(t, issuesMap["child5"].ImpactPaths[0][0].ComponentId, "root")
	assert.Equal(t, issuesMap["child5"].ImpactPaths[0][1].ComponentId, "child3")
	assert.Equal(t, issuesMap["child5"].ImpactPaths[0][2].ComponentId, "child5")
}

func TestUpdateVulnerableComponent(t *testing.T) {
	// Create test data
	components := map[string]scan.Component{
		"dependency1": {
			FixedVersions: []string{"1.0.0"},
			ImpactPaths:   [][]scan.ImpactPathNode{},
		},
	}
	dependencyName := "dependency1"
	issuesMap := map[string]*scan.Component{
		dependencyName: {
			FixedVersions: []string{"1.0.0"},
			ImpactPaths: [][]scan.ImpactPathNode{
				{{ComponentId: "dependency2"}},
			},
		},
	}

	updateComponentsWithImpactPaths(components, issuesMap)

	// Check the result
	expected := scan.Component{
		FixedVersions: []string{"1.0.0"},
		ImpactPaths:   issuesMap[dependencyName].ImpactPaths,
	}
	assert.Equal(t, expected, components[dependencyName])
}

func TestBuildImpactPaths(t *testing.T) {
	// create sample scan result and dependency trees
	scanResult := []scan.ScanResponse{
		{
			Vulnerabilities: []scan.Vulnerability{
				{
					Components: map[string]scan.Component{
						"dep1": {
							FixedVersions: []string{"1.2.3"},
							Cpes:          []string{"cpe:/o:vendor:product:1.2.3"},
						},
					},
				},
			},
			Violations: []scan.Violation{
				{
					Components: map[string]scan.Component{
						"dep2": {
							FixedVersions: []string{"4.5.6"},
							Cpes:          []string{"cpe:/o:vendor:product:4.5.6"},
						},
					},
				},
			},
			Licenses: []scan.License{
				{
					Components: map[string]scan.Component{
						"dep3": {
							FixedVersions: []string{"7.8.9"},
							Cpes:          []string{"cpe:/o:vendor:product:7.8.9"},
						},
					},
				},
			},
		},
	}
	dependencyTrees := []*xrayUtils.GraphNode{
		{
			Id: "dep1",
			Nodes: []*xrayUtils.GraphNode{
				{
					Id: "dep2",
					Nodes: []*xrayUtils.GraphNode{
						{
							Id:    "dep3",
							Nodes: []*xrayUtils.GraphNode{},
						},
					},
				},
			},
		},
	}

	scanResult = BuildImpactPathsForScanResponse(scanResult, dependencyTrees)
	// assert that the components were updated with impact paths
	expectedImpactPaths := [][]scan.ImpactPathNode{{{ComponentId: "dep1"}}}
	assert.Equal(t, expectedImpactPaths, scanResult[0].Vulnerabilities[0].Components["dep1"].ImpactPaths)
	expectedImpactPaths = [][]scan.ImpactPathNode{{{ComponentId: "dep1"}, {ComponentId: "dep2"}}}
	assert.Equal(t, expectedImpactPaths, scanResult[0].Violations[0].Components["dep2"].ImpactPaths)
	expectedImpactPaths = [][]scan.ImpactPathNode{{{ComponentId: "dep1"}, {ComponentId: "dep2"}, {ComponentId: "dep3"}}}
	assert.Equal(t, expectedImpactPaths, scanResult[0].Licenses[0].Components["dep3"].ImpactPaths)
}
