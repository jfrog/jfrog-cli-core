package scan

import (
	"encoding/json"
	"os"

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
	serverDetails    *config.ServerDetails
	workingDirectory string
	arguments        []string
	typeRestriction  npm.TypeRestriction
}

func (auditCmd *XrAuditNpmCommand) SetWorkingDirectory(dir string) *XrAuditNpmCommand {
	auditCmd.workingDirectory = dir
	return auditCmd
}

func (auditCmd *XrAuditNpmCommand) SetArguments(args []string) *XrAuditNpmCommand {
	auditCmd.arguments = args
	return auditCmd
}

func (auditCmd *XrAuditNpmCommand) SetNpmTypeRestriction(typeRestriction npm.TypeRestriction) *XrAuditNpmCommand {
	auditCmd.typeRestriction = typeRestriction
	return auditCmd
}

func (auditCmd *XrAuditNpmCommand) SetServerDetails(server *config.ServerDetails) *XrAuditNpmCommand {
	auditCmd.serverDetails = server
	return auditCmd
}

func (auditCmd *XrAuditNpmCommand) ServerDetails() (*config.ServerDetails, error) {
	return auditCmd.serverDetails, nil
}

func NewXrAuditNpmCommand() *XrAuditNpmCommand {
	return &XrAuditNpmCommand{}
}

func (na XrAuditNpmCommand) Run() (err error) {
	nca := npm.NewNpmCommandArgs("")
	nca.SetTypeRestriction(na.typeRestriction)

	currentDir, err := commandsutils.GetWorkingDirectory()
	if err != nil {
		return err
	}
	if na.workingDirectory == "" {
		na.workingDirectory = currentDir
	} else {
		err = os.Chdir(na.workingDirectory)
		if err != nil {
			return err
		}
		defer func() {
			err = os.Chdir(currentDir)
		}()
	}
	log.Debug("Working directory set to:", na.workingDirectory)

	packageInfo, err := commandsutils.ReadPackageInfoFromPackageJson(na.workingDirectory)
	if err != nil {
		return err
	}
	nca.SetPackageInfo(packageInfo)
	err = nca.SetNpmExecutable()
	if err != nil {
		return err
	}
	// Calculate npm dependencies
	err = nca.SetDependenciesList()
	if err != nil {
		return err
	}
	// Parse the dependencies into an Xray dependency tree format
	npmGraph := parseNpmDependenciesList(nca.GetDependenciesList(), packageInfo)
	xrayManager, err := commands.CreateXrayServiceManager(na.serverDetails)
	if err != nil {
		return err
	}
	params := services.NewXrayGraphScanParams()
	params.Graph = npmGraph
	scanId, err := xrayManager.ScanGraph(params)
	if err != nil {
		return err
	}

	scanResults, err := xrayManager.GetScanGraphResults(scanId)
	if err != nil {
		return err
	}
	return printTable(scanResults)

}

func printTable(res *services.ScanResponse) error {
	jsonOut, err := json.Marshal(res)
	print(string(jsonOut))
	return err
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
	return buildXrayDependencyTree(treeMap, NpmPackageTypeIdentifier+packageInfo.BuildInfoModuleId())
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
