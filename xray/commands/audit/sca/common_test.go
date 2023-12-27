package sca

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/xray/services"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"github.com/stretchr/testify/assert"
	"reflect"
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
	issuesMap := make(map[string][][]services.ImpactPathNode)
	issuesMap["child1"] = [][]services.ImpactPathNode{}
	issuesMap["child4"] = [][]services.ImpactPathNode{}
	issuesMap["child5"] = [][]services.ImpactPathNode{}

	// Call setPathsForIssues with the test data
	setPathsForIssues(rootNode, issuesMap, []services.ImpactPathNode{})

	// Check the results
	assert.Equal(t, issuesMap["child1"][0][0].ComponentId, "root")
	assert.Equal(t, issuesMap["child1"][0][1].ComponentId, "child1")

	assert.Equal(t, issuesMap["child4"][0][0].ComponentId, "root")
	assert.Equal(t, issuesMap["child4"][0][1].ComponentId, "child2")
	assert.Equal(t, issuesMap["child4"][0][2].ComponentId, "child4")

	assert.Equal(t, issuesMap["child5"][0][0].ComponentId, "root")
	assert.Equal(t, issuesMap["child5"][0][1].ComponentId, "child3")
	assert.Equal(t, issuesMap["child5"][0][2].ComponentId, "child5")
}

func TestUpdateVulnerableComponent(t *testing.T) {
	components := map[string]services.Component{
		"dependency1": {
			FixedVersions: []string{"1.0.0"},
			ImpactPaths:   [][]services.ImpactPathNode{},
		},
	}
	dependencyName, issuesMap := "dependency1", map[string][][]services.ImpactPathNode{
		"dependency1": {},
	}

	updateComponentsWithImpactPaths(components, issuesMap)

	// Check the result
	expected := services.Component{
		FixedVersions: []string{"1.0.0"},
		ImpactPaths:   issuesMap[dependencyName],
	}
	assert.Equal(t, expected, components[dependencyName])
}

func TestBuildImpactPaths(t *testing.T) {
	// create sample scan result and dependency trees
	scanResult := []services.ScanResponse{
		{
			Vulnerabilities: []services.Vulnerability{
				{
					Components: map[string]services.Component{
						"dep1": {
							FixedVersions: []string{"1.2.3"},
							Cpes:          []string{"cpe:/o:vendor:product:1.2.3"},
						},
						"dep2": {
							FixedVersions: []string{"3.0.0"},
						},
					},
				},
			},
			Violations: []services.Violation{
				{
					Components: map[string]services.Component{
						"dep2": {
							FixedVersions: []string{"4.5.6"},
							Cpes:          []string{"cpe:/o:vendor:product:4.5.6"},
						},
					},
				},
			},
			Licenses: []services.License{
				{
					Components: map[string]services.Component{
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
		{
			Id: "dep7",
			Nodes: []*xrayUtils.GraphNode{
				{
					Id: "dep4",
					Nodes: []*xrayUtils.GraphNode{
						{
							Id:    "dep2",
							Nodes: []*xrayUtils.GraphNode{},
						},
						{
							Id:    "dep5",
							Nodes: []*xrayUtils.GraphNode{},
						},
						{
							Id:    "dep6",
							Nodes: []*xrayUtils.GraphNode{},
						},
					},
				},
			},
		},
	}

	scanResult = BuildImpactPathsForScanResponse(scanResult, dependencyTrees)
	// assert that the components were updated with impact paths
	expectedImpactPaths := [][]services.ImpactPathNode{{{ComponentId: "dep1"}}}
	assert.Equal(t, expectedImpactPaths, scanResult[0].Vulnerabilities[0].Components["dep1"].ImpactPaths)
	expectedImpactPaths = [][]services.ImpactPathNode{{{ComponentId: "dep1"}, {ComponentId: "dep2"}}}
	reflect.DeepEqual(expectedImpactPaths, scanResult[0].Vulnerabilities[0].Components["dep2"].ImpactPaths[0])
	expectedImpactPaths = [][]services.ImpactPathNode{{{ComponentId: "dep7"}, {ComponentId: "dep4"}, {ComponentId: "dep2"}}}
	reflect.DeepEqual(expectedImpactPaths, scanResult[0].Vulnerabilities[0].Components["dep2"].ImpactPaths[1])
	expectedImpactPaths = [][]services.ImpactPathNode{{{ComponentId: "dep1"}}}
	reflect.DeepEqual(expectedImpactPaths, scanResult[0].Violations[0].Components["dep1"].ImpactPaths)
	expectedImpactPaths = [][]services.ImpactPathNode{{{ComponentId: "dep1"}, {ComponentId: "dep2"}}}
	reflect.DeepEqual(expectedImpactPaths, scanResult[0].Violations[0].Components["dep2"].ImpactPaths[0])
	expectedImpactPaths = [][]services.ImpactPathNode{{{ComponentId: "dep7"}, {ComponentId: "dep4"}, {ComponentId: "dep2"}}}
	reflect.DeepEqual(expectedImpactPaths, scanResult[0].Violations[0].Components["dep2"].ImpactPaths[1])
	expectedImpactPaths = [][]services.ImpactPathNode{{{ComponentId: "dep7"}, {ComponentId: "dep4"}, {ComponentId: "dep2"}}}
	reflect.DeepEqual(expectedImpactPaths, scanResult[0].Violations[0].Components["dep2"].ImpactPaths)
	expectedImpactPaths = [][]services.ImpactPathNode{{{ComponentId: "dep1"}, {ComponentId: "dep2"}, {ComponentId: "dep3"}}}
	reflect.DeepEqual(expectedImpactPaths, scanResult[0].Licenses[0].Components["dep3"].ImpactPaths)
}

func TestBuildXrayDependencyTree(t *testing.T) {
	treeHelper := make(map[string][]string)
	rootDep := []string{"topDep1", "topDep2", "topDep3"}
	topDep1 := []string{"midDep1", "midDep2"}
	topDep2 := []string{"midDep2", "midDep3"}
	midDep1 := []string{"bottomDep1"}
	midDep2 := []string{"bottomDep2", "bottomDep3"}
	bottomDep3 := []string{"leafDep"}
	treeHelper["rootDep"] = rootDep
	treeHelper["topDep1"] = topDep1
	treeHelper["topDep2"] = topDep2
	treeHelper["midDep1"] = midDep1
	treeHelper["midDep2"] = midDep2
	treeHelper["bottomDep3"] = bottomDep3

	expectedUniqueDeps := []string{"rootDep", "topDep1", "topDep2", "topDep3", "midDep1", "midDep2", "midDep3", "bottomDep1", "bottomDep2", "bottomDep3", "leafDep"}

	// Constructing the expected tree Nodes
	leafDepNode := &xrayUtils.GraphNode{Id: "leafDep", Nodes: []*xrayUtils.GraphNode{}}
	bottomDep3Node := &xrayUtils.GraphNode{Id: "bottomDep3", Nodes: []*xrayUtils.GraphNode{}}
	bottomDep2Node := &xrayUtils.GraphNode{Id: "bottomDep2", Nodes: []*xrayUtils.GraphNode{}}
	bottomDep1Node := &xrayUtils.GraphNode{Id: "bottomDep1", Nodes: []*xrayUtils.GraphNode{}}
	midDep3Node := &xrayUtils.GraphNode{Id: "midDep3", Nodes: []*xrayUtils.GraphNode{}}
	midDep2Node := &xrayUtils.GraphNode{Id: "midDep2", Nodes: []*xrayUtils.GraphNode{}}
	midDep1Node := &xrayUtils.GraphNode{Id: "midDep1", Nodes: []*xrayUtils.GraphNode{}}
	topDep3Node := &xrayUtils.GraphNode{Id: "topDep3", Nodes: []*xrayUtils.GraphNode{}}
	topDep2Node := &xrayUtils.GraphNode{Id: "topDep2", Nodes: []*xrayUtils.GraphNode{}}
	topDep1Node := &xrayUtils.GraphNode{Id: "topDep1", Nodes: []*xrayUtils.GraphNode{}}
	rootNode := &xrayUtils.GraphNode{Id: "rootDep", Nodes: []*xrayUtils.GraphNode{}}

	// Setting children to parents
	bottomDep3Node.Nodes = append(bottomDep3Node.Nodes, leafDepNode)
	midDep2Node.Nodes = append(midDep2Node.Nodes, bottomDep3Node)
	midDep2Node.Nodes = append(midDep2Node.Nodes, bottomDep2Node)
	midDep1Node.Nodes = append(midDep1Node.Nodes, bottomDep1Node)
	topDep2Node.Nodes = append(topDep2Node.Nodes, midDep3Node)
	topDep2Node.Nodes = append(topDep2Node.Nodes, midDep2Node)
	topDep1Node.Nodes = append(topDep1Node.Nodes, midDep2Node)
	topDep1Node.Nodes = append(topDep1Node.Nodes, midDep1Node)
	rootNode.Nodes = append(rootNode.Nodes, topDep1Node)
	rootNode.Nodes = append(rootNode.Nodes, topDep2Node)
	rootNode.Nodes = append(rootNode.Nodes, topDep3Node)

	// Setting children to parents
	leafDepNode.Parent = bottomDep3Node
	bottomDep3Node.Parent = midDep2Node
	bottomDep3Node.Parent = midDep2Node
	bottomDep1Node.Parent = midDep1Node
	midDep3Node.Parent = topDep2Node
	midDep2Node.Parent = topDep2Node
	midDep2Node.Parent = topDep1Node
	midDep1Node.Parent = topDep1Node
	topDep1Node.Parent = rootNode
	topDep2Node.Parent = rootNode
	topDep3Node.Parent = rootNode

	tree, uniqueDeps := BuildXrayDependencyTree(treeHelper, "rootDep")

	assert.ElementsMatch(t, expectedUniqueDeps, uniqueDeps)
	assert.True(t, tests.CompareTree(tree, rootNode))
}
