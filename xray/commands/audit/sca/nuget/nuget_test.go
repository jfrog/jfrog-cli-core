package nuget

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"os"
	"testing"

	"github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/stretchr/testify/assert"
)

func TestBuildNugetDependencyTree(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := sca.CreateTestWorkspace(t, "nuget")
	defer cleanUp()
	dependenciesJson, err := os.ReadFile("dependencies.json")
	assert.NoError(t, err)

	var dependencies *entities.BuildInfo
	err = json.Unmarshal(dependenciesJson, &dependencies)
	assert.NoError(t, err)
	expectedUniqueDeps := []string{
		nugetPackageTypeIdentifier + "Microsoft.Net.Http:2.2.29",
		nugetPackageTypeIdentifier + "Microsoft.Bcl:1.1.10",
		nugetPackageTypeIdentifier + "Microsoft.Bcl.Build:1.0.14",
		nugetPackageTypeIdentifier + "Newtonsoft.Json:11.0.2",
		nugetPackageTypeIdentifier + "NUnit:3.10.1",
		nugetPackageTypeIdentifier + "bootstrap:4.1.1",
		nugetPackageTypeIdentifier + "popper.js:1.14.0",
		nugetPackageTypeIdentifier + "jQuery:3.0.0",
		nugetPackageTypeIdentifier + "MsbuildExample",
		nugetPackageTypeIdentifier + "MsbuildLibrary",
	}
	xrayDependenciesTree, uniqueDeps := parseNugetDependencyTree(dependencies)
	assert.ElementsMatch(t, uniqueDeps, expectedUniqueDeps, "First is actual, Second is Expected")
	expectedTreeJson, err := os.ReadFile("expectedTree.json")
	assert.NoError(t, err)

	var expectedTrees *[]xrayUtils.GraphNode
	err = json.Unmarshal(expectedTreeJson, &expectedTrees)
	assert.NoError(t, err)

	for i := range *expectedTrees {
		expectedTree := &(*expectedTrees)[i]
		assert.True(t, tests.CompareTree(expectedTree, xrayDependenciesTree[i]), "expected:", expectedTree.Nodes, "got:", xrayDependenciesTree[i].Nodes)
	}
}
