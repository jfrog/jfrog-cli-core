package nuget

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"

	"github.com/jfrog/jfrog-client-go/xray/services"
)

func TestBuildNugetDependencyTree(t *testing.T) {
	dependenciesJson, err := ioutil.ReadFile("../../testdata/nuget/dependencies.json")
	if err != nil {
		t.Error(err)
	}
	var dependencies *entities.BuildInfo
	err = json.Unmarshal(dependenciesJson, &dependencies)
	if err != nil {
		t.Error(err)
	}

	xrayDependenciesTree := buildNugetDependencyTree(dependencies)

	expectedTreeJson, err := ioutil.ReadFile("../../testdata/nuget/expectedTree.json")
	if err != nil {
		t.Error(err)
	}
	var expectedTrees *[]services.GraphNode
	err = json.Unmarshal(expectedTreeJson, &expectedTrees)
	if err != nil {
		t.Error(err)
	}

	for i, expectedTree := range *expectedTrees {
		equals := tests.CompareTree(&expectedTree, xrayDependenciesTree[i])
		if !equals {
			t.Error("expected:", expectedTree.Nodes, "got:", xrayDependenciesTree[i].Nodes)
		}
	}

}
