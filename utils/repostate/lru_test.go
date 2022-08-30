package repostate

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLRU(t *testing.T) {
	originalCacheSize := cacheSize
	cacheSize = 3
	defer func() {
		cacheSize = originalCacheSize
	}()

	lru := NewRepoStateLruCache()
	node0 := createAndAddNode(t, lru, "node0", 1)
	_ = createAndAddNode(t, lru, "node1", 2)
	node2 := createAndAddNode(t, lru, "node2", 3)

	// Add a node that exceeds the cache size and verify the first node was removed.
	_ = createAndAddNode(t, lru, "node3", 3)
	receivedNode := lru.Get("node0")
	assert.Nil(t, receivedNode)

	// Get a previously added node and assert it was moved to the end of the array (MRU).
	receivedNode = lru.Get("node2")
	assertInsertion(t, node2, receivedNode, lru, 3)

	// Add an existing node and assert it is set as MRU.
	lru.Add("node0", node0)
	addNodeToLruAndAssert(t, lru, node0, 3)
}

func assertInsertion(t *testing.T, expectedNode, actualNode *Node, lru *RepoStateLruCache, expectedLen int) {
	assert.Equal(t, expectedNode, actualNode)
	assert.Len(t, lru.insertionOrder, expectedLen)
	assert.True(t, lru.isPathMRU(expectedNode.Name))
}

func createAndAddNode(t *testing.T, lru *RepoStateLruCache, dirName string, expectedLen int) *Node {
	node := createNewNode(dirName, nil)
	addNodeToLruAndAssert(t, lru, node, expectedLen)
	return node
}

func addNodeToLruAndAssert(t *testing.T, lru *RepoStateLruCache, node *Node, expectedLen int) {
	relativePath := node.getActualPath()
	lru.Add(relativePath, node)
	returnedNode := lru.Get(relativePath)
	assertInsertion(t, node, returnedNode, lru, expectedLen)
}
