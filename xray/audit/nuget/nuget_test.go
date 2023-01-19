package nuget

import (
	"encoding/json"
	"os"
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
	dependenciesJson, err := os.ReadFile("dependencies.json")
	assert.NoError(t, err)

	var dependencies *entities.BuildInfo
	err = json.Unmarshal(dependenciesJson, &dependencies)
	assert.NoError(t, err)
	xrayDependenciesTree := parseNugetDependencyTree(dependencies)

	expectedTreeJson, err := os.ReadFile("expectedTree.json")
	assert.NoError(t, err)

	var expectedTrees *[]services.GraphNode
	err = json.Unmarshal(expectedTreeJson, &expectedTrees)
	assert.NoError(t, err)

	for i := range *expectedTrees {
		expectedTree := &(*expectedTrees)[i]
		assert.True(t, tests.CompareTree(expectedTree, xrayDependenciesTree[i]), "expected:", expectedTree.Nodes, "got:", xrayDependenciesTree[i].Nodes)
	}
}
