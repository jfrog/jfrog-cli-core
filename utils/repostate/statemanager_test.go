package repostate

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
	t.Run("repo state doesn't exist", func(t *testing.T) { testLoad(t, "/path/to/file", true, createNewNode(".", nil)) })
	t.Run("repo state exists", func(t *testing.T) { testLoad(t, expectedFile, false, createTestStateTree()) })
}

func testLoad(t *testing.T, statePath string, expectedCreated bool, expectedRoot *Node) {
	sm, created, err := LoadOrCreateRepoStateManager(dummyRepoKey, statePath)
	assert.NoError(t, err)
	assert.Equal(t, expectedCreated, created)
	assert.Equal(t, expectedRoot, sm.Root)
	assert.Equal(t, statePath, sm.stateFilePath)
	assert.Equal(t, dummyRepoKey, sm.repoKey)
}

func TestSaveToFile(t *testing.T) {
	tmpDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)

	manager := RepoStateManager{
		Root:          createTestStateTree(),
		repoKey:       dummyRepoKey,
		stateFilePath: filepath.Join(tmpDir, dummyRepoKey),
	}
	assert.NoError(t, manager.SaveToFile())

	// Assert file written as expected.
	expected, err := os.ReadFile(expectedFile)
	assert.NoError(t, err)
	actual, err := os.ReadFile(manager.stateFilePath)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestNodeCompletedAndTreeCollapsing(t *testing.T) {
	manager := RepoStateManager{
		Root: createTestStateTree(),
	}
	path0 := "0"
	node0, err := manager.LookUpNode(path0)
	assert.NoError(t, err)
	assert.NotNil(t, node0)
	assert.Equal(t, path.Join(".", path0), node0.getActualPath())

	// Try setting a dir with no files as completed. Should not succeed as children are not completed.
	node0.CheckCompleted()
	assertNotCompleted(t, node0)

	if len(node0.Children) != 1 {
		assert.Len(t, node0.Children, 1)
		return
	}
	node0a := node0.Children["a"]

	// Try setting a dir with no children as completed. Should not succeed as it still contains files.
	node0a.CheckCompleted()
	assertNotCompleted(t, node0a)

	setAllNodeFilesCompleted(t, node0a)
	node0a.CheckCompleted()
	// Node should be completed as all files completed.
	assertCompleted(t, node0a)
	// Tree collapsing expected - parent should also be completed as it has no files and all children are completed.
	assertCompleted(t, node0)
	// Root should not be completed as it still has uncompleted children.
	assertNotCompleted(t, manager.Root)
}

func TestNodeCompletedWhileExploring(t *testing.T) {
	manager := RepoStateManager{
		Root: createTestStateTree(),
	}
	// Mark a node without files or children as unexplored and try to set it as completed. Should not succeed.
	path2 := "2"
	node2, err := manager.LookUpNode(path2)
	assert.NoError(t, err)
	assert.NotNil(t, node2)
	node2.DoneExploring = false
	node2.CheckCompleted()
	assertNotCompleted(t, node2)

	// Mark it as explored and try again.
	node2.DoneExploring = true
	node2.CheckCompleted()
	assertCompleted(t, node2)
}

func assertCompleted(t *testing.T, node *Node) {
	assert.True(t, node.Completed)
	assert.Nil(t, node.Parent)
	assert.Len(t, node.Children, 0)
}

func assertNotCompleted(t *testing.T, node *Node) {
	assert.False(t, node.Completed)
}

type lookUpAndPathTestSuite struct {
	testName      string
	path          string
	errorExpected bool
}

func TestLookUpNodeAndActualPath(t *testing.T) {
	manager := RepoStateManager{
		Root: createTestStateTree(),
	}

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
				assert.ErrorContains(t, err, getLookUpErrorPrefix(test.path))
				return
			} else {
				assert.NoError(t, err)
			}
			if node == nil {
				assert.NotNil(t, node)
				return
			}
			assert.Equal(t, path.Join(".", test.path), node.getActualPath())
		})
	}
}

func setAllNodeFilesCompleted(t *testing.T, node *Node) {
	for key := range node.FilesNames {
		assert.NoError(t, node.FileCompleted(key))
	}
}

// Tree dirs representation:
// root - 0 - a + 3 files
//      - 1 - a + 1 file
//          - b + 2 files
//			+ 1 file
//      - 2
//      + 1 file
func createTestStateTree() *Node {
	root := createNodeBase(".", []string{"file-on-root"}, nil)
	dir0 := createNodeBase("0", []string{}, root)
	dir1 := createNodeBase("1", []string{"file-1-f0"}, root)
	dir2 := createNodeBase("2", []string{}, root)

	dir0a := createNodeBase("a", []string{"file-1-0-a-f0", "file-1-0-a-f1", "file-1-0-a-f2"}, dir0)
	dir1a := createNodeBase("a", []string{"file-1-1-a-f0"}, dir1)
	dir1b := createNodeBase("b", []string{"file-1-1-b-f0", "file-1-1-b-f1"}, dir1)

	addChildren(root, dir0, dir1, dir2)
	addChildren(dir0, dir0a)
	addChildren(dir1, dir1a, dir1b)
	return root
}

func addChildren(node *Node, children ...*Node) {
	node.Children = make(map[string]*Node)
	for i := range children {
		node.Children[children[i].Name] = children[i]
	}
}

func createNodeBase(name string, filesNames []string, parent *Node) *Node {
	node := createNewNode(name, parent)
	node.DoneExploring = true
	for _, fileName := range filesNames {
		node.AddFileName(fileName)
	}
	return node
}

func TestAddChildNode(t *testing.T) {
	root := createNewNode(".", nil)
	// Add child with no children pool.
	addAndAssertChild(t, nil, root, createNewNode("no-pool", root))
	// Add child with empty children pool.
	pool := make(map[string]*Node)
	addAndAssertChild(t, pool, root, createNewNode("empty-pool", root))
	// Add child with pool.
	exists := createNewNode("exists", root)
	pool[exists.Name] = exists
	addAndAssertChild(t, pool, root, exists)
}

func addAndAssertChild(t *testing.T, childrenMapPool map[string]*Node, root, expectedChild *Node) {
	root.AddChildNode(expectedChild.Name, childrenMapPool)
	assert.Equal(t, expectedChild, root.Children[expectedChild.Name])
}
