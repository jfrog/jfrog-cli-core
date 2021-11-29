package python

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBuildPipDependencyList(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, "pip-project")
	defer cleanUp()
	// Run getModulesDependencyTrees
	auditCmd := NewEmptyAuditPipCommand()
	rootNodes, err := auditCmd.buildPipDependencyTree()
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
