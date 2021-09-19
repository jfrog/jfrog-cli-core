package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildPipDependencyList(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := createTestWorkspace(t, "pip-example")
	defer cleanUp()
	// Run getModulesDependencyTrees
	auditCmd := NewAuditPipCommand()
	parentNodes, err := auditCmd.buildPipDependencyTree()
	assert.NoError(t, err)
	assert.NotEmpty(t, parentNodes)

	// Test root module
	rootNode := getAndAssertNode(t, parentNodes, "pip-example:1.2.3")
	// Test child module
	childNode := getAndAssertNode(t, rootNode.Nodes, "pexpect:4.8.0")
	// Test sub child module
	getAndAssertNode(t, childNode.Nodes, "ptyprocess:0.7.0")
}
