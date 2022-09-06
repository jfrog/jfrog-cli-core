package npm

import (
	biutils "github.com/jfrog/build-info-go/build/utils"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	npmPackageTypeIdentifier = "npm://"
)

func AuditNpm(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails, args []string, progress ioUtils.ProgressMgr) (results []services.ScanResponse, isMultipleRootProject bool, err error) {
	graph, err := BuildNpmDependencyTree(args)
	if err != nil {
		return
	}
	isMultipleRootProject = false
	results, err = audit.Scan([]*services.GraphNode{graph}, xrayGraphScanPrams, serverDetails, progress, coreutils.Npm.ToString())
	return
}

func BuildNpmDependencyTree(npmArgs []string) (rootNode *services.GraphNode, err error) {

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
		return
	}
	// Parse the dependencies into Xray dependency tree format
	rootNode = parseNpmDependenciesList(dependenciesList, packageInfo)
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
