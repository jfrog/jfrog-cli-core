package java

import (
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"

	"github.com/stretchr/testify/assert"
)

func TestMavenTreesMultiModule(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, "maven-example")
	defer cleanUp()

	// Run getModulesDependencyTrees
	modulesDependencyTrees, err := BuildMvnDependencyTree(false)
	assert.NoError(t, err)
	assert.NotEmpty(t, modulesDependencyTrees)

	// Check root module
	multi := audit.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi:3.7-SNAPSHOT")
	if multi == nil {
		t.FailNow()
	}
	assert.Empty(t, multi.Nodes)

	// Check multi1 with a transitive dependency
	multi1 := audit.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi1:3.7-SNAPSHOT")
	assert.Len(t, multi1.Nodes, 7)
	commonsEmail := audit.GetAndAssertNode(t, multi1.Nodes, "org.apache.commons:commons-email:1.1")
	assert.Len(t, commonsEmail.Nodes, 2)

	// Check multi2 and multi3
	multi2 := audit.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi2:3.7-SNAPSHOT")
	assert.Len(t, multi2.Nodes, 1)
	multi3 := audit.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi3:3.7-SNAPSHOT")
	assert.Len(t, multi3.Nodes, 4)
}
