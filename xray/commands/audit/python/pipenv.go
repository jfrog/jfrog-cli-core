package python

import (
	"os"
	"path/filepath"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	pipenvutils "github.com/jfrog/jfrog-cli-core/v2/utils/python"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

func AuditPipenv(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails) (results []services.ScanResponse, isMultipleRootProject bool, err error) {
	graph, err := BuildPipenvDependencyTree()
	if err != nil {
		return
	}
	isMultipleRootProject = false
	results, err = audit.Scan([]*services.GraphNode{graph}, xrayGraphScanPrams, serverDetails)
	return
}

func BuildPipenvDependencyTree() (*services.GraphNode, error) {
	// Run pipenv graph to get dependencies tree
	dependenciesGraph, rootDependencies, err := pipenvutils.GetPipenvDependenciesGraph(".jfrog")
	if err != nil {
		return nil, err
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	rootNode := &services.GraphNode{
		Id:    pythonPackageTypeIdentifier + filepath.Base(workingDir),
		Nodes: []*services.GraphNode{},
	}
	for _, subDep := range rootDependencies {
		subDep := &services.GraphNode{
			Id:     pythonPackageTypeIdentifier + subDep,
			Nodes:  []*services.GraphNode{},
			Parent: rootNode,
		}
		populatePythonDependencyTree(subDep, dependenciesGraph)
		rootNode.Nodes = append(rootNode.Nodes, subDep)
	}
	return rootNode, nil
}
