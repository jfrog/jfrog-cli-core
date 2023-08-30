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
	rootNode, uniqueDeps, err := BuildDependencyTree(&AuditPython{Tool: pythonutils.Pip})
	assert.NoError(t, err)
	assert.Contains(t, uniqueDeps, pythonPackageTypeIdentifier+"pexpect:4.8.0")
	assert.Contains(t, uniqueDeps, pythonPackageTypeIdentifier+"ptyprocess:0.7.0")
	assert.Contains(t, uniqueDeps, pythonPackageTypeIdentifier+"pip-example:1.2.3")
	assert.Len(t, rootNode, 1)
	if len(rootNode) > 0 {
		assert.NotEmpty(t, rootNode[0].Nodes)
		if rootNode[0].Nodes != nil {
			// Test direct dependency
			directDepNode := audit.GetAndAssertNode(t, rootNode[0].Nodes, "pip-example:1.2.3")
			// Test child module
			childNode := audit.GetAndAssertNode(t, directDepNode.Nodes, "pexpect:4.8.0")
			// Test sub child module
			audit.GetAndAssertNode(t, childNode.Nodes, "ptyprocess:0.7.0")
		}
	}
}

func TestPipDependencyListRequirementsFallback(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, filepath.Join("pip-project", "requirementsproject"))
	defer cleanUp()
	// No requirements file field specified, expect the command to use the fallback 'pip install -r requirements.txt' command
	rootNode, uniqueDeps, err := BuildDependencyTree(&AuditPython{Tool: pythonutils.Pip})
	assert.NoError(t, err)
	assert.Contains(t, uniqueDeps, pythonPackageTypeIdentifier+"pexpect:4.7.0")
	assert.Contains(t, uniqueDeps, pythonPackageTypeIdentifier+"ptyprocess:0.7.0")
	assert.Len(t, rootNode, 1)
	if assert.True(t, len(rootNode[0].Nodes) > 2) {
		childNode := audit.GetAndAssertNode(t, rootNode[0].Nodes, "pexpect:4.7.0")
		if childNode != nil {
			// Test child module
			audit.GetAndAssertNode(t, childNode.Nodes, "ptyprocess:0.7.0")
		}
	}
}

func TestBuildPipDependencyListRequirements(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, filepath.Join("pip-project", "requirementsproject"))
	defer cleanUp()
	// Run getModulesDependencyTrees
	rootNode, uniqueDeps, err := BuildDependencyTree(&AuditPython{Tool: pythonutils.Pip, PipRequirementsFile: "requirements.txt"})
	assert.NoError(t, err)
	assert.Contains(t, uniqueDeps, "pexpect:4.7.0")
	assert.Contains(t, uniqueDeps, "ptyprocess:0.7.0")
	assert.Len(t, rootNode, 1)
	if len(rootNode) > 0 {
		assert.NotEmpty(t, rootNode[0].Nodes)
		if rootNode[0].Nodes != nil {
			// Test root module
			directDepNode := audit.GetAndAssertNode(t, rootNode[0].Nodes, "pexpect:4.7.0")
			// Test child module
			audit.GetAndAssertNode(t, directDepNode.Nodes, "ptyprocess:0.7.0")
		}
	}
}

func TestBuildPipenvDependencyList(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, "pipenv-project")
	defer cleanUp()
	expectedPipenvUniqueDeps := []string{
		pythonPackageTypeIdentifier + "toml:0.10.2",
		pythonPackageTypeIdentifier + "pexpect:4.8.0",
		pythonPackageTypeIdentifier + "ptyprocess:0.7.0",
	}
	// Run getModulesDependencyTrees
	rootNode, uniqueDeps, err := BuildDependencyTree(&AuditPython{Tool: pythonutils.Pipenv})
	if err != nil {
		t.Fatal(err)
	}
	assert.ElementsMatch(t, uniqueDeps, expectedPipenvUniqueDeps, "First is actual, Second is Expected")
	assert.Len(t, rootNode, 1)
	if len(rootNode) > 0 {
		assert.NotEmpty(t, rootNode[0].Nodes)
		// Test child module
		childNode := audit.GetAndAssertNode(t, rootNode[0].Nodes, "pexpect:4.8.0")
		// Test sub child module
		if assert.NotNil(t, childNode) {
			audit.GetAndAssertNode(t, childNode.Nodes, "ptyprocess:0.7.0")
		}
	}
}

func TestBuildPoetryDependencyList(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, "poetry-project")
	defer cleanUp()
	expectedPoetryUniqueDeps := []string{
		pythonPackageTypeIdentifier + "wcwidth:0.2.5",
		pythonPackageTypeIdentifier + "colorama:0.4.6",
		pythonPackageTypeIdentifier + "packaging:22.0",
		pythonPackageTypeIdentifier + "python:",
		pythonPackageTypeIdentifier + "pluggy:0.13.1",
		pythonPackageTypeIdentifier + "py:1.11.0",
		pythonPackageTypeIdentifier + "atomicwrites:1.4.1",
		pythonPackageTypeIdentifier + "attrs:22.1.0",
		pythonPackageTypeIdentifier + "more-itertools:9.0.0",
		pythonPackageTypeIdentifier + "numpy:1.24.0",
		pythonPackageTypeIdentifier + "pytest:5.4.3",
	}
	// Run getModulesDependencyTrees
	rootNode, uniqueDeps, err := BuildDependencyTree(&AuditPython{Tool: pythonutils.Poetry})
	if err != nil {
		t.Fatal(err)
	}
	assert.ElementsMatch(t, uniqueDeps, expectedPoetryUniqueDeps, "First is actual, Second is Expected")
	assert.Len(t, rootNode, 1)
	if len(rootNode) > 0 {
		assert.NotEmpty(t, rootNode[0].Nodes)
		// Test child module
		childNode := audit.GetAndAssertNode(t, rootNode[0].Nodes, "pytest:5.4.3")
		// Test sub child module
		if assert.NotNil(t, childNode) {
			audit.GetAndAssertNode(t, childNode.Nodes, "packaging:22.0")
		}
	}
}

func TestGetPipInstallArgs(t *testing.T) {
	assert.Equal(t, []string{"-m", "pip", "install", "."}, getPipInstallArgs(""))
	assert.Equal(t, []string{"-m", "pip", "install", "-r", "requirements.txt"}, getPipInstallArgs("requirements.txt"))
}
