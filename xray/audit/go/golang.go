package _go

import (
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	goutils "github.com/jfrog/jfrog-cli-core/v2/utils/golang"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	goPackageTypeIdentifier = "go://"
)

func AuditGo(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails) (results []services.ScanResponse, isMultipleRootProject bool, err error) {
	graph, err := BuildGoDependencyTree()
	if err != nil {
		return
	}
	isMultipleRootProject = false
	results, err = audit.Scan([]*services.GraphNode{graph}, xrayGraphScanPrams, serverDetails)
	return
}

func BuildGoDependencyTree() (*services.GraphNode, error) {
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return nil, err
	}
	// Calculate go dependencies graph
	dependenciesGraph, err := goutils.GetDependenciesGraph(currentDir)
	if err != nil {
		return nil, err
	}
	// Calculate go dependencies list
	dependenciesList, err := goutils.GetDependenciesList(currentDir)
	if err != nil {
		return nil, err
	}
	// Get root module name
	rootModuleName, err := goutils.GetModuleName(currentDir)
	if err != nil {
		return nil, err
	}
	// Parse the dependencies into Xray dependency tree format
	rootNode := &services.GraphNode{
		Id:    goPackageTypeIdentifier + rootModuleName,
		Nodes: []*services.GraphNode{},
	}
	populateGoDependencyTree(rootNode, dependenciesGraph, dependenciesList)
	return rootNode, err
}

func populateGoDependencyTree(currNode *services.GraphNode, dependenciesGraph map[string][]string, dependenciesList map[string]bool) {
	if currNode.NodeHasLoop() {
		return
	}
	currDepChildren := dependenciesGraph[strings.TrimPrefix(currNode.Id, goPackageTypeIdentifier)]
	// Recursively create & append all node's dependencies.
	for _, childName := range currDepChildren {
		if !dependenciesList[childName] {
			// 'go list all' is more accurate than 'go graph' so we filter out deps that don't exist in go list
			continue
		}
		childNode := &services.GraphNode{
			Id:     goPackageTypeIdentifier + childName,
			Nodes:  []*services.GraphNode{},
			Parent: currNode,
		}
		currNode.Nodes = append(currNode.Nodes, childNode)
		populateGoDependencyTree(childNode, dependenciesGraph, dependenciesList)
	}
}
