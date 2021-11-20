package audit

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

func TestParseNpmDependenciesList(t *testing.T) {
	dependenciesJson, err := ioutil.ReadFile("../../../utils/testdata/dependencies.json")
	if err != nil {
		t.Error(err)
	}
	dependencies := make(map[string]*npmutils.Dependency)
	err = json.Unmarshal(dependenciesJson, &dependencies)
	if err != nil {
		t.Error(err)
	}
	packageInfo := &npmutils.PackageInfo{Name: "root", Version: "0.0.0"}
	expectedTree := &services.GraphNode{
		Id: "npm://root:0.0.0",
		Nodes: []*services.GraphNode{
			{Id: "npm://find:0.2.7",
				Nodes: []*services.GraphNode{
					{Id: "npm://nub:1.0.0",
						Nodes: []*services.GraphNode{}},
				}},
			{Id: "npm://xml:1.0.1",
				Nodes: []*services.GraphNode{}},
			{Id: "npm://jquery:3.2.0",
				Nodes: []*services.GraphNode{}},
			{Id: "npm://@jfrog/npm_scoped:1.0.0",
				Nodes: []*services.GraphNode{
					{Id: "npm://xpm:0.1.1",
						Nodes: []*services.GraphNode{
							{Id: "npm://@ilg/cli-start-options:0.1.19",
								Nodes: []*services.GraphNode{
									{Id: "npm://@ilg/es6-promisifier:0.1.9",
										Nodes: []*services.GraphNode{}},
									{Id: "npm://wscript-avoider:3.0.2",
										Nodes: []*services.GraphNode{}},
								}},
							{Id: "npm://shopify-liquid:1.d7.9",
								Nodes: []*services.GraphNode{}},
						}},
				}},
			{Id: "npm://yaml:0.2.3",
				Nodes: []*services.GraphNode{}},
			{Id: "npm://nedb:1.0.2",
				Nodes: []*services.GraphNode{
					{Id: "npm://async:0.2.10",
						Nodes: []*services.GraphNode{}},
					{Id: "npm://binary-search-tree:0.2.4",
						Nodes: []*services.GraphNode{
							{Id: "npm://underscore:1.4.4",
								Nodes: []*services.GraphNode{}},
						}},
				}},
		},
	}

	xrayDependenciesTree := parseNpmDependenciesList(dependencies, packageInfo)

	equals := compareTree(expectedTree, xrayDependenciesTree)
	if !equals {
		t.Error("expected:", expectedTree.Nodes, "got:", xrayDependenciesTree.Nodes)
	}

}

func compareTree(a, b *services.GraphNode) bool {
	if a.Id != b.Id {
		return false
	}
	// Make sure all children are equal, when order doesn't matter
	for _, nodeA := range a.Nodes {
		found := false
		for _, nodeB := range b.Nodes {
			if compareTree(nodeA, nodeB) {
				found = true
				break
			}
		}
		// After iterating over all B's nodes, non match nodeA so the tree aren't equals.
		if !found {
			return false
		}
	}
	return true
}
