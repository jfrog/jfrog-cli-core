package _go

import (
	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"os"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	goutils "github.com/jfrog/jfrog-cli-core/v2/utils/golang"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	goPackageTypeIdentifier = "go://"
	goSourceCodePrefix      = "github.com/golang/go:v"
)

func BuildDependencyTree(server *config.ServerDetails, remoteGoRepo string) (dependencyTree []*services.GraphNode, err error) {
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
	dependenciesGraph = mergeDependencies(dependenciesGraph, dependenciesList)
	// Get root module name
	rootModuleName, err := goutils.GetModuleName(currentDir)
	if err != nil {
		return
	}
	// Parse the dependencies into Xray dependency tree format
	rootNode := &services.GraphNode{
		Id:    goPackageTypeIdentifier + rootModuleName,
		Nodes: []*services.GraphNode{},
	}
	audit.PopulateDependencyTree(rootNode, dependenciesGraph, goPackageTypeIdentifier)

	// Add go version as child node to dependencies tree
	err = addGoVersionAsDependency(rootNode)
	if err != nil {
		return
	}

	dependencyTree = []*services.GraphNode{rootNode}
	return
}

func mergeDependencies(dependenciesGraph map[string][]string, dependenciesList map[string]bool) map[string][]string {
	// 'go list all' is more accurate than 'go graph' so we filter out deps that don't exist in go list
	mergedDepMap := make(map[string][]string)
	for dependencyName, children := range dependenciesGraph {
		for _, descendant := range children {
			if dependenciesList[descendant] {
				mergedDepMap[dependencyName] = append(mergedDepMap[dependencyName], descendant)
			}
		}
	}
	return mergedDepMap
}

func setGoProxy(server *config.ServerDetails, remoteGoRepo string) error {
	repoUrl, err := goutils.GetArtifactoryRemoteRepoUrl(server, remoteGoRepo)
	if err != nil {
		return err
	}
	repoUrl += "|direct"
	return os.Setenv("GOPROXY", repoUrl)
}

func addGoVersionAsDependency(rootNode *services.GraphNode) error {
	goVersion, err := utils.GetParsedGoVersion()
	if err != nil {
		return err
	}
	// Convert "go1.17.3" to "github.com/golang/go:v1.17.3"
	goVersionID := strings.Replace(goVersion.GetVersion(), "go", goSourceCodePrefix, -1)
	rootNode.Nodes = append(rootNode.Nodes, &services.GraphNode{
		Id:    goPackageTypeIdentifier + goVersionID,
		Nodes: []*services.GraphNode{},
	})
	return nil
}
