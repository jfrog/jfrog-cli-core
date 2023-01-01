package reposnapshot

import (
	"encoding/json"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"os"
	"path"
	"sync"
)

// Represents a directory in the repo state snapshot.
type Node struct {
	// Mutex is on the Node level to allow modifying non-conflicting content on multiple nodes simultaneously.
	rwMutex            sync.RWMutex
	name               string
	parent             *Node
	children           map[string]*Node
	filesNamesAndSizes map[string]int64
	completed          bool
	doneExploring      bool
}

// Used to export/load the node tree to/from a file.
// Wrapper is needed since fields on the original node are unexported (to avoid operations that aren't thread safe).
// In addition, wrapper does not hold the parent pointer to avoid cyclic reference on export.
type NodeExportWrapper struct {
	Name               string                        `json:"name,omitempty"`
	Children           map[string]*NodeExportWrapper `json:"children,omitempty"`
	FilesNamesAndSizes map[string]int64              `json:"files,omitempty"`
	Completed          bool                          `json:"completed,omitempty"`
	DoneExploring      bool                          `json:"done_exploring,omitempty"`
}

type ActionOnNodeFunc func(node *Node) error

// Perform a writing action on the node's content.
// Warning: Calling an action inside another action will cause a deadlock!
func (node *Node) writeAction(action ActionOnNodeFunc) error {
	node.rwMutex.Lock()
	defer node.rwMutex.Unlock()

	return action(node)
}

// Perform a read only action on the node's content.
// Warning: Calling an action inside another action will cause a deadlock!
func (node *Node) readAction(action ActionOnNodeFunc) error {
	node.rwMutex.RLock()
	defer node.rwMutex.RUnlock()

	return action(node)
}

// Convert node to wrapper in order to save it to file.
func (node *Node) convertToWrapper() (wrapper *NodeExportWrapper, err error) {
	err = node.readAction(func(node *Node) error {
		wrapper = &NodeExportWrapper{
			Name:               node.name,
			FilesNamesAndSizes: node.filesNamesAndSizes,
			Completed:          node.completed,
			DoneExploring:      node.doneExploring,
		}
		if len(node.children) > 0 {
			wrapper.Children = make(map[string]*NodeExportWrapper)
			for _, child := range node.children {
				converted, err := child.convertToWrapper()
				if err != nil {
					return err
				}
				wrapper.Children[child.name] = converted
			}
		}
		return nil
	})
	return
}

// Convert the loaded node export wrapper to node.
func (wrapper *NodeExportWrapper) convertToNode() *Node {
	node := &Node{
		name:               wrapper.Name,
		filesNamesAndSizes: wrapper.FilesNamesAndSizes,
		completed:          wrapper.Completed,
		doneExploring:      wrapper.DoneExploring,
	}
	if len(wrapper.Children) > 0 {
		node.children = make(map[string]*Node)
		for i := range wrapper.Children {
			converted := wrapper.Children[i].convertToNode()
			node.children[converted.name] = converted
			node.children[converted.name].parent = node
		}
	}
	return node
}

// Returns the node's relative path in the repository.
func (node *Node) getActualPath() (actualPath string, err error) {
	err = node.readAction(func(node *Node) error {
		curPath := node.name
		curNode := node
		// Progress through parent references till reaching root.
		for {
			curNode = curNode.parent
			if curNode == nil {
				// Reached root.
				actualPath = curPath
				return nil
			}
			// Append parent node's dir name to beginning of path.
			curPath = path.Join(curNode.name, curPath)
		}
	})
	return
}

// Sets node as completed, clear its contents, notifies parent to check completion.
func (node *Node) setCompleted() error {
	return node.writeAction(func(node *Node) error {
		node.completed = true
		node.children = nil
		node.filesNamesAndSizes = nil
		parent := node.parent
		node.parent = nil
		if parent != nil {
			return parent.CheckCompleted()
		}
		return nil
	})
}

// Check if node completed - if done exploring, done handling files, children are completed.
func (node *Node) CheckCompleted() error {
	isCompleted := false
	err := node.readAction(func(node *Node) error {
		if !node.doneExploring || len(node.filesNamesAndSizes) > 0 {
			return nil
		}
		for _, child := range node.children {
			if !child.completed {
				return nil
			}
		}
		isCompleted = true
		return nil
	})
	if err != nil || !isCompleted {
		return err
	}
	// All files and children completed. Mark this node as completed as well.
	return node.setCompleted()
}

func (node *Node) AddFile(fileName string, size int64) error {
	return node.writeAction(func(node *Node) error {
		if node.filesNamesAndSizes == nil {
			node.filesNamesAndSizes = make(map[string]int64)
		}
		node.filesNamesAndSizes[fileName] = size
		return nil
	})
}

func (node *Node) FileCompleted(fileName string) error {
	return node.writeAction(func(node *Node) error {
		if _, exists := node.filesNamesAndSizes[fileName]; exists {
			delete(node.filesNamesAndSizes, fileName)
			return nil
		}
		return errorutils.CheckErrorf("could not find file name '%s' in node of dir '%s'", fileName, node.name)
	})
}

// Adds a new child node to children map.
// childrenMapPool - [Optional] Children map to check existence of a dirName in before creating a new node.
func (node *Node) AddChildNode(dirName string, childrenMapPool map[string]*Node) error {
	return node.writeAction(func(node *Node) error {
		if node.children == nil {
			node.children = make(map[string]*Node)
		}
		if child, exists := childrenMapPool[dirName]; exists {
			child.parent = node
			node.children[dirName] = child
			return nil
		}
		node.children[dirName] = CreateNewNode(dirName, node)
		return nil
	})
}

func (node *Node) convertAndSaveToFile(stateFilePath string) error {
	wrapper, err := node.convertToWrapper()
	if err != nil {
		return err
	}
	content, err := json.Marshal(wrapper)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return errorutils.CheckError(os.WriteFile(stateFilePath, content, 0644))
}

// Marks that all contents of the node have been found and added.
func (node *Node) MarkDoneExploring() error {
	return node.writeAction(func(node *Node) error {
		node.doneExploring = true
		return nil
	})
}

func (node *Node) GetFiles() (filesMap map[string]int64, err error) {
	err = node.readAction(func(node *Node) error {
		filesMap = node.filesNamesAndSizes
		return nil
	})
	return
}

func (node *Node) GetChildren() (children map[string]*Node, err error) {
	err = node.readAction(func(node *Node) error {
		children = node.children
		return nil
	})
	return
}

func (node *Node) IsCompleted() (completed bool, err error) {
	err = node.readAction(func(node *Node) error {
		completed = node.completed
		return nil
	})
	return
}

func (node *Node) IsDoneExploring() (doneExploring bool, err error) {
	err = node.readAction(func(node *Node) error {
		doneExploring = node.doneExploring
		return nil
	})
	return
}

func (node *Node) RestartExploring() error {
	return node.writeAction(func(node *Node) error {
		node.doneExploring = false
		node.completed = false
		node.children = nil
		node.filesNamesAndSizes = nil
		return nil
	})
}

// Recursively find the node matching the path represented by the dirs array.
// The search is done by comparing the children of each node path, till reaching the final node in the array.
// If the node is not found, nil is returned.
// For example:
// For a structure such as repo->dir1->dir2->dir3
// The initial call will be to the root, and for an input of ({"dir1","dir2"}), and the final output will be a pointer to dir2.
func (node *Node) findMatchingNode(childrenDirs []string) (matchingNode *Node, err error) {
	err = node.readAction(func(node *Node) error {
		if len(childrenDirs) == 0 {
			matchingNode = node
			return nil
		}
		for childName, child := range node.children {
			if childName == childrenDirs[0] {
				matchingNode, err = child.findMatchingNode(childrenDirs[1:])
				return err
			}
		}
		return nil
	})
	return
}

func CreateNewNode(dirName string, parent *Node) *Node {
	return &Node{
		name:   dirName,
		parent: parent,
	}
}
