package audit

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	npmPackageTypeIdentifier = "npm://"
)

func NewAuditNpmCommand(auditCmd AuditCommand) *AuditNpmCommand {
	return &AuditNpmCommand{AuditCommand: auditCmd}
}

type AuditNpmCommand struct {
	AuditCommand
	typeRestriction npmutils.TypeRestriction
}

func (auditCmd *AuditNpmCommand) SetNpmTypeRestriction(typeRestriction npmutils.TypeRestriction) *AuditNpmCommand {
	auditCmd.typeRestriction = typeRestriction
	return auditCmd
}

func (auditCmd *AuditNpmCommand) Run() (err error) {
	typeRestriction := auditCmd.typeRestriction

	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}
	npmVersion, npmExecutablePath, err := npmutils.GetNpmVersionAndExecPath()
	if err != nil {
		return err
	}
	packageInfo, err := npmutils.ReadPackageInfoFromPackageJson(currentDir, npmVersion)
	if err != nil {
		return err
	}
	// Calculate npm dependencies
	dependenciesList, err := npmutils.CalculateDependenciesList(typeRestriction, []string{}, npmExecutablePath, packageInfo.BuildInfoModuleId())
	if err != nil {
		return err
	}
	// Parse the dependencies into Xray dependency tree format
	rootNode := parseNpmDependenciesList(dependenciesList, packageInfo)

	return auditCmd.ScanDependencyTree([]*services.GraphNode{rootNode})
}

// Parse the dependencies into an Xray dependency tree format
func parseNpmDependenciesList(dependencies map[string]*npmutils.Dependency, packageInfo *npmutils.PackageInfo) (xrDependencyTree *services.GraphNode) {
	treeMap := make(map[string][]string)
	for dependencyId, dependency := range dependencies {
		dependencyId = npmPackageTypeIdentifier + dependencyId
		parent := npmPackageTypeIdentifier + dependency.GetPathToRoot()[0][0]
		if children, ok := treeMap[parent]; ok {
			treeMap[parent] = append(children, dependencyId)
		} else {
			treeMap[parent] = []string{dependencyId}
		}
	}
	return buildXrayDependencyTree(treeMap, npmPackageTypeIdentifier+packageInfo.BuildInfoModuleId())
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

func (auditCmd *AuditNpmCommand) CommandName() string {
	return "xr_audit_npm"
}
