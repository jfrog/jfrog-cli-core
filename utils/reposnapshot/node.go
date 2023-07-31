package reposnapshot

import (
	"encoding/json"
	"os"
	"path"
	"sync"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

// Represents a directory in the repo state snapshot.
type Node struct {
	parent   *Node
	name     string
	children []*Node
	// Mutex is on the Node level to allow modifying non-conflicting content on multiple nodes simultaneously.
	mutex sync.Mutex
	// The files count is used to identify when handling a node is completed. It is only used during runtime, and is not persisted to disk for future runs.
	filesCount uint32
	NodeStatus
}

type NodeStatus uint8

const (
	Exploring NodeStatus = iota
	DoneExploring
	Completed
)

// Used to export/load the node tree to/from a file.
// Wrapper is needed since fields on the original node are unexported (to avoid operations that aren't thread safe).
// The wrapper only contains fields that are used in future runs, hence not all fields from Node are persisted.
// In addition, it does not hold the parent pointer to avoid cyclic reference on export.
type NodeExportWrapper struct {
	Name      string               `json:"name,omitempty"`
	Children  []*NodeExportWrapper `json:"children,omitempty"`
	Completed bool                 `json:"completed,omitempty"`
}

type ActionOnNodeFunc func(node *Node) error

// Perform an action on the node's content.
// Warning: Calling an action inside another action will cause a deadlock!
func (node *Node) action(action ActionOnNodeFunc) error {
	node.mutex.Lock()
	defer node.mutex.Unlock()

	return action(node)
}

// Convert node to wrapper in order to save it to file.
func (node *Node) convertToWrapper() (wrapper *NodeExportWrapper, err error) {
	err = node.action(func(node *Node) error {
		wrapper = &NodeExportWrapper{
			Name:      node.name,
			Completed: node.NodeStatus == Completed,
		}
		for i := range node.children {
			converted, err := node.children[i].convertToWrapper()
			if err != nil {
				return err
			}
			wrapper.Children = append(wrapper.Children, converted)
		}
		return nil
	})
	return
}

// Convert the loaded node export wrapper to node.
func (wrapper *NodeExportWrapper) convertToNode() *Node {
	node := &Node{
		name: wrapper.Name,
	}
	// If node wasn't previously completed, we will start exploring it from scratch.
	if wrapper.Completed {
		node.NodeStatus = Completed
	}
	for i := range wrapper.Children {
		converted := wrapper.Children[i].convertToNode()
		converted.parent = node
		node.children = append(node.children, converted)
	}
	return node
}

// Returns the node's relative path in the repository.
func (node *Node) getActualPath() (actualPath string, err error) {
	err = node.action(func(node *Node) error {
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
	return node.action(func(node *Node) error {
		node.NodeStatus = Completed
		node.children = nil
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
	err := node.action(func(node *Node) error {
		if node.NodeStatus == Exploring || node.filesCount > 0 {
			return nil
		}
		for _, child := range node.children {
			if child.NodeStatus < Completed {
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

func (node *Node) IncrementFilesCount() error {
	return node.action(func(node *Node) error {
		node.filesCount++
		return nil
	})
}

func (node *Node) DecrementFilesCount() error {
	return node.action(func(node *Node) error {
		if node.filesCount > 0 {
			node.filesCount--
		}
		return nil
	})
}

// Adds a new child node to children map.
// childrenPool - [Optional] Children array to check existence of a dirName in before creating a new node.
func (node *Node) AddChildNode(dirName string, childrenPool []*Node) error {
	return node.action(func(node *Node) error {
		for i := range childrenPool {
			if childrenPool[i].name == dirName {
				childrenPool[i].parent = node
				node.children = append(node.children, childrenPool[i])
				return nil
			}
		}
		node.children = append(node.children, CreateNewNode(dirName, node))
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
	return node.action(func(node *Node) error {
		node.NodeStatus = DoneExploring
		return nil
	})
}

func (node *Node) GetChildren() (children []*Node, err error) {
	err = node.action(func(node *Node) error {
		children = node.children
		return nil
	})
	return
}

func (node *Node) IsCompleted() (completed bool, err error) {
	err = node.action(func(node *Node) error {
		completed = node.NodeStatus == Completed
		return nil
	})
	return
}

func (node *Node) IsDoneExploring() (doneExploring bool, err error) {
	err = node.action(func(node *Node) error {
		doneExploring = node.NodeStatus >= DoneExploring
		return nil
	})
	return
}

func (node *Node) RestartExploring() error {
	return node.action(func(node *Node) error {
		node.NodeStatus = Exploring
		node.filesCount = 0
		return nil
	})
}

// Recursively find the node matching the path represented by the dirs array.
// The search is done by comparing the children of each node path, till reaching the final node in the array.
// If the node is not found, it is added and then returned.
// For example:
// For a structure such as repo->dir1->dir2->dir3
// The initial call will be to the root, and for an input of ({"dir1","dir2"}), and the final output will be a pointer to dir2.
func (node *Node) findMatchingNode(childrenDirs []string) (matchingNode *Node, err error) {
	err = node.action(func(node *Node) error {
		// The node was found in the cache. Let's return it.
		if len(childrenDirs) == 0 {
			matchingNode = node
			return nil
		}

		// Check if any of the current node's children are parents of the current node.
		for i := range node.children {
			if node.children[i].name == childrenDirs[0] {
				matchingNode, err = node.children[i].findMatchingNode(childrenDirs[1:])
				return err
			}
		}

		// None of the current node's children are parents of the current node.
		// This means we need to start creating the searched node parents.
		newNode := CreateNewNode(childrenDirs[0], node)
		newNode.parent = node
		node.children = append(node.children, newNode)
		matchingNode, err = newNode.findMatchingNode(childrenDirs[1:])
		return err
	})
	return
}

func CreateNewNode(dirName string, parent *Node) *Node {
	return &Node{
		name:   dirName,
		parent: parent,
	}
}
