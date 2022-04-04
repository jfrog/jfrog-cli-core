package nuget

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/stretchr/testify/assert"

	"github.com/jfrog/jfrog-client-go/xray/services"
)

func TestBuildNugetDependencyTree(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, "nuget")
	defer cleanUp()
	dependenciesJson, err := ioutil.ReadFile("dependencies.json")
	assert.NoError(t, err)

	var dependencies *entities.BuildInfo
	err = json.Unmarshal(dependenciesJson, &dependencies)
	assert.NoError(t, err)
	xrayDependenciesTree := parseNugetDependencyTree(dependencies)

	expectedTreeJson, err := ioutil.ReadFile("expectedTree.json")
	assert.NoError(t, err)

	var expectedTrees *[]services.GraphNode
	err = json.Unmarshal(expectedTreeJson, &expectedTrees)
	assert.NoError(t, err)

	for i, expectedTree := range *expectedTrees {
		equals := tests.CompareTree(&expectedTree, xrayDependenciesTree[i])
		if !equals {
			t.Error("expected:", expectedTree.Nodes, "got:", xrayDependenciesTree[i].Nodes)
		}
	}
}
