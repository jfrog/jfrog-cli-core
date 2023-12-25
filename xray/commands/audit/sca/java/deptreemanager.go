package java

import (
	"encoding/json"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca"
	xrayutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
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

// Reads the output files of the gradle-dep-tree and maven-dep-tree plugins and returns them as a slice of GraphNodes.
// It takes the output of the plugin's run (which is a byte representation of a list of paths of the output files, separated by newlines) as input.
func getGraphFromDepTree(outputFilePaths string) (depsGraph []*xrayUtils.GraphNode, uniqueDeps []string, err error) {
	modules, err := parseDepTreeFiles(outputFilePaths)
	if err != nil {
		return
	}

	uniqueDepsSet := datastructures.MakeSet[string]()
	for _, module := range modules {
		moduleTreeMap := make(map[string][]string)
		moduleDeps := module.Nodes
		for depName, dependency := range moduleDeps {
			dependencyId := GavPackageTypeIdentifier + depName
			var childrenList []string
			for _, childName := range dependency.Children {
				childId := GavPackageTypeIdentifier + childName
				childrenList = append(childrenList, childId)
			}
			if len(childrenList) > 0 {
				moduleTreeMap[dependencyId] = childrenList
			}
		}
		moduleTree, moduleUniqueDeps := sca.BuildXrayDependencyTree(moduleTreeMap, GavPackageTypeIdentifier+module.Root)
		depsGraph = append(depsGraph, moduleTree)
		for _, depToAdd := range moduleUniqueDeps {
			uniqueDepsSet.Add(depToAdd)
		}
	}
	uniqueDeps = uniqueDepsSet.ToSlice()
	return
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
