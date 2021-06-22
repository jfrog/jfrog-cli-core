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
	_, cleanUp := createTestWorkspace(t, "gradle-example-ci-server")
	defer cleanUp()

	// Run getModulesDependencyTrees
	auditCmd := NewXrAuditGradleCommand()
	auditCmd.useWrapper = true
	modulesDependencyTrees, err := auditCmd.getModulesDependencyTrees()
	assert.NoError(t, err)
	assert.Len(t, modulesDependencyTrees, 5)

	// Check module
	moduleApi := getAndAssertNode(t, modulesDependencyTrees, "org.jfrog.example.gradle:api:1.0")
	assert.Len(t, moduleApi.Nodes, 3)

	// Check direct dependency
	commonsLang := getAndAssertNode(t, moduleApi.Nodes, "commons-lang:commons-lang:2.4")
	assert.Len(t, commonsLang.Nodes, 1)

	// Check transitive dependency
	getAndAssertNode(t, commonsLang.Nodes, "org.slf4j:slf4j-api:1.4.2")
}

func TestGradleTreesWithConfig(t *testing.T) {
	// Create and change directory to test workspace
	tempDirPath, cleanUp := createTestWorkspace(t, "artifactory-client-java")
	defer cleanUp()
	err := os.Chmod(filepath.Join(tempDirPath, "gradlew"), 0700)
	assert.NoError(t, err)

	// Run getModulesDependencyTrees
	auditCmd := NewXrAuditGradleCommand()
	auditCmd.useWrapper = true
	modulesDependencyTrees, err := auditCmd.getModulesDependencyTrees()
	assert.NoError(t, err)
	assert.Len(t, modulesDependencyTrees, 3)

	// Check direct dependency
	moduleServices := getAndAssertNode(t, modulesDependencyTrees, "org.jfrog.artifactory.client:artifactory-java-client-services:2.9.x-SNAPSHOT")
	assert.Len(t, moduleServices.Nodes, 12)

	// Check transitive dependency
	httpClient := getAndAssertNode(t, moduleServices.Nodes, "org.apache.httpcomponents:httpclient:4.5.13")
	assert.Len(t, httpClient.Nodes, 2)
}

func TestGradleTreesWithConfigExcludeTestDeps(t *testing.T) {
	// Create and change directory to test workspace
	tempDirPath, cleanUp := createTestWorkspace(t, "artifactory-client-java")
	defer cleanUp()
	err := os.Chmod(filepath.Join(tempDirPath, "gradlew"), 0700)
	assert.NoError(t, err)

	// Run getModulesDependencyTrees
	auditCmd := NewXrAuditGradleCommand()
	auditCmd.useWrapper = true
	auditCmd.excludeTestDeps = true
	modulesDependencyTrees, err := auditCmd.getModulesDependencyTrees()
	assert.NoError(t, err)
	assert.Len(t, modulesDependencyTrees, 3)

	// Check direct dependency
	moduleServices := getAndAssertNode(t, modulesDependencyTrees, "org.jfrog.artifactory.client:artifactory-java-client-services:2.9.x-SNAPSHOT")
	assert.Len(t, moduleServices.Nodes, 11)

	// Check transitive dependency
	httpClient := getAndAssertNode(t, moduleServices.Nodes, "org.apache.httpcomponents:httpclient:4.5.13")
	assert.Len(t, httpClient.Nodes, 2)
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
	for _, module := range modules {
		if module.Id == GavPackageTypeIdentifier+moduleId {
			return module
		}
	}
	assert.Fail(t, "Module '"+moduleId+"' doesn't exist")
	return nil
}
