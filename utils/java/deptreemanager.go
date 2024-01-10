package java

import (
	"encoding/json"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	xrayutils "github.com/jfrog/jfrog-cli-security/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"os"
	"strings"
)

const (
	GavPackageTypeIdentifier = "gav://"
)

func BuildDependencyTree(params xrayutils.AuditParams, tech coreutils.Technology) ([]*xrayUtils.GraphNode, []string, error) {
	serverDetails, err := params.ServerDetails()
	if err != nil {
		return nil, nil, err
	}
	depTreeParams := &DepTreeParams{
		UseWrapper: params.UseWrapper(),
		Server:     serverDetails,
		DepsRepo:   params.DepsRepo(),
	}
	if tech == coreutils.Maven {
		return buildMavenDependencyTree(depTreeParams, params.IsMavenDepTreeInstalled())
	}
	return buildGradleDependencyTree(depTreeParams)
}

type DepTreeParams struct {
	UseWrapper bool
	Server     *config.ServerDetails
	DepsRepo   string
}

type DepTreeManager struct {
	server     *config.ServerDetails
	depsRepo   string
	useWrapper bool
}

func NewDepTreeManager(params *DepTreeParams) DepTreeManager {
	return DepTreeManager{useWrapper: params.UseWrapper, depsRepo: params.DepsRepo, server: params.Server}
}

// The structure of a dependency tree of a module in a Gradle/Maven project, as created by the gradle-dep-tree and maven-dep-tree plugins.
type moduleDepTree struct {
	Root  string                 `json:"root"`
	Nodes map[string]depTreeNode `json:"nodes"`
}

type depTreeNode struct {
	Children []string `json:"children"`
}

// getGraphFromDepTree reads the output files of the gradle-dep-tree and maven-dep-tree plugins and returns them as a slice of GraphNodes.
// It takes the output of the plugin's run (which is a byte representation of a list of paths of the output files, separated by newlines) as input.
func getGraphFromDepTree(outputFilePaths string) (depsGraph []*xrayUtils.GraphNode, uniqueDeps []string, err error) {
	modules, err := parseDepTreeFiles(outputFilePaths)
	if err != nil {
		return
	}
	uniqueDepsSet := datastructures.MakeSet[string]()
	for _, moduleTree := range modules {
		directDepId := GavPackageTypeIdentifier + moduleTree.Root
		directDependency := &xrayUtils.GraphNode{
			Id:    directDepId,
			Nodes: []*xrayUtils.GraphNode{},
		}
		uniqueDepsSet.Add(directDepId)
		populateDependencyTree(directDependency, moduleTree.Root, moduleTree, uniqueDepsSet)
		depsGraph = append(depsGraph, directDependency)
	}
	uniqueDeps = uniqueDepsSet.ToSlice()
	return
}

func populateDependencyTree(currNode *xrayUtils.GraphNode, currNodeId string, moduleTree *moduleDepTree, uniqueDepsSet *datastructures.Set[string]) {
	if currNode.NodeHasLoop() {
		return
	}
	for _, childId := range moduleTree.Nodes[currNodeId].Children {
		childGav := GavPackageTypeIdentifier + childId
		childNode := &xrayUtils.GraphNode{
			Id:     childGav,
			Nodes:  []*xrayUtils.GraphNode{},
			Parent: currNode,
		}
		uniqueDepsSet.Add(childGav)
		populateDependencyTree(childNode, childId, moduleTree, uniqueDepsSet)
		currNode.Nodes = append(currNode.Nodes, childNode)
	}
}

func parseDepTreeFiles(jsonFilePaths string) ([]*moduleDepTree, error) {
	outputFilePaths := strings.Split(strings.TrimSpace(jsonFilePaths), "\n")
	var modules []*moduleDepTree
	for _, path := range outputFilePaths {
		results, err := parseDepTreeFile(path)
		if err != nil {
			return nil, err
		}
		modules = append(modules, results)
	}
	return modules, nil
}

func parseDepTreeFile(path string) (results *moduleDepTree, err error) {
	depTreeJson, err := os.ReadFile(strings.TrimSpace(path))
	if errorutils.CheckError(err) != nil {
		return
	}
	results = &moduleDepTree{}
	err = errorutils.CheckError(json.Unmarshal(depTreeJson, &results))
	return
}

func getArtifactoryAuthFromServer(server *config.ServerDetails) (string, string, error) {
	username, password, err := server.GetAuthenticationCredentials()
	if err != nil {
		return "", "", err
	}
	if username == "" {
		return "", "", errorutils.CheckErrorf("a username is required for authenticating with Artifactory")
	}
	return username, password, nil
}
