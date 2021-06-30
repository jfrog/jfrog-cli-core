package audit

import (
	"os"

	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	npmutils "github.com/jfrog/jfrog-cli-core/utils/npm"
	"github.com/jfrog/jfrog-cli-core/xray/commands"
	xrutils "github.com/jfrog/jfrog-cli-core/xray/utils"
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
	typeRestriction  npmutils.TypeRestriction
}

func (auditCmd *XrAuditNpmCommand) SetWorkingDirectory(dir string) *XrAuditNpmCommand {
	auditCmd.workingDirectory = dir
	return auditCmd
}

func (auditCmd *XrAuditNpmCommand) SetArguments(args []string) *XrAuditNpmCommand {
	auditCmd.arguments = args
	return auditCmd
}

func (auditCmd *XrAuditNpmCommand) SetNpmTypeRestriction(typeRestriction npmutils.TypeRestriction) *XrAuditNpmCommand {
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

func (auditCmd *XrAuditNpmCommand) Run() (err error) {
	typeRestriction := auditCmd.typeRestriction

	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}
	if auditCmd.workingDirectory == "" {
		auditCmd.workingDirectory = currentDir
	} else {
		err = os.Chdir(auditCmd.workingDirectory)
		if err != nil {
			return err
		}
		defer func() {
			err = os.Chdir(currentDir)
		}()
	}
	log.Debug("Working directory set to:", auditCmd.workingDirectory)

	packageInfo, err := coreutils.ReadPackageInfoFromPackageJson(auditCmd.workingDirectory)
	if err != nil {
		return err
	}
	npmExecutablePath, err := npmutils.FindNpmExecutable()
	if err != nil {
		return err
	}
	// Calculate npm dependencies
	dependenciesList, err := npmutils.CalculateDependenciesList(typeRestriction, []string{}, npmExecutablePath, packageInfo.BuildInfoModuleId())
	if err != nil {
		return err
	}
	// Parse the dependencies into an Xray dependency tree format
	npmGraph := parseNpmDependenciesList(dependenciesList, packageInfo)
	xrayManager, err := commands.CreateXrayServiceManager(auditCmd.serverDetails)
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
	if len(scanResults.Violations) > 0 {
		err = xrutils.PrintViolationsTable(scanResults.Violations)
	}
	if len(scanResults.Vulnerabilities) > 0 {
		xrutils.PrintVulnerabilitiesTable(scanResults.Vulnerabilities)
	}
	return err

}

func parseNpmDependenciesList(dependencies map[string]*npmutils.Dependency, packageInfo *coreutils.PackageInfo) (xrDependencyTree *services.GraphNode) {
	treeMap := make(map[string][]string)
	for dependencyId, dependency := range dependencies {
		dependencyId = NpmPackageTypeIdentifier + dependencyId
		parent := NpmPackageTypeIdentifier + dependency.GetPathToRoot()[0][0]
		if children, ok := treeMap[parent]; ok {
			treeMap[parent] = append(children, dependencyId)
		} else {
			treeMap[parent] = []string{dependencyId}
		}
	}
	return buildXrayDependencyTree(treeMap, NpmPackageTypeIdentifier+packageInfo.BuildInfoModuleId())
}

func buildXrayDependencyTree(treeHelper map[string][]string, nodeId string) *services.GraphNode {
	// Initialize the new node
	xrDependencyTree := &services.GraphNode{}
	xrDependencyTree.Id = nodeId
	xrDependencyTree.Nodes = []*services.GraphNode{}
	// Recursively create & append all node's dependencies.
	for _, dependency := range treeHelper[nodeId] {
		xrDependencyTree.Nodes = append(xrDependencyTree.Nodes, buildXrayDependencyTree(treeHelper, dependency))

	}
	return xrDependencyTree
}

func (auditCmd *XrAuditNpmCommand) CommandName() string {
	return "xr_audit_npm"
}
