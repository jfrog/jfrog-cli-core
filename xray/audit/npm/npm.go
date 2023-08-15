package npm

import (
	biutils "github.com/jfrog/build-info-go/build/utils"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/jfrog/jfrog-client-go/utils/log"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"golang.org/x/exp/slices"
)

const (
	npmPackageTypeIdentifier = "npm://"
	ignoreScriptsFlag        = "--ignore-scripts"
)

func BuildDependencyTree(npmArgs []string) (dependencyTree []*xrayUtils.GraphNode, err error) {
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
	npmArgs = addIgnoreScriptsFlag(npmArgs)

	// Calculate npm dependencies
	dependenciesMap, err := biutils.CalculateDependenciesMap(npmExecutablePath, currentDir, packageInfo.BuildInfoModuleId(), npmArgs, log.Logger)
	if err != nil {
		log.Info("Used npm version:", npmVersion.GetVersion())
		return
	}
	var dependenciesList []buildinfo.Dependency
	for _, dependency := range dependenciesMap {
		dependenciesList = append(dependenciesList, dependency.Dependency)
	}

	// Parse the dependencies into Xray dependency tree format
	dependencyTree = []*xrayUtils.GraphNode{parseNpmDependenciesList(dependenciesList, packageInfo)}
	return
}

// Add the --ignore-scripts to prevent execution of npm scripts during npm install.
func addIgnoreScriptsFlag(npmArgs []string) []string {
	if !slices.Contains(npmArgs, ignoreScriptsFlag) {
		return append(npmArgs, ignoreScriptsFlag)
	}
	return npmArgs
}

// Parse the dependencies into an Xray dependency tree format
func parseNpmDependenciesList(dependencies []buildinfo.Dependency, packageInfo *biutils.PackageInfo) (xrDependencyTree *xrayUtils.GraphNode) {
	treeMap := make(map[string][]string)
	for _, dependency := range dependencies {
		dependencyId := npmPackageTypeIdentifier + dependency.Id
		for _, requestedByNode := range dependency.RequestedBy {
			parent := npmPackageTypeIdentifier + requestedByNode[0]
			if children, ok := treeMap[parent]; ok {
				treeMap[parent] = appendUniqueChild(children, dependencyId)
			} else {
				treeMap[parent] = []string{dependencyId}
			}
		}
	}
	return audit.BuildXrayDependencyTree(treeMap, npmPackageTypeIdentifier+packageInfo.BuildInfoModuleId())
}

func appendUniqueChild(children []string, candidateDependency string) []string {
	for _, existingChild := range children {
		if existingChild == candidateDependency {
			return children
		}
	}
	return append(children, candidateDependency)
}
