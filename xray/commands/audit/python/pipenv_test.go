package python

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBuildPipenvDependencyList(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, "pipenv-project")
	defer cleanUp()
	// Run getModulesDependencyTrees
	auditCmd := NewEmptyAuditPipenvCommand()
	dependencyTree, err := auditCmd.buildPipenvDependencyTree()
	assert.NoError(t, err)
	if assert.NotNil(t, dependencyTree) {
		assert.NotEmpty(t, dependencyTree.Nodes)
		// Test child module
		audit.GetAndAssertNode(t, dependencyTree.Nodes, "toml:0.10.2")

		// Test child module
		childNode := audit.GetAndAssertNode(t, dependencyTree.Nodes, "pexpect:4.8.0")
		// Test sub child module
		audit.GetAndAssertNode(t, childNode.Nodes, "ptyprocess:0.7.0")

	}
}
