package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
)

func TestMavenTreesMultiModule(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := createTestWorkspace(t, "maven-example")
	defer cleanUp()

	// Run getModulesDependencyTrees
	auditCmd := NewXrAuditMvnCommand()
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
	auditCmd := NewXrAuditMvnCommand()
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

func TestGradleTreesWithoutConfig(t *testing.T) {
	// Create and change directory to test workspace
	tempDirPath, cleanUp := createTestWorkspace(t, "gradle-example-ci-server")
	defer cleanUp()
	err := os.Chmod(filepath.Join(tempDirPath, "gradlew"), 0700)
	assert.NoError(t, err)

	// Run getModulesDependencyTrees
	auditCmd := NewXrAuditGradleCommand()
	auditCmd.useWrapper = true
	modulesDependencyTrees, err := auditCmd.getModulesDependencyTrees()
	assert.NoError(t, err)
	assert.Len(t, modulesDependencyTrees, 5)

	// Check module
	module := getAndAssertNode(t, modulesDependencyTrees, "org.jfrog.example.gradle:webservice:1.0")
	assert.Len(t, module.Nodes, 7)
	assert.NotNil(t, getModule(t, module.Nodes, "junit:junit:4.11"))

	// Check direct dependency
	directDependency := getAndAssertNode(t, module.Nodes, "org.apache.wicket:wicket:1.3.7")
	assert.Len(t, directDependency.Nodes, 1)

	// Check transitive dependency
	getAndAssertNode(t, directDependency.Nodes, "org.hamcrest:hamcrest-core:1.3")
}

func TestGradleTreesWithConfig(t *testing.T) {
	// Create and change directory to test workspace
	tempDirPath, cleanUp := createTestWorkspace(t, "gradle-example-publish")
	defer cleanUp()
	err := os.Chmod(filepath.Join(tempDirPath, "gradlew"), 0700)
	assert.NoError(t, err)

	// Run getModulesDependencyTrees
	auditCmd := NewXrAuditGradleCommand()
	modulesDependencyTrees, err := auditCmd.getModulesDependencyTrees()
	assert.NoError(t, err)
	assert.Len(t, modulesDependencyTrees, 3)

	// Check module
	module := getAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test.gradle.publish:webservice:1.0-SNAPSHOT")
	assert.Len(t, module.Nodes, 7)

	// Check direct dependency
	directDependency := getAndAssertNode(t, module.Nodes, "org.apache.wicket:wicket:1.3.7")
	assert.Len(t, directDependency.Nodes, 1)

	// Check transitive dependency
	getAndAssertNode(t, directDependency.Nodes, "junit:junit:4.7")
}

func TestGradleTreesExcludeTestDeps(t *testing.T) {
	// Create and change directory to test workspace
	tempDirPath, cleanUp := createTestWorkspace(t, "gradle-example-ci-server")
	defer cleanUp()
	err := os.Chmod(filepath.Join(tempDirPath, "gradlew"), 0700)
	assert.NoError(t, err)

	// Run getModulesDependencyTrees
	auditCmd := NewXrAuditGradleCommand()
	auditCmd.useWrapper = true
	auditCmd.excludeTestDeps = true
	modulesDependencyTrees, err := auditCmd.getModulesDependencyTrees()
	assert.NoError(t, err)
	assert.Len(t, modulesDependencyTrees, 5)

	// Check module
	module := getAndAssertNode(t, modulesDependencyTrees, "org.jfrog.example.gradle:webservice:1.0")
	assert.Len(t, module.Nodes, 6)
	assert.Nil(t, getModule(t, module.Nodes, "junit:junit:4.11"))

	// Check direct dependency
	directDependency := getAndAssertNode(t, module.Nodes, "org.apache.wicket:wicket:1.3.7")
	assert.Len(t, directDependency.Nodes, 1)

	// Check transitive dependency
	getAndAssertNode(t, directDependency.Nodes, "org.slf4j:slf4j-api:1.4.2")
}

func createTestWorkspace(t *testing.T, sourceDir string) (string, func()) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)
	tempDirPath, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	err = fileutils.CopyDir(filepath.Join("..", "testdata", sourceDir), tempDirPath, true, nil)
	assert.NoError(t, err)
	err = os.Chdir(tempDirPath)
	assert.NoError(t, err)
	return tempDirPath, func() {
		assert.NoError(t, os.Chdir(cwd))
		assert.NoError(t, fileutils.RemoveTempDir(tempDirPath))
	}
}

func getAndAssertNode(t *testing.T, modules []*services.GraphNode, moduleId string) *services.GraphNode {
	module := getModule(t, modules, moduleId)
	assert.NotNil(t, module, "Module '"+moduleId+"' doesn't exist")
	return module
}

func getModule(t *testing.T, modules []*services.GraphNode, moduleId string) *services.GraphNode {
	for _, module := range modules {
		if module.Id == GavPackageTypeIdentifier+moduleId {
			return module
		}
	}
	return nil
}
