package reposnapshot

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"strings"
)

// Represents a snapshot of a repository being traversed to do a certain action.
// Each directory in the repository is represented by a node.
// The snapshot is constructed as a linked prefix tree, where every node has pointers to its children and parent.
// While traversing over the repository, contents found are added to their respecting node. Later on, files handled are removed from their node.
// Each node has one of three states:
//  1. Unexplored / Partially explored - NOT all contents of the directory were found and added to its node (Marked by !DoneExploring).
//  2. Fully explored - All contents of the directory were found, but NOT all of them handled (Marked by DoneExploring && !Completed).
//  3. Completed - All contents found and handled (Marked by Completed).
//
// In the event of a node reaching completion, a tree collapsing may occur in order to save space:
// The node will mark itself completed, and will then notify the parent to check completion as well.
type RepoSnapshotManager struct {
	repoKey string
	// Pointer to the root node of the repository tree.
	root     *Node
	lruCache RepoSnapshotLruCache
	// File path for saving the snapshot to and reading the snapshot from.
	snapshotFilePath string
}

// Loads a repo snapshot from the provided snapshotFilePath if such file exists.
// If successful, returns the snapshot and exists=true.
func LoadRepoSnapshotManager(repoKey, snapshotFilePath string) (RepoSnapshotManager, bool, error) {
	exists, err := fileutils.IsFileExists(snapshotFilePath, false)
	if err != nil || !exists {
		return RepoSnapshotManager{}, false, err
	}

	root, err := loadAndConvertNodeTree(snapshotFilePath)
	if err != nil {
		return RepoSnapshotManager{}, false, err
	}
	return newRepoSnapshotManager(root, repoKey, snapshotFilePath), true, nil
}

func CreateRepoSnapshotManager(repoKey, snapshotFilePath string) RepoSnapshotManager {
	return newRepoSnapshotManager(CreateNewNode(".", nil), repoKey, snapshotFilePath)
}

func newRepoSnapshotManager(root *Node, repoKey, snapshotFilePath string) RepoSnapshotManager {
	return RepoSnapshotManager{
		root:             root,
		repoKey:          repoKey,
		snapshotFilePath: snapshotFilePath,
		lruCache:         newRepoSnapshotLruCache(),
	}
}

func loadAndConvertNodeTree(snapshotFilePath string) (root *Node, err error) {
	content, err := fileutils.ReadFile(snapshotFilePath)
	if err != nil {
		return nil, err
	}

	var nodeWrapper NodeExportWrapper
	err = json.Unmarshal(content, &nodeWrapper)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	return nodeWrapper.convertToNode(), nil
}

func (sm *RepoSnapshotManager) PersistRepoSnapshot() error {
	return sm.root.convertAndSaveToFile(sm.snapshotFilePath)
}

// Returns the node corresponding to the directory in the provided relative path. Path should be provided without the repository name.
func (sm *RepoSnapshotManager) LookUpNode(relativePath string) (requestedNode *Node, err error) {
	if relativePath == "" {
		return nil, errorutils.CheckErrorf(getLookUpErrorPrefix(relativePath) + "- unexpected empty path provided to look up")
	}
	relativePath = strings.TrimSuffix(relativePath, "/")
	if relativePath == "." {
		requestedNode = sm.root
		return
	}

	// Progress through the children maps till reaching the node that represents the requested path.
	dirs := strings.Split(relativePath, "/")
	requestedNode = findMatchingNode(dirs, sm.root)
	if requestedNode == nil {
		return nil, errorutils.CheckErrorf("Repo snapshot manager - %s", getLookUpErrorPrefix(relativePath))
	}
	return
}

// Returns the node that represents the directory from the repo state. Updates the lru cache.
// relativePath - relative path of the directory.
func (sm *RepoSnapshotManager) GetDirectorySnapshotNodeWithLru(relativePath string) (node *Node, err error) {
	node, err = sm.lruCache.get(relativePath)
	// If node already exists in cache, return it.
	if err != nil || node != nil {
		return node, err
	}

	// Otherwise, manually search for the node.
	node, err = sm.LookUpNode(relativePath)
	if err != nil {
		return nil, err
	}

	// Add it to cache.
	return node, sm.lruCache.add(relativePath, node)
}

// Recursively find the node matching the path represented by the dirs array.
// The search is done by comparing the children of each node path, till reaching the final node in the array.
// If the node is not found, nil is returned.
// For example:
// For a structure such as repo->dir1->dir2->dir3
// The initial input will be ({"dir1","dir2","dir3"},<pointer to root>), and the final output will be a pointer to dir3.
func findMatchingNode(childrenDirs []string, curNode *Node) (matchingNode *Node) {
	if len(childrenDirs) == 0 {
		return curNode
	}
	for childName, child := range curNode.children {
		if childName == childrenDirs[0] {
			return findMatchingNode(childrenDirs[1:], child)
		}
	}
	return nil
}

func getLookUpErrorPrefix(relativePath string) string {
	return fmt.Sprintf("could not reach the representing node for path '%s'", relativePath)
}
