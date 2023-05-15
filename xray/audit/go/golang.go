package _go

import (
	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"os"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	goutils "github.com/jfrog/jfrog-cli-core/v2/utils/golang"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
)

const (
	goPackageTypeIdentifier = "go://"
	goSourceCodePrefix      = "github.com/golang/go:v"
)

func BuildDependencyTree(server *config.ServerDetails, remoteGoRepo string) (dependencyTree []*xrayUtils.GraphNode, err error) {
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return
	}
	if remoteGoRepo != "" {
		if err = setGoProxy(server, remoteGoRepo); err != nil {
			return
		}
	}
	// Calculate go dependencies graph
	dependenciesGraph, err := goutils.GetDependenciesGraph(currentDir)
	if err != nil {
		return
	}
	// Calculate go dependencies list
	dependenciesList, err := goutils.GetDependenciesList(currentDir)
	if err != nil {
		return
	}
	// Get root module name
	rootModuleName, err := goutils.GetModuleName(currentDir)
	if err != nil {
		return
	}
	// Parse the dependencies into Xray dependency tree format
	rootNode := &xrayUtils.GraphNode{
		Id:    goPackageTypeIdentifier + rootModuleName,
		Nodes: []*xrayUtils.GraphNode{},
	}
	populateGoDependencyTree(rootNode, dependenciesGraph, dependenciesList)

	// Add go version as child node to dependencies tree
	err = addGoVersionAsDependency(rootNode)
	if err != nil {
		return
	}

	dependencyTree = []*xrayUtils.GraphNode{rootNode}
	return
}

func setGoProxy(server *config.ServerDetails, remoteGoRepo string) error {
	repoUrl, err := goutils.GetArtifactoryRemoteRepoUrl(server, remoteGoRepo)
	if err != nil {
		return err
	}
	repoUrl += "|direct"
	return os.Setenv("GOPROXY", repoUrl)
}

func populateGoDependencyTree(currNode *xrayUtils.GraphNode, dependenciesGraph map[string][]string, dependenciesList map[string]bool) {
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
		childNode := &xrayUtils.GraphNode{
			Id:     goPackageTypeIdentifier + childName,
			Nodes:  []*xrayUtils.GraphNode{},
			Parent: currNode,
		}
		currNode.Nodes = append(currNode.Nodes, childNode)
		populateGoDependencyTree(childNode, dependenciesGraph, dependenciesList)
	}
}

func addGoVersionAsDependency(rootNode *xrayUtils.GraphNode) error {
	goVersion, err := utils.GetParsedGoVersion()
	if err != nil {
		return err
	}
	// Convert "go1.17.3" to "github.com/golang/go:v1.17.3"
	goVersionID := strings.ReplaceAll(goVersion.GetVersion(), "go", goSourceCodePrefix)
	rootNode.Nodes = append(rootNode.Nodes, &xrayUtils.GraphNode{
		Id:    goPackageTypeIdentifier + goVersionID,
		Nodes: []*xrayUtils.GraphNode{},
	})
	return nil
}
