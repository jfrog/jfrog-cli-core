package reposnapshot

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

	lruCache := newRepoSnapshotLruCache()
	lru := &lruCache
	node0 := createAndAddNode(t, lru, "node0", 1)
	_ = createAndAddNode(t, lru, "node1", 2)
	node2 := createAndAddNode(t, lru, "node2", 3)

	// Add a node that exceeds the cache size and verify the first node was removed.
	_ = createAndAddNode(t, lru, "node3", 3)
	receivedNode, err := lru.get("node0")
	assert.NoError(t, err)
	assert.Nil(t, receivedNode)

	// Get a previously added node and assert it was moved to the end of the array (MRU).
	receivedNode, err = lru.get("node2")
	assert.NoError(t, err)
	assertInsertion(t, node2, receivedNode, lru, 3)

	// Add an existing node and assert it is set as MRU.
	err = lru.add("node0", node0)
	assert.NoError(t, err)
	addNodeToLruAndAssert(t, lru, node0, 3)
}

func assertInsertion(t *testing.T, expectedNode, actualNode *Node, lru *RepoSnapshotLruCache, expectedLen int) {
	assert.Equal(t, expectedNode, actualNode)
	assert.Len(t, lru.insertionOrder, expectedLen)
	assert.True(t, lru.isPathMRU(expectedNode.name))
}

func createAndAddNode(t *testing.T, lru *RepoSnapshotLruCache, dirName string, expectedLen int) *Node {
	node := CreateNewNode(dirName, nil)
	addNodeToLruAndAssert(t, lru, node, expectedLen)
	return node
}

func addNodeToLruAndAssert(t *testing.T, lru *RepoSnapshotLruCache, node *Node, expectedLen int) {
	relativePath, err := node.getActualPath()
	assert.NoError(t, err)
	assert.NoError(t, lru.add(relativePath, node))
	returnedNode, err := lru.get(relativePath)
	assert.NoError(t, err)
	assertInsertion(t, node, returnedNode, lru, expectedLen)
}
