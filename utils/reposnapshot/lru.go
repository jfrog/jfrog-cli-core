package reposnapshot

import (
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

var cacheSize = 50

// Repo snapshot LRU cache - Adds nodes while keeping insertion order, and discards nodes by an LRU first policy.
// Useful when the same nodes are frequently accessed, to avoid searching for a node in the whole repository tree each time it is needed.
// LRU - least recently used, MRU - most recently used.
type RepoSnapshotLruCache struct {
	cache          map[string]*Node
	insertionOrder []string
}

func newRepoSnapshotLruCache() RepoSnapshotLruCache {
	return RepoSnapshotLruCache{
		cache: make(map[string]*Node, cacheSize),
	}
}

// Returns the node corresponding to the provided relative path if it exists in cache. If so, also sets the path as MRU.
// Otherwise, returns nil.
func (lru *RepoSnapshotLruCache) get(relativePath string) (node *Node, err error) {
	node = lru.cache[relativePath]
	if node != nil {
		err = lru.setPathAsMRU(relativePath)
	}
	return
}

// Adds a relative path and its corresponding node to cache and the insertion order array, if they don't exist already.
// If cache is full, discards the LRU node.
func (lru *RepoSnapshotLruCache) add(relativePath string, node *Node) error {
	existingNode, err := lru.get(relativePath)
	if err != nil || existingNode != nil {
		return err
	}
	// If max cache size, remove the oldest element.
	if len(lru.insertionOrder) == cacheSize {
		oldestPath := lru.insertionOrder[0]
		delete(lru.cache, oldestPath)
		lru.insertionOrder = lru.insertionOrder[1:]
	}

	lru.cache[relativePath] = node
	lru.insertionOrder = append(lru.insertionOrder, relativePath)
	return nil
}

// Sets a path as the MRU by moving it to the end of the insertion order.
func (lru *RepoSnapshotLruCache) setPathAsMRU(relativePath string) error {
	if lru.isPathMRU(relativePath) {
		return nil
	}
	for i, path := range lru.insertionOrder {
		if path == relativePath {
			lru.insertionOrder = append(lru.insertionOrder[:i], lru.insertionOrder[i+1:]...)
			lru.insertionOrder = append(lru.insertionOrder, relativePath)
			return nil
		}
	}
	return errorutils.CheckErrorf("unexpected - path '" + relativePath + "' exists in the lru node map but does not exist in the insertion order array")
}

// Checks whether the path is the MRU, by checking if it is last on the insertion order array.
func (lru *RepoSnapshotLruCache) isPathMRU(relativePath string) bool {
	return lru.insertionOrder[len(lru.insertionOrder)-1] == relativePath
}
