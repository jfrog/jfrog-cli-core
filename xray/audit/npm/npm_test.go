package npm

import (
	"encoding/json"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"os"
	"testing"

	biutils "github.com/jfrog/build-info-go/build/utils"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/stretchr/testify/assert"
)

func TestParseNpmDependenciesList(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, "npm")
	defer cleanUp()
	dependenciesJson, err := os.ReadFile("dependencies.json")
	assert.NoError(t, err)

	var dependencies []buildinfo.Dependency
	err = json.Unmarshal(dependenciesJson, &dependencies)
	assert.NoError(t, err)

	packageInfo := &biutils.PackageInfo{Name: "root", Version: "0.0.0"}
	expectedTree := &xrayUtils.GraphNode{
		Id: "npm://root:0.0.0",
		Nodes: []*xrayUtils.GraphNode{
			{Id: "npm://find:0.2.7",
				Nodes: []*xrayUtils.GraphNode{
					{Id: "npm://nub:1.0.0",
						Nodes: []*xrayUtils.GraphNode{}},
				}},
			{Id: "npm://xml:1.0.1",
				Nodes: []*xrayUtils.GraphNode{}},
			{Id: "npm://jquery:3.2.0",
				Nodes: []*xrayUtils.GraphNode{}},
			{Id: "npm://@jfrog/npm_scoped:1.0.0",
				Nodes: []*xrayUtils.GraphNode{
					{Id: "npm://xpm:0.1.1",
						Nodes: []*xrayUtils.GraphNode{
							{Id: "npm://@ilg/cli-start-options:0.1.19",
								Nodes: []*xrayUtils.GraphNode{
									{Id: "npm://@ilg/es6-promisifier:0.1.9",
										Nodes: []*xrayUtils.GraphNode{}},
									{Id: "npm://wscript-avoider:3.0.2",
										Nodes: []*xrayUtils.GraphNode{}},
								}},
							{Id: "npm://shopify-liquid:1.d7.9",
								Nodes: []*xrayUtils.GraphNode{}},
						}},
				}},
			{Id: "npm://yaml:0.2.3",
				Nodes: []*xrayUtils.GraphNode{}},
			{Id: "npm://nedb:1.0.2",
				Nodes: []*xrayUtils.GraphNode{
					{Id: "npm://async:0.2.10",
						Nodes: []*xrayUtils.GraphNode{}},
					{Id: "npm://binary-search-tree:0.2.4",
						Nodes: []*xrayUtils.GraphNode{
							{Id: "npm://underscore:1.4.4",
								Nodes: []*xrayUtils.GraphNode{}},
						}},
				}},
		},
	}

	xrayDependenciesTree := parseNpmDependenciesList(dependencies, packageInfo)

	equals := tests.CompareTree(expectedTree, xrayDependenciesTree)
	if !equals {
		t.Error("expected:", expectedTree.Nodes, "got:", xrayDependenciesTree.Nodes)
	}

}

func TestIgnoreScripts(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, "npm-scripts")
	defer cleanUp()

	// The package.json file contain a postinstall script running an "exit 1" command.
	// Without the "--ignore-scripts" flag, the test will fail.
	_, err := BuildDependencyTree([]string{})
	assert.NoError(t, err)
}
