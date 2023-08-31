package nuget

import (
	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-client-go/utils/log"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"os"

	"github.com/jfrog/build-info-go/build/utils/dotnet/solution"
	"github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
)

const (
	nugetPackageTypeIdentifier = "nuget://"
)

func BuildDependencyTree() (dependencyTree []*xrayUtils.GraphNode, uniqueDeps []string, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	sol, err := solution.Load(wd, "", log.Logger)
	if err != nil {
		return
	}
	buildInfo, err := sol.BuildInfo("", log.Logger)
	if err != nil {
		return
	}
	dependencyTree, uniqueDeps = parseNugetDependencyTree(buildInfo)
	return
}

func parseNugetDependencyTree(buildInfo *entities.BuildInfo) (nodes []*xrayUtils.GraphNode, allUniqueDeps []string) {
	uniqueDepsSet := datastructures.MakeSet[string]()
	for _, module := range buildInfo.Modules {
		treeMap := make(map[string][]string)
		for _, dependency := range module.Dependencies {
			dependencyId := nugetPackageTypeIdentifier + dependency.Id
			parent := nugetPackageTypeIdentifier + dependency.RequestedBy[0][0]
			if children, ok := treeMap[parent]; ok {
				treeMap[parent] = append(children, dependencyId)
			} else {
				treeMap[parent] = []string{dependencyId}
			}
		}
		dependencyTree, uniqueDeps := audit.BuildXrayDependencyTree(treeMap, nugetPackageTypeIdentifier+module.Id)
		nodes = append(nodes, dependencyTree)
		for _, uniqueDep := range uniqueDeps {
			uniqueDepsSet.Add(uniqueDep)
		}
	}
	allUniqueDeps = uniqueDepsSet.ToSlice()
	return
}
