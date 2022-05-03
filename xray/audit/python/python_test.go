package python

import (
	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func TestBuildPipDependencyListSetuppy(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, filepath.Join("pip-project", "setuppyproject"))
	defer cleanUp()
	// Run getModulesDependencyTrees
	rootNodes, err := BuildDependencyTree(pythonutils.Pip)
	assert.NoError(t, err)
	assert.NotEmpty(t, rootNodes)
	if rootNodes != nil {
		// Test root module
		rootNode := audit.GetAndAssertNode(t, rootNodes, "pip-example:1.2.3")
		// Test child module
		childNode := audit.GetAndAssertNode(t, rootNode.Nodes, "pexpect:4.8.0")
		// Test sub child module
		audit.GetAndAssertNode(t, childNode.Nodes, "ptyprocess:0.7.0")
	}
}

func TestBuildPipDependencyListRequirements(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, filepath.Join("pip-project", "requirementsproject"))
	defer cleanUp()
	// Run getModulesDependencyTrees
	rootNodes, err := BuildDependencyTree(pythonutils.Pip)
	assert.NoError(t, err)
	assert.NotEmpty(t, rootNodes)
	if rootNodes != nil {
		// Test root module
		rootNode := audit.GetAndAssertNode(t, rootNodes, "pexpect:4.8.0")
		// Test child module
		audit.GetAndAssertNode(t, rootNode.Nodes, "ptyprocess:0.7.0")
	}
}

func TestBuildPipenvDependencyList(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, "pipenv-project")
	defer cleanUp()
	// Run getModulesDependencyTrees
	rootNodes, err := BuildDependencyTree(pythonutils.Pipenv)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEmpty(t, rootNodes)

	// Test child module
	childNode := audit.GetAndAssertNode(t, rootNodes, "pexpect:4.8.0")
	// Test sub child module
	audit.GetAndAssertNode(t, childNode.Nodes, "ptyprocess:0.7.0")
}
