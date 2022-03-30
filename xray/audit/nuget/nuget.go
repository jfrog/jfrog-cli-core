package nuget

import (
	"os"

	"github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/dotnet/solution"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	nugetPackageTypeIdentifier = "nuget://"
)

func AuditNuget(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails) (results []services.ScanResponse, isMultipleRootProject bool, err error) {
	graph, err := BuildNugetDependencyTree()
	if err != nil {
		return
	}
	isMultipleRootProject = len(graph) > 1
	results, err = audit.Scan(graph, xrayGraphScanPrams, serverDetails)
	return
}

func BuildNugetDependencyTree() (nodes []*services.GraphNode, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	sol, err := solution.Load(wd, "")
	if err != nil {
		return
	}
	buildInfo, err := sol.BuildInfo("")
	if err != nil {
		return
	}
	return parseNugetDependencyTree(buildInfo), nil
}

func parseNugetDependencyTree(buildInfo *entities.BuildInfo) (nodes []*services.GraphNode) {
	treeMap := make(map[string][]string)
	for _, module := range buildInfo.Modules {
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
