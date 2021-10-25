package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildPipDependencyList(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := createTestWorkspace(t, "pip-project")
	defer cleanUp()
	// Run getModulesDependencyTrees
	auditCmd := NewEmptyAuditPipCommand()
	rootNodes, err := auditCmd.buildPipDependencyTree()
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEmpty(t, rootNodes)

	// Test root module
	rootNode := getAndAssertNode(t, rootNodes, "pip-example:1.2.3")
	// Test child module
	childNode := getAndAssertNode(t, rootNode.Nodes, "pexpect:4.8.0")
	// Test sub child module
	getAndAssertNode(t, childNode.Nodes, "ptyprocess:0.7.0")
}
