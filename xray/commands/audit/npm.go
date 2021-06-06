package scan

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/commands/npm"
	commandsutils "github.com/jfrog/jfrog-cli-core/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-cli-core/xray/commands"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	NpmPackageTypeIdentifier = "npm://"
)

type XrAuditNpmCommand struct {
	serverDetails   *config.ServerDetails
	arguments       []string
	typeRestriction npm.TypeRestriction
}

func NewXrNpmScanCommand() *XrAuditNpmCommand {
	return &XrAuditNpmCommand{}
}

func (na XrAuditNpmCommand) Run() (*services.ScanResponse, error) {
	nca := npm.NewNpmCommandArgs("")
	nca.SetTypeRestriction(na.typeRestriction)
	err := nca.SetNpmExecutable()
	if err != nil {
		return nil, err
	}
	// Calculate npm dependencies
	err = nca.SetDependenciesList()
	if err != nil {
		return nil, err
	}
	workingDirectory, err := commandsutils.GetWorkingDirectory()
	if err != nil {
		return nil, err
	}
	log.Debug("Working directory set to:", workingDirectory)
	packageInfo, err := commandsutils.ReadPackageInfoFromPackageJson(workingDirectory)
	// Parse the dependencies into an Xray dependency tree format
	npmGraph := parseNpmDependenciesList(nca.GetDependenciesList(), packageInfo)
	xrayManager, err := commands.CreateXrayServiceManager(na.serverDetails)
	if err != nil {
		return nil, err
	}
	params := services.NewXrayGraphScanParams()
	params.Graph = npmGraph
	scanId, err := xrayManager.ScanGraph(params)
	if err != nil {
		return nil, err
	}
	return xrayManager.GetScanGraphResults(scanId)

}

func parseNpmDependenciesList(dependencies map[string]*npm.Dependency, packageInfo *commandsutils.PackageInfo) (xrDependencyTree *services.GraphNode) {
	treeMap := make(map[string][]string)
	for dependencyId, dependency := range dependencies {
		dependencyId = NpmPackageTypeIdentifier + dependencyId
		// Because we are dealing with a
		father := NpmPackageTypeIdentifier + dependency.GetPathToRoot()[0][0]
		if sons, ok := treeMap[father]; ok {
			treeMap[father] = append(sons, dependencyId)
		} else {
			treeMap[father] = []string{dependencyId}
		}
	}
	return buildXrayDependencyTree(treeMap, packageInfo.BuildInfoModuleId())
}

func buildXrayDependencyTree(treeHelper map[string][]string, node string) *services.GraphNode {
	// Initialize the new node
	xrDependencyTree := &services.GraphNode{}
	xrDependencyTree.Id = node
	xrDependencyTree.Nodes = []*services.GraphNode{}
	// Recursively create & append all node's dependencies.
	for _, dependency := range treeHelper[node] {
		xrDependencyTree.Nodes = append(xrDependencyTree.Nodes, buildXrayDependencyTree(treeHelper, dependency))

	}
	return xrDependencyTree
}

func (na *XrAuditNpmCommand) CommandName() string {
	return "xr_audit_npm"
}
