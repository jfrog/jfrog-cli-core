package nuget

import (
	"os"

	"github.com/jfrog/build-info-go/entities"
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
	dependencyTree, err := BuildNugetDependencyTree()
	if err != nil {
		return err
	}
	return anc.ScanDependencyTree(dependencyTree)
}

func BuildNugetDependencyTree() (nodes []*services.GraphNode, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	sol, err := solution.Load(wd, "")
	if err != nil {
		return
	}
	buildInfo, err := sol.BuildInfo("")
	if err != nil {
		return
	}
	return parseNugetDependencyTree(buildInfo), nil
}

func parseNugetDependencyTree(buildInfo *entities.BuildInfo) (nodes []*services.GraphNode) {
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

func (anc *AuditNugetCommand) CommandName() string {
	return "xr_audit_nuget"
}
