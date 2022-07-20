package python

import (
	"path/filepath"
	"testing"

	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/stretchr/testify/assert"
)

func TestBuildPipDependencyListSetuppy(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, filepath.Join("pip-project", "setuppyproject"))
	defer cleanUp()
	// Run getModulesDependencyTrees
	rootNodes, err := BuildDependencyTree(pythonutils.Pip, "")
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
	rootNodes, err := BuildDependencyTree(pythonutils.Pip, "requirements.txt")
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
	rootNodes, err := BuildDependencyTree(pythonutils.Pipenv, "")
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEmpty(t, rootNodes)

	// Test child module
	childNode := audit.GetAndAssertNode(t, rootNodes, "pexpect:4.8.0")
	// Test sub child module
	audit.GetAndAssertNode(t, childNode.Nodes, "ptyprocess:0.7.0")
}

func TestBuildPoetryDependencyList(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, "poetry-project")
	defer cleanUp()
	// Run getModulesDependencyTrees
	rootNodes, err := BuildDependencyTree(pythonutils.Poetry, "")
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEmpty(t, rootNodes)

	// Test child module
	childNode := audit.GetAndAssertNode(t, rootNodes, "pytest:5.4.3")
	// Test sub child module
	if assert.NotNil(t, childNode) {
		transitiveChildNode := audit.GetAndAssertNode(t, childNode.Nodes, "packaging:21.3")
		if assert.NotNil(t, transitiveChildNode) {
			audit.GetAndAssertNode(t, transitiveChildNode.Nodes, "pyparsing:3.0.9")
		}
	}

}
