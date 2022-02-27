package python

import (
	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func TestBuildPipDependencyListSetuppy(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, filepath.Join("pip-project", "setuppyproject"))
	defer cleanUp()
	// Run getModulesDependencyTrees
	auditCmd := NewEmptyPythonCommand(pythonutils.Pip)
	dependencyTree, err := auditCmd.buildDependencyTree()
	if assert.NoError(t, err) && assert.NotNil(t, dependencyTree) {
		assert.NotEmpty(t, dependencyTree.Nodes)
		// Test root module
		rootNode := audit.GetAndAssertNode(t, dependencyTree.Nodes, "pip-example:1.2.3")
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
	auditCmd := NewEmptyPythonCommand(pythonutils.Pip)
	rootNodes, err := auditCmd.buildDependencyTree()
	if assert.NoError(t, err) && assert.NotEmpty(t, rootNodes) {
		if rootNodes != nil {
			// Test root module
			rootNode := audit.GetAndAssertNode(t, rootNodes.Nodes, "pexpect:4.8.0")
			// Test child module
			audit.GetAndAssertNode(t, rootNode.Nodes, "ptyprocess:0.7.0")
		}
	}
}

func TestBuildPipenvDependencyList(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, "pipenv-project")
	defer cleanUp()
	// Run getModulesDependencyTrees
	auditCmd := NewEmptyPythonCommand(pythonutils.Pipenv)
	dependencyTree, err := auditCmd.buildDependencyTree()
	if assert.NoError(t, err) && assert.NotNil(t, dependencyTree) {
		assert.NotEmpty(t, dependencyTree.Nodes)
		// Test child module
		audit.GetAndAssertNode(t, dependencyTree.Nodes, "toml:0.10.2")

		// Test child module
		childNode := audit.GetAndAssertNode(t, dependencyTree.Nodes, "pexpect:4.8.0")
		// Test sub child module
		audit.GetAndAssertNode(t, childNode.Nodes, "ptyprocess:0.7.0")

	}
}
