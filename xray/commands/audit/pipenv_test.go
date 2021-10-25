package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildPipenvDependencyList(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := createTestWorkspace(t, "pipenv-project")
	defer cleanUp()
	// Run getModulesDependencyTrees
	auditCmd := NewEmptyAuditPipenvCommand()
	rootNode, err := auditCmd.buildPipenvDependencyTree()
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEmpty(t, rootNode)

	// Test root module
	someNode := getAndAssertNode(t, rootNode.Nodes, "delegator.py:0.1.1")
	// Test child module
	childNode := getAndAssertNode(t, someNode.Nodes, "pexpect:4.8.0")
	// Test sub child module
	getAndAssertNode(t, childNode.Nodes, "ptyprocess:0.7.0")
}
