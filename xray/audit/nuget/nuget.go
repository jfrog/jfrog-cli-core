package nuget

import (
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"

	"github.com/jfrog/build-info-go/build/utils/dotnet/solution"
	"github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	nugetPackageTypeIdentifier = "nuget://"
)

func BuildDependencyTree() (dependencyTree []*services.GraphNode, err error) {
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
	dependencyTree = parseNugetDependencyTree(buildInfo)
	return
}

func parseNugetDependencyTree(buildInfo *entities.BuildInfo) (nodes []*services.GraphNode) {
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
		nodes = append(nodes, audit.BuildXrayDependencyTree(treeMap, nugetPackageTypeIdentifier+module.Id))
	}
	return
}
