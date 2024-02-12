package java

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/xray"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
)

const (
	GavPackageTypeIdentifier = "gav://"
)

func BuildDependencyTree(depTreeParams DepTreeParams, tech coreutils.Technology) ([]*xrayUtils.GraphNode, map[string][]string, error) {
	if tech == coreutils.Maven {
		return buildMavenDependencyTree(&depTreeParams)
	}
	return buildGradleDependencyTree(&depTreeParams)
}

type DepTreeParams struct {
	UseWrapper              bool
	Server                  *config.ServerDetails
	DepsRepo                string
	IsMavenDepTreeInstalled bool
	IsCurationCmd           bool
	CurationCacheFolder     string
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
	Root  string                      `json:"root"`
	Nodes map[string]xray.DepTreeNode `json:"nodes"`
}

// Reads the output files of the gradle-dep-tree and maven-dep-tree plugins and returns them as a slice of GraphNodes.
// It takes the output of the plugin's run (which is a byte representation of a list of paths of the output files, separated by newlines) as input.
func getGraphFromDepTree(outputFilePaths string) (depsGraph []*xrayUtils.GraphNode, uniqueDepsMap map[string][]string, err error) {
	modules, err := parseDepTreeFiles(outputFilePaths)
	if err != nil {
		return
	}
	uniqueDepsMap = map[string][]string{}
	for _, module := range modules {
		moduleTree, moduleUniqueDeps := GetModuleTreeAndDependencies(module)
		depsGraph = append(depsGraph, moduleTree)
		for depToAdd, depTypes := range moduleUniqueDeps {
			uniqueDepsMap[depToAdd] = depTypes
		}
	}
	return
}

// Returns a dependency tree and a flat list of the module's dependencies for the given module
func GetModuleTreeAndDependencies(module *moduleDepTree) (*xrayUtils.GraphNode, map[string][]string) {
	moduleTreeMap := make(map[string]xray.DepTreeNode)
	moduleDeps := module.Nodes
	for depName, dependency := range moduleDeps {
		dependencyId := GavPackageTypeIdentifier + depName
		var childrenList []string
		for _, childName := range dependency.Children {
			childId := GavPackageTypeIdentifier + childName
			childrenList = append(childrenList, childId)
		}
		moduleTreeMap[dependencyId] = xray.DepTreeNode{
			Types:    dependency.Types,
			Children: childrenList,
		}
	}
	return xray.BuildXrayDependencyTree(moduleTreeMap, GavPackageTypeIdentifier+module.Root)
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

func (dtm *DepTreeManager) GetDepsRepo() string {
	return dtm.depsRepo
}
