package npm

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	biutils "github.com/jfrog/build-info-go/build/utils"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/stretchr/testify/assert"

	"github.com/jfrog/jfrog-client-go/xray/services"
)

func TestParseNpmDependenciesList(t *testing.T) {
	dependenciesJson, err := ioutil.ReadFile("../../testdata/npm/dependencies.json")
	assert.NoError(t, err)

	var dependencies []buildinfo.Dependency
	err = json.Unmarshal(dependenciesJson, &dependencies)
	assert.NoError(t, err)

	packageInfo := &biutils.PackageInfo{Name: "root", Version: "0.0.0"}
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

	equals := tests.CompareTree(expectedTree, xrayDependenciesTree)
	if !equals {
		t.Error("expected:", expectedTree.Nodes, "got:", xrayDependenciesTree.Nodes)
	}

}
