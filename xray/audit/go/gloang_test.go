package _go

import (
	"strings"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"

	"github.com/stretchr/testify/assert"
)

func TestBuildGoDependencyList(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := audit.CreateTestWorkspace(t, "go-project")
	defer cleanUp()

	err := removeTxtSuffix("go.mod.txt")
	assert.NoError(t, err)
	err = removeTxtSuffix("go.sum.txt")
	assert.NoError(t, err)
	err = removeTxtSuffix("test.go.txt")
	assert.NoError(t, err)

	// Run getModulesDependencyTrees
	rootNode, err := BuildGoDependencyTree()
	assert.NoError(t, err)
	assert.NotEmpty(t, rootNode)

	// Check root module
	assert.Equal(t, rootNode.Id, goPackageTypeIdentifier+"testGoList")
	assert.Len(t, rootNode.Nodes, 2)

	// Test child without sub nodes
	child1 := audit.GetAndAssertNode(t, rootNode.Nodes, "golang.org/x/text:v0.3.3")
	assert.Len(t, child1.Nodes, 0)

	// Test child with 1 sub node
	child2 := audit.GetAndAssertNode(t, rootNode.Nodes, "rsc.io/quote:v1.5.2")
	assert.Len(t, child2.Nodes, 1)
	audit.GetAndAssertNode(t, child2.Nodes, "rsc.io/sampler:v1.3.0")
}

func removeTxtSuffix(txtFileName string) error {
	// go.sum.txt  >> go.sum
	return fileutils.MoveFile(txtFileName, strings.TrimSuffix(txtFileName, ".txt"))
}
