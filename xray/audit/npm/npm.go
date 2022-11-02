package npm

import (
	biutils "github.com/jfrog/build-info-go/build/utils"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	npmPackageTypeIdentifier = "npm://"
)

func BuildDependencyTree(npmArgs []string) (dependencyTree []*services.GraphNode, err error) {
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return
	}
	npmVersion, npmExecutablePath, err := biutils.GetNpmVersionAndExecPath(log.Logger)
	if err != nil {
		return
	}
	packageInfo, err := biutils.ReadPackageInfoFromPackageJson(currentDir, npmVersion)
	if err != nil {
		return
	}
	// Calculate npm dependencies
	dependenciesList, err := biutils.CalculateNpmDependenciesList(npmExecutablePath, currentDir, packageInfo.BuildInfoModuleId(), npmArgs, false, log.Logger)
	if err != nil {
		log.Info("Used npm version:", npmVersion)
		return
	}
	// Parse the dependencies into Xray dependency tree format
	dependencyTree = []*services.GraphNode{parseNpmDependenciesList(dependenciesList, packageInfo)}
	return
}

// Parse the dependencies into an Xray dependency tree format
func parseNpmDependenciesList(dependencies []buildinfo.Dependency, packageInfo *biutils.PackageInfo) (xrDependencyTree *services.GraphNode) {
	treeMap := make(map[string][]string)
	for _, dependency := range dependencies {
		dependencyId := npmPackageTypeIdentifier + dependency.Id
		parent := npmPackageTypeIdentifier + dependency.RequestedBy[0][0]
		if children, ok := treeMap[parent]; ok {
			treeMap[parent] = append(children, dependencyId)
		} else {
			treeMap[parent] = []string{dependencyId}
		}
	}
	return audit.BuildXrayDependencyTree(treeMap, npmPackageTypeIdentifier+packageInfo.BuildInfoModuleId())
}
