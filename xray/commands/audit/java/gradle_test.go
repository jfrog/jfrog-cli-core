package java

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGradleTreesWithoutConfig(t *testing.T) {
	// Create and change directory to test workspace
	tempDirPath, cleanUp := audit.CreateTestWorkspace(t, "gradle-example-ci-server")
	defer cleanUp()
	err := os.Chmod(filepath.Join(tempDirPath, "gradlew"), 0700)
	assert.NoError(t, err)

	// Run getModulesDependencyTrees
	auditCmd := NewEmptyAuditGradleCommand()
	auditCmd.useWrapper = true
	modulesDependencyTrees, err := auditCmd.getModulesDependencyTrees()
	assert.NoError(t, err)
	assert.Len(t, modulesDependencyTrees, 5)

	// Check module
	module := audit.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.example.gradle:webservice:1.0")
	assert.Len(t, module.Nodes, 7)

	// Check direct dependency
	directDependency := audit.GetAndAssertNode(t, module.Nodes, "junit:junit:4.11")
	assert.Len(t, directDependency.Nodes, 1)

	// Check transitive dependency
	audit.GetAndAssertNode(t, directDependency.Nodes, "org.hamcrest:hamcrest-core:1.3")
}

func TestGradleTreesWithConfig(t *testing.T) {
	// Create and change directory to test workspace
	tempDirPath, cleanUp := audit.CreateTestWorkspace(t, "gradle-example-publish")
	defer cleanUp()
	err := os.Chmod(filepath.Join(tempDirPath, "gradlew"), 0700)
	assert.NoError(t, err)

	// Run getModulesDependencyTrees
	auditCmd := NewEmptyAuditGradleCommand()
	modulesDependencyTrees, err := auditCmd.getModulesDependencyTrees()
	assert.NoError(t, err)
	assert.Len(t, modulesDependencyTrees, 3)

	// Check module
	module := audit.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test.gradle.publish:webservice:1.0-SNAPSHOT")
	assert.Len(t, module.Nodes, 7)

	// Check direct dependency
	directDependency := audit.GetAndAssertNode(t, module.Nodes, "org.apache.wicket:wicket:1.3.7")
	assert.Len(t, directDependency.Nodes, 1)

	// Check transitive dependency
	audit.GetAndAssertNode(t, directDependency.Nodes, "org.slf4j:slf4j-api:1.4.2")
}

func TestGradleTreesExcludeTestDeps(t *testing.T) {
	// Create and change directory to test workspace
	tempDirPath, cleanUp := audit.CreateTestWorkspace(t, "gradle-example-ci-server")
	defer cleanUp()
	err := os.Chmod(filepath.Join(tempDirPath, "gradlew"), 0700)
	assert.NoError(t, err)

	// Run getModulesDependencyTrees
	auditCmd := NewEmptyAuditGradleCommand()
	auditCmd.useWrapper = true
	auditCmd.excludeTestDeps = true
	modulesDependencyTrees, err := auditCmd.getModulesDependencyTrees()
	assert.NoError(t, err)
	assert.Len(t, modulesDependencyTrees, 5)

	// Check module
	module := audit.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.example.gradle:webservice:1.0")
	assert.Len(t, module.Nodes, 6)
	assert.Nil(t, audit.GetModule(module.Nodes, "junit:junit:4.11"))

	// Check direct dependency
	directDependency := audit.GetAndAssertNode(t, module.Nodes, "org.apache.wicket:wicket:1.3.7")
	assert.Len(t, directDependency.Nodes, 1)

	// Check transitive dependency
	audit.GetAndAssertNode(t, directDependency.Nodes, "org.slf4j:slf4j-api:1.4.2")
}
