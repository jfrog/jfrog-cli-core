package repostate

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"io/ioutil"
	"path"
	"strings"
)

// Represents a state of a repository being traversed to do a certain action.
// The state is constructed as a linked prefix tree, where every node has pointers to its children and parent.
// While traversing over the repository, contents found are added to their respecting node, and those handled are then removed.
// Each node has one of three states:
//   1. Unexplored / Partially explored - NOT all contents of the directory were found and added to its struct (Marked by !DoneExploring).
//   2. Fully explored - All contents of the directory were found, but NOT all of them handled (Marked by DoneExploring && !Completed).
//   3. Completed - All contents found and handled (Marked by Completed).
// In the event of a node reaching completion, a tree collapsing may occur in order to save space:
// The node will mark itself completed, and will then notify the parent to check completion as well.
type RepoStateManager struct {
	Root    *Node `json:"root,omitempty"`
	repoKey string
	// File path to save and load state to/from.
	stateFilePath string
}

// Used as an empty value for the FilesNames map, where we only use keys.
var emptyValue struct{}

type Node struct {
	Name          string              `json:"name,omitempty"`
	Parent        *Node               `json:"-"`
	Children      map[string]*Node    `json:"children,omitempty"`
	FilesNames    map[string]struct{} `json:"files,omitempty"`
	Completed     bool                `json:"completed,omitempty"`
	DoneExploring bool                `json:"done_exploring,omitempty"`
}

// Tries to load a repo state from the provided repoStatePath.
// If successful, returns the state and created=false.
// Otherwise, generates a new repo state and returns it with created=true.
func LoadOrCreateRepoStateManager(repoKey, repoStatePath string) (sm *RepoStateManager, created bool, err error) {
	exists, err := fileutils.IsFileExists(repoStatePath, false)
	if err != nil {
		return nil, false, err
	}

	stateManager := new(RepoStateManager)
	if exists {
		content, err := fileutils.ReadFile(repoStatePath)
		if err != nil {
			return nil, false, err
		}

		err = json.Unmarshal(content, stateManager)
		if err != nil {
			return nil, false, errorutils.CheckError(err)
		}
		// Parent references cannot be saved to file due to cyclic references. We add them manually here.
		stateManager.Root.addParentsReferences(nil)
	} else {
		stateManager.Root = createNewNode(".", nil)
		created = true
	}

	stateManager.repoKey = repoKey
	stateManager.stateFilePath = repoStatePath
	return stateManager, created, nil
}

func (sm *RepoStateManager) SaveToFile() error {
	content, err := json.Marshal(sm)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return errorutils.CheckError(ioutil.WriteFile(sm.stateFilePath, content, 0644))
}

// Returns the node corresponding to the directory in the provided relative path. Path should be provided without the repository name.
func (sm *RepoStateManager) LookUpNode(relativePath string) (*Node, error) {
	if relativePath == "" {
		return nil, errorutils.CheckErrorf(getLookUpErrorPrefix(relativePath) + "- unexpected empty path provided to look up")
	}
	relativePath = strings.TrimSuffix(relativePath, "/")
	if relativePath == "." {
		return sm.Root, nil
	}

	// Progress through the children maps till reaching the node that represents the requested path.
	dirs := strings.Split(relativePath, "/")
	curNode := sm.Root
dirsLoop:
	for _, dir := range dirs {
		for childName, child := range curNode.Children {
			if childName == dir {
				curNode = child
				continue dirsLoop
			}
		}
		return nil, errorutils.CheckErrorf("%s - failed to find the node of directory '%s'", getLookUpErrorPrefix(relativePath), dir)
	}
	return curNode, nil
}

func getLookUpErrorPrefix(relativePath string) string {
	return fmt.Sprintf("could not reach the representing node for path '%s'", relativePath)
}

// Returns the node's relative path in the repository.
func (node *Node) getActualPath() string {
	curPath := node.Name
	curNode := node
	// Progress through parent references till reaching root.
	for {
		curNode = curNode.Parent
		if curNode == nil {
			// Reached root.
			return curPath
		}
		// Append parent node's dir name to beginning of path.
		curPath = path.Join(curNode.Name, curPath)
	}
}

// Sets node as completed, empties its contents, notifies parent to check completion.
func (node *Node) setCompleted() {
	node.Completed = true
	node.Children = nil
	node.FilesNames = nil
	parent := node.Parent
	node.Parent = nil
	if parent != nil {
		parent.CheckCompleted()
	}
}

// Check if node completed - if done exploring, done handling files, children are completed.
func (node *Node) CheckCompleted() {
	if !node.DoneExploring || len(node.FilesNames) > 0 {
		return
	}
	for _, child := range node.Children {
		if !child.Completed {
			return
		}
	}
	// All files and children completed. Mark this node as completed as well.
	node.setCompleted()
}

func (node *Node) AddFileName(fileName string) {
	if node.FilesNames == nil {
		node.FilesNames = make(map[string]struct{})
	}
	node.FilesNames[fileName] = emptyValue
}

func (node *Node) FileCompleted(fileName string) error {
	if _, exists := node.FilesNames[fileName]; exists {
		delete(node.FilesNames, fileName)
		return nil
	}
	return errorutils.CheckErrorf("could not find file name '%s' in node of dir '%s'", fileName, node.Name)
}

// Adds a new child node to children map.
// childrenMapPool - [Optional] Children map to check existence of a dirName in before creating a new node.
func (node *Node) AddChildNode(dirName string, childrenMapPool map[string]*Node) {
	if node.Children == nil {
		node.Children = make(map[string]*Node)
	}
	if child, exists := childrenMapPool[dirName]; exists {
		node.Children[dirName] = child
		return
	}
	node.Children[dirName] = createNewNode(dirName, node)
}

// Recursively adds parent references to all nodes.
func (node *Node) addParentsReferences(parent *Node) {
	node.Parent = parent
	for _, child := range node.Children {
		child.addParentsReferences(node)
	}
}

func createNewNode(dirName string, parent *Node) *Node {
	return &Node{
		Name:   dirName,
		Parent: parent,
	}
}
