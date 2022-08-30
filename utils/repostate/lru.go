package repostate

import "github.com/jfrog/jfrog-client-go/utils/log"

var cacheSize = 50

// Repo state LRU cache - Adds node while keeping insertion order, and discards nodes by an LRU first policy.
// LRU - least recently used, MRU - most recently used.
type RepoStateLruCache struct {
	cache          map[string]*Node
	insertionOrder []string
}

func NewRepoStateLruCache() *RepoStateLruCache {
	return &RepoStateLruCache{
		cache: make(map[string]*Node, cacheSize),
	}
}

// Returns the node corresponding to the provided relative path if it exists in cache. If so, also sets the path as MRU.
// Otherwise, returns nil.
func (lru *RepoStateLruCache) Get(relativePath string) *Node {
	node := lru.cache[relativePath]
	if node != nil {
		lru.setPathAsMRU(relativePath)
	}
	return node
}

// Adds a relative path and its corresponding node to cache and the insertion order array, if they don't exist already.
// If cache is full, discards the LRU node.
func (lru *RepoStateLruCache) Add(relativePath string, node *Node) {
	if lru.Get(relativePath) != nil {
		return
	}
	// If max cache size, remove the oldest element.
	if len(lru.insertionOrder) == cacheSize {
		oldestPath := lru.insertionOrder[0]
		delete(lru.cache, oldestPath)
		lru.insertionOrder = lru.insertionOrder[1:]
	}

	lru.cache[relativePath] = node
	lru.insertionOrder = append(lru.insertionOrder, relativePath)
}

// Sets a path as the MRU by moving it to the end of the insertion order.
func (lru *RepoStateLruCache) setPathAsMRU(relativePath string) {
	if lru.isPathMRU(relativePath) {
		return
	}
	for i, path := range lru.insertionOrder {
		if path == relativePath {
			lru.insertionOrder = append(lru.insertionOrder[:i], lru.insertionOrder[i+1:]...)
			lru.insertionOrder = append(lru.insertionOrder, relativePath)
			return
		}
	}
	log.Error("unexpected - path '" + relativePath + "' exists in the lru state map but does not exist in the insertion order array")
}

// Checks whether the path is the MRU, by checking if it is last on the insertion order array.
func (lru *RepoStateLruCache) isPathMRU(relativePath string) bool {
	return lru.insertionOrder[len(lru.insertionOrder)-1] == relativePath
}
