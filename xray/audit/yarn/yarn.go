package yarn

import (
	biUtils "github.com/jfrog/build-info-go/build/utils"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
)

const (
	npmPackageTypeIdentifier = "npm://"
	yarnV2Version            = "2.0.0"
	YarnV1ErrorPrefix        = "jf audit is only supported for yarn v2 and above."
)

func BuildDependencyTree() (dependencyTree []*xrayUtils.GraphNode, err error) {
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return
	}
	executablePath, err := biUtils.GetYarnExecutable()
	if errorutils.CheckError(err) != nil {
		return
	}
	if err = logAndValidateYarnVersion(executablePath); err != nil {
		return
	}

	packageInfo, err := biUtils.ReadPackageInfoFromPackageJson(currentDir, nil)
	if errorutils.CheckError(err) != nil {
		return
	}
	// Calculate Yarn dependencies
	dependenciesMap, root, err := biUtils.GetYarnDependencies(executablePath, currentDir, packageInfo, log.Logger)
	if err != nil {
		return
	}
	// Parse the dependencies into Xray dependency tree format
	dependencyTree = []*xrayUtils.GraphNode{parseYarnDependenciesMap(dependenciesMap, getXrayDependencyId(root))}
	return
}

// Yarn audit is only supported from yarn v2.
func logAndValidateYarnVersion(executablePath string) error {
	versionStr, err := audit.GetExecutableVersion(executablePath)
	if errorutils.CheckError(err) != nil {
		return err
	}
	yarnVer := version.NewVersion(versionStr)
	if yarnVer.Compare(yarnV2Version) > 0 {
		return errorutils.CheckErrorf(YarnV1ErrorPrefix + "The current version is: " + versionStr)
	}
	return nil
}

// Parse the dependencies into a Xray dependency tree format
func parseYarnDependenciesMap(dependencies map[string]*biUtils.YarnDependency, rootXrayId string) (xrDependencyTree *xrayUtils.GraphNode) {
	treeMap := make(map[string][]string)
	for _, dependency := range dependencies {
		xrayDepId := getXrayDependencyId(dependency)
		var subDeps []string
		for _, subDepPtr := range dependency.Details.Dependencies {
			subDeps = append(subDeps, getXrayDependencyId(dependencies[biUtils.GetYarnDependencyKeyFromLocator(subDepPtr.Locator)]))
		}
		if len(subDeps) > 0 {
			treeMap[xrayDepId] = subDeps
		}
	}
	return audit.BuildXrayDependencyTree(treeMap, rootXrayId)
}

func getXrayDependencyId(yarnDependency *biUtils.YarnDependency) string {
	return npmPackageTypeIdentifier + yarnDependency.Name() + ":" + yarnDependency.Details.Version
}
