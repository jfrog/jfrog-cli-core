package yarn

import (
	biutils "github.com/jfrog/build-info-go/build/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	npmPackageTypeIdentifier = "npm://"
)

func BuildDependencyTree() (dependencyTree []*services.GraphNode, err error) {
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return
	}
	executablePath, err := biutils.GetYarnExecutable()
	if errorutils.CheckError(err) != nil {
		return
	}
	defer func() {
		if err != nil && executablePath != "" {
			audit.LogExecutableVersion(executablePath)
		}
	}()
	packageInfo, err := biutils.ReadPackageInfoFromPackageJson(currentDir, nil)
	if errorutils.CheckError(err) != nil {
		return
	}
	// Calculate Yarn dependencies
	dependenciesMap, _, err := biutils.GetYarnDependencies(executablePath, currentDir, packageInfo, log.Logger)
	if err != nil {
		return
	}
	// Parse the dependencies into Xray dependency tree format
	dependencyTree = []*services.GraphNode{parseYarnDependenciesMap(dependenciesMap, packageInfo)}
	return
}

// Parse the dependencies into an Xray dependency tree format
func parseYarnDependenciesMap(dependencies map[string]*biutils.YarnDependency, packageInfo *biutils.PackageInfo) (xrDependencyTree *services.GraphNode) {
	treeMap := make(map[string][]string)
	for _, dependency := range dependencies {
		xrayDepId := getXrayDependencyId(dependency)
		var subDeps []string
		for _, subDepPtr := range dependency.Details.Dependencies {
			subDeps = append(subDeps, getXrayDependencyId(dependencies[biutils.GetYarnDependencyKeyFromLocator(subDepPtr.Locator)]))
		}
		if len(subDeps) > 0 {
			treeMap[xrayDepId] = subDeps
		}
	}
	return audit.BuildXrayDependencyTree(treeMap, npmPackageTypeIdentifier+packageInfo.BuildInfoModuleId())
}

func getXrayDependencyId(yarnDependency *biutils.YarnDependency) string {
	return npmPackageTypeIdentifier + yarnDependency.Name() + ":" + yarnDependency.Details.Version
}
