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
		// Convert to wrapper in order to compare.
		expectedWrapper, err := expectedRoot.convertToWrapper()
		assert.NoError(t, err)
		rootWrapper, err := sm.root.convertToWrapper()
		assert.NoError(t, err)
		assert.Equal(t, expectedWrapper, rootWrapper)
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
	node0a := getChild(node0, "a")

	// Try setting a dir with no children as completed. Should not succeed as it still contains files.
	assert.NoError(t, node0a.CheckCompleted())
	assertNotCompleted(t, node0a)

	setAllNodeFilesCompleted(node0a)
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
	node2.NodeStatus = Exploring
	assert.NoError(t, node2.CheckCompleted())
	assertNotCompleted(t, node2)

	// Mark it as explored and try again.
	node2.NodeStatus = DoneExploring
	assert.NoError(t, node2.CheckCompleted())
	assertCompleted(t, node2)
}

func assertCompleted(t *testing.T, node *Node) {
	assert.Equal(t, Completed, node.NodeStatus)
	assert.Nil(t, node.parent)
	assert.Len(t, node.children, 0)
}

func assertNotCompleted(t *testing.T, node *Node) {
	assert.Less(t, node.NodeStatus, Completed)
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

func setAllNodeFilesCompleted(node *Node) {
	node.filesCount = 0
}

// Tree dirs representation:
// root -> 0 -> a + 3 files
// ------> 1 -> a + 1 file
// -----------> b + 2 files
// ---------- + 1 file
// ------> 2
// ----- + 1 file
func createTestSnapshotTree(t *testing.T) *Node {
	root := createNodeBase(t, ".", 1, nil)
	dir0 := createNodeBase(t, "0", 0, root)
	dir1 := createNodeBase(t, "1", 1, root)
	dir2 := createNodeBase(t, "2", 0, root)

	dir0a := createNodeBase(t, "a", 3, dir0)
	dir1a := createNodeBase(t, "a", 1, dir1)
	dir1b := createNodeBase(t, "b", 2, dir1)

	addChildren(root, dir0, dir1, dir2)
	addChildren(dir0, dir0a)
	addChildren(dir1, dir1a, dir1b)
	return root
}

func addChildren(node *Node, children ...*Node) {
	node.children = append(node.children, children...)
}

func createNodeBase(t *testing.T, name string, filesCount int, parent *Node) *Node {
	node := CreateNewNode(name, parent)
	node.NodeStatus = DoneExploring
	for i := 0; i < filesCount; i++ {
		assert.NoError(t, node.IncrementFilesCount())
	}
	return node
}

func getChild(node *Node, childName string) *Node {
	for _, child := range node.children {
		if child.name == childName {
			return child
		}
	}
	return nil
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
