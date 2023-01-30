package reposnapshot

import (
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"path/filepath"
	"testing"
)

const dummyRepoKey = "dummy-repo-local"

var expectedFile = filepath.Join("testdata", dummyRepoKey)

func TestLoad(t *testing.T) {
	t.Run("repo snapshot doesn't exist", func(t *testing.T) { testLoad(t, "/path/to/file", false, CreateNewNode(".", nil)) })
	t.Run("repo snapshot exists", func(t *testing.T) { testLoad(t, expectedFile, true, createTestSnapshotTree(t)) })
}

func testLoad(t *testing.T, snapshotPath string, expectedExists bool, expectedRoot *Node) {
	sm, exists, err := LoadRepoSnapshotManager(dummyRepoKey, snapshotPath)
	assert.NoError(t, err)
	assert.Equal(t, expectedExists, exists)
	if expectedExists {
		assert.Equal(t, expectedRoot, sm.root)
		assert.Equal(t, snapshotPath, sm.snapshotFilePath)
		assert.Equal(t, dummyRepoKey, sm.repoKey)
	}
}

func TestSaveToFile(t *testing.T) {
	manager := initSnapshotManagerTest(t)
	assert.NoError(t, manager.root.convertAndSaveToFile(manager.snapshotFilePath))

	// Assert file written as expected.
	expected, err := os.ReadFile(expectedFile)
	assert.NoError(t, err)
	actual, err := os.ReadFile(manager.snapshotFilePath)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestNodeCompletedAndTreeCollapsing(t *testing.T) {
	manager := initSnapshotManagerTest(t)
	path0 := "0"
	node0, err := manager.LookUpNode(path0)
	assert.NoError(t, err)
	assert.NotNil(t, node0)
	actualPath, err := node0.getActualPath()
	assert.NoError(t, err)
	assert.Equal(t, path.Join(".", path0), actualPath)

	// Try setting a dir with no files as completed. Should not succeed as children are not completed.
	assert.NoError(t, node0.CheckCompleted())
	assertNotCompleted(t, node0)

	if len(node0.children) != 1 {
		assert.Len(t, node0.children, 1)
		return
	}
	node0a := node0.children["a"]

	// Try setting a dir with no children as completed. Should not succeed as it still contains files.
	assert.NoError(t, node0a.CheckCompleted())
	assertNotCompleted(t, node0a)

	setAllNodeFilesCompleted(t, node0a)
	assert.NoError(t, node0a.CheckCompleted())
	// Node should be completed as all files completed.
	assertCompleted(t, node0a)
	// Tree collapsing expected - parent should also be completed as it has no files and all children are completed.
	assertCompleted(t, node0)
	// root should not be completed as it still has uncompleted children.
	assertNotCompleted(t, manager.root)
}

func TestNodeCompletedWhileExploring(t *testing.T) {
	manager := initSnapshotManagerTest(t)
	// Mark a node without files or children as unexplored and try to set it as completed. Should not succeed.
	path2 := "2"
	node2, err := manager.LookUpNode(path2)
	assert.NoError(t, err)
	assert.NotNil(t, node2)
	node2.doneExploring = false
	assert.NoError(t, node2.CheckCompleted())
	assertNotCompleted(t, node2)

	// Mark it as explored and try again.
	node2.doneExploring = true
	assert.NoError(t, node2.CheckCompleted())
	assertCompleted(t, node2)
}

func assertCompleted(t *testing.T, node *Node) {
	assert.True(t, node.completed)
	assert.Nil(t, node.parent)
	assert.Len(t, node.children, 0)
}

func assertNotCompleted(t *testing.T, node *Node) {
	assert.False(t, node.completed)
}

type lookUpAndPathTestSuite struct {
	testName      string
	path          string
	errorExpected bool
}

func TestLookUpNodeAndActualPath(t *testing.T) {
	manager := initSnapshotManagerTest(t)

	tests := []lookUpAndPathTestSuite{
		{"root", ".", false},
		{"dir on root", "2", false},
		{"complex path with separator suffix", "1/a/", false},
		{"complex path with no separator suffix", "1/a", false},
		{"repository provided", path.Join("test-local", "2"), true},
		{"relative path includes root", "./2", true},
		{"dir doesn't exist", "no/where", true},
		{"empty path", "", true},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			node, err := manager.LookUpNode(test.path)
			if test.errorExpected {
				assert.ErrorContains(t, err, getLookUpNodeError(test.path))
				return
			} else {
				assert.NoError(t, err)
			}
			if node == nil {
				assert.NotNil(t, node)
				return
			}
			actualPath, err := node.getActualPath()
			assert.NoError(t, err)
			assert.Equal(t, path.Join(".", test.path), actualPath)
		})
	}
}

func setAllNodeFilesCompleted(t *testing.T, node *Node) {
	for key := range node.filesNamesAndSizes {
		assert.NoError(t, node.FileCompleted(key))
	}
}

// Tree dirs representation:
// root -> 0 -> a + 3 files
// ------> 1 -> a + 1 file
// -----------> b + 2 files
// ---------- + 1 file
// ------> 2
// ----- + 1 file
func createTestSnapshotTree(t *testing.T) *Node {
	root := createNodeBase(t, ".", []string{"file-on-root"}, nil)
	dir0 := createNodeBase(t, "0", []string{}, root)
	dir1 := createNodeBase(t, "1", []string{"file-1-f0"}, root)
	dir2 := createNodeBase(t, "2", []string{}, root)

	dir0a := createNodeBase(t, "a", []string{"file-1-0-a-f0", "file-1-0-a-f1", "file-1-0-a-f2"}, dir0)
	dir1a := createNodeBase(t, "a", []string{"file-1-1-a-f0"}, dir1)
	dir1b := createNodeBase(t, "b", []string{"file-1-1-b-f0", "file-1-1-b-f1"}, dir1)

	addChildren(root, dir0, dir1, dir2)
	addChildren(dir0, dir0a)
	addChildren(dir1, dir1a, dir1b)
	return root
}

func addChildren(node *Node, children ...*Node) {
	node.children = make(map[string]*Node)
	for i := range children {
		node.children[children[i].name] = children[i]
	}
}

func createNodeBase(t *testing.T, name string, filesNames []string, parent *Node) *Node {
	node := CreateNewNode(name, parent)
	node.doneExploring = true
	for _, fileName := range filesNames {
		assert.NoError(t, node.AddFile(fileName, 123))
	}
	return node
}

func TestAddChildNode(t *testing.T) {
	root := CreateNewNode(".", nil)
	// Add child with no children pool.
	addAndAssertChild(t, nil, root, CreateNewNode("no-pool", root))
	// Add child with empty children pool.
	pool := make(map[string]*Node)
	addAndAssertChild(t, pool, root, CreateNewNode("empty-pool", root))
	// Add child with pool.
	exists := CreateNewNode("exists", root)
	pool[exists.name] = exists
	addAndAssertChild(t, pool, root, exists)
}

func addAndAssertChild(t *testing.T, childrenMapPool map[string]*Node, root, expectedChild *Node) {
	assert.NoError(t, root.AddChildNode(expectedChild.name, childrenMapPool))
	assert.Equal(t, expectedChild, root.children[expectedChild.name])
}

func initSnapshotManagerTest(t *testing.T) RepoSnapshotManager {
	file, err := fileutils.CreateTempFile()
	assert.NoError(t, err)
	assert.NoError(t, file.Close())
	return newRepoSnapshotManager(createTestSnapshotTree(t), dummyRepoKey, file.Name())
}

func TestGetDirectorySnapshotNodeWithLruLRU(t *testing.T) {
	originalCacheSize := cacheSize
	cacheSize = 3
	defer func() {
		cacheSize = originalCacheSize
	}()
	manager := initSnapshotManagerTest(t)

	// Assert lru cache is empty before getting nodes.
	assert.Zero(t, manager.lruCache.Len())

	// Get 3 nodes which will cause the cache to reach its cache size.
	_ = getNodeAndAssert(t, manager, "1/b/", 1)
	_ = getNodeAndAssert(t, manager, "./", 2)
	_ = getNodeAndAssert(t, manager, "2/", 3)

	// Get another node that exceeds the cache size and assert the LRU node was removed.
	_ = getNodeAndAssert(t, manager, "0/a/", 3)
	_, exists := manager.lruCache.Get("1/b/")
	assert.False(t, exists)
}

func assertReturnedNode(t *testing.T, manager RepoSnapshotManager, node *Node, relativePath string, expectedLen int) {
	if !assert.NotNil(t, node) {
		return
	}
	actualPath, err := node.getActualPath()
	assert.NoError(t, err)
	assert.Equal(t, relativePath, actualPath)
	assert.Equal(t, expectedLen, manager.lruCache.Len())
}

func getNodeAndAssert(t *testing.T, manager RepoSnapshotManager, relativePath string, expectedLen int) *Node {
	node, err := manager.GetDirectorySnapshotNodeWithLru(relativePath)
	assert.NoError(t, err)
	assertReturnedNode(t, manager, node, path.Dir(relativePath), expectedLen)
	return node
}
