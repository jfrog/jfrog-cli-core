package npm

import (
	biutils "github.com/jfrog/build-info-go/build/utils"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	npmPackageTypeIdentifier = "npm://"
)

func NewAuditNpmCommand(auditCmd audit.AuditCommand) *AuditNpmCommand {
	return &AuditNpmCommand{AuditCommand: auditCmd}
}

type AuditNpmCommand struct {
	audit.AuditCommand
	typeRestriction biutils.TypeRestriction
}

func (auditCmd *AuditNpmCommand) SetNpmTypeRestriction(typeRestriction biutils.TypeRestriction) *AuditNpmCommand {
	auditCmd.typeRestriction = typeRestriction
	return auditCmd
}

func (auditCmd *AuditNpmCommand) Run() (err error) {
	typeRestriction := auditCmd.typeRestriction

	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}
	npmVersion, npmExecutablePath, err := biutils.GetNpmVersionAndExecPath(log.Logger)
	if err != nil {
		return err
	}
	packageInfo, err := biutils.ReadPackageInfoFromPackageJson(currentDir, npmVersion)
	if err != nil {
		return err
	}
	// Calculate npm dependencies
	dependenciesList, err := biutils.CalculateDependenciesList(typeRestriction, npmExecutablePath, currentDir, packageInfo.BuildInfoModuleId(), []string{}, nil, 1, log.Logger)
	if err != nil {
		return err
	}
	// Parse the dependencies into Xray dependency tree format
	rootNode := parseNpmDependenciesList(dependenciesList, packageInfo)
	return auditCmd.ScanDependencyTree([]*services.GraphNode{rootNode})
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

func (auditCmd *AuditNpmCommand) CommandName() string {
	return "xr_audit_npm"
}
