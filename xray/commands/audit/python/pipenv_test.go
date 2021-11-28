package python

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildPipenvDependencyList(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, "pipenv-project")
	defer cleanUp()
	// Run getModulesDependencyTrees
	auditCmd := NewEmptyAuditPipenvCommand()
	rootNode, err := auditCmd.buildPipenvDependencyTree()
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEmpty(t, rootNode)

	// Test child module
	childNode := audit.GetAndAssertNode(t, rootNode.Nodes, "pexpect:4.8.0")
	// Test sub child module
	audit.GetAndAssertNode(t, childNode.Nodes, "ptyprocess:0.7.0")
}
