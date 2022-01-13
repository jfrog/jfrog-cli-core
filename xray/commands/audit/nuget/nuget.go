package nuget

import (
	"os"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/dotnet/solution"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	nugetPackageTypeIdentifier = "nuget://"
)

type AuditNugetCommand struct {
	audit.AuditCommand
}

func NewEmptyAuditNugetCommand() *AuditNugetCommand {
	return &AuditNugetCommand{AuditCommand: *audit.NewAuditCommand()}
}

func NewAuditNugetCommand(auditCmd audit.AuditCommand) *AuditNugetCommand {
	return &AuditNugetCommand{AuditCommand: auditCmd}
}

func (anc *AuditNugetCommand) Run() error {
	dependencyTree, err := anc.buildNugetDependencyTree()
	if err != nil {
		return err
	}
	return anc.ScanDependencyTree(dependencyTree)
}

func (anc *AuditNugetCommand) buildNugetDependencyTree() (nodes []*services.GraphNode, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	sol, err := solution.Load(wd, "")
	if err != nil {
		return nil, err
	}
	buildInfo, err := sol.BuildInfo("")
	if err != nil {
		return nil, err
	}
	treeMap := make(map[string][]string)
	for _, module := range buildInfo.Modules {
		for _, dependency := range module.Dependencies {
			dependencyId := nugetPackageTypeIdentifier + dependency.Id
			parent := nugetPackageTypeIdentifier + dependency.RequestedBy[0][0]
			if children, ok := treeMap[parent]; ok {
				treeMap[parent] = append(children, dependencyId)
			} else {
				treeMap[parent] = []string{dependencyId}
			}
		}
		nodes = append(nodes, audit.BuildXrayDependencyTree(treeMap, nugetPackageTypeIdentifier+module.Id))
	}

	return
}

func (apc *AuditNugetCommand) CommandName() string {
	return "xr_audit_nuget"
}
