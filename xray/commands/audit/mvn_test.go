package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMavenTreesMultiModule(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := createTestWorkspace(t, "maven-example")
	defer cleanUp()

	// Run getModulesDependencyTrees
	auditCmd := NewAuditMvnCommand()
	modulesDependencyTrees, err := auditCmd.getModulesDependencyTrees()
	assert.NoError(t, err)
	assert.NotEmpty(t, modulesDependencyTrees)

	// Check root module
	multi := getAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi:3.7-SNAPSHOT")
	assert.Empty(t, multi.Nodes)

	// Check multi1 with a transitive dependency
	multi1 := getAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi1:3.7-SNAPSHOT")
	assert.Len(t, multi1.Nodes, 7)
	commonsEmail := getAndAssertNode(t, multi1.Nodes, "org.apache.commons:commons-email:1.1")
	assert.Len(t, commonsEmail.Nodes, 2)

	// Check multi2 and multi3
	multi2 := getAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi2:3.7-SNAPSHOT")
	assert.Len(t, multi2.Nodes, 1)
	multi3 := getAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi3:3.7-SNAPSHOT")
	assert.Len(t, multi3.Nodes, 4)
}

func TestMavenTreesExcludeTestDeps(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := createTestWorkspace(t, "artifactory-maven-plugin")
	defer cleanUp()

	// Run getModulesDependencyTrees
	auditCmd := NewAuditMvnCommand()
	modulesDependencyTrees, err := auditCmd.getModulesDependencyTrees()
	assert.NoError(t, err)
	assert.NotEmpty(t, modulesDependencyTrees)

	// Assert module
	module := getAndAssertNode(t, modulesDependencyTrees, "org.jfrog.buildinfo:artifactory-maven-plugin:3.2.3")
	assert.Len(t, module.Nodes, 12)

	// Assert direct and transitive dependencies
	directDependency := getAndAssertNode(t, module.Nodes, "org.slf4j:slf4j-simple:1.7.30")
	assert.Len(t, directDependency.Nodes, 1)
	getAndAssertNode(t, directDependency.Nodes, "org.eclipse.sisu:org.eclipse.sisu.plexus:0.3.2")

	// Run getModulesDependencyTrees
	auditCmd.excludeTestDeps = true
	modulesDependencyTrees, err = auditCmd.getModulesDependencyTrees()
	assert.NoError(t, err)
	assert.NotEmpty(t, modulesDependencyTrees)

	// Assert module
	module = getAndAssertNode(t, modulesDependencyTrees, "org.jfrog.buildinfo:artifactory-maven-plugin:3.2.3")
	assert.Len(t, module.Nodes, 11)
}
