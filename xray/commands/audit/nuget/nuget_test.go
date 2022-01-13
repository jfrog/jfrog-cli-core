package nuget

import (
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"github.com/stretchr/testify/assert"
)

func TestBuildNugetDependencyList(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, "nuget-project")
	defer cleanUp()
	// Run getModulesDependencyTrees
	auditCmd := NewEmptyAuditNugetCommand()
	tree, err := auditCmd.buildNugetDependencyTree()
	assert.NoError(t, err)
	assert.NotEmpty(t, tree)
	if tree != nil {
		// Test root module
		rootNode := audit.GetAndAssertNode(t, tree, "core")
		// Test child module
		audit.GetAndAssertNode(t, rootNode.Nodes, "MyLogger:1.0.0.0")
	}
}
