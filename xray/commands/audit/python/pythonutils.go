package python

import (
	"github.com/jfrog/jfrog-client-go/xray/services"
	"os"
	"path/filepath"
	"strings"
)

const (
	pythonPackageTypeIdentifier = "pypi://"
)

func CreateDependencyTree(dependenciesGraph map[string][]string, rootDependencies []string) (*services.GraphNode, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	rootNode := &services.GraphNode{
		Id:    pythonPackageTypeIdentifier + filepath.Base(workingDir),
		Nodes: []*services.GraphNode{},
	}
	dependenciesGraph[filepath.Base(workingDir)] = rootDependencies
	populatePythonDependencyTree(rootNode, dependenciesGraph)

	return rootNode, nil
}

func populatePythonDependencyTree(currNode *services.GraphNode, dependenciesGraph map[string][]string) {
	if currNode.NodeHasLoop() {
		return
	}
	currDepChildren := dependenciesGraph[strings.TrimPrefix(currNode.Id, pythonPackageTypeIdentifier)]
	// Recursively create & append all node's dependencies.
	for _, dependency := range currDepChildren {
		childNode := &services.GraphNode{
			Id:     pythonPackageTypeIdentifier + dependency,
			Nodes:  []*services.GraphNode{},
			Parent: currNode,
		}
		currNode.Nodes = append(currNode.Nodes, childNode)
		populatePythonDependencyTree(childNode, dependenciesGraph)
	}
}
