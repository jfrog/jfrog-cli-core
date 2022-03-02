package python

import (
	"os"
	"path/filepath"

	pipenvutils "github.com/jfrog/jfrog-cli-core/v2/utils/python"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type AuditPipenvCommand struct {
	audit.AuditCommand
}

func NewEmptyAuditPipenvCommand() *AuditPipenvCommand {
	return &AuditPipenvCommand{AuditCommand: *audit.NewAuditCommand()}
}

func NewAuditPipenvCommand(auditCmd audit.AuditCommand) *AuditPipenvCommand {
	return &AuditPipenvCommand{AuditCommand: auditCmd}
}

func (apec *AuditPipenvCommand) Run() (err error) {
	rootNode, err := BuildPipenvDependencyTree()
	if err != nil {
		return err
	}
	return apec.ScanDependencyTree([]*services.GraphNode{rootNode})
}

func BuildPipenvDependencyTree() (*services.GraphNode, error) {
	// Run pipenv graph to get dependencies tree
	dependenciesGraph, rootDependencies, err := pipenvutils.GetPipenvDependenciesGraph(".jfrog")
	if err != nil {
		return nil, err
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	rootNode := &services.GraphNode{
		Id:    pythonPackageTypeIdentifier + filepath.Base(workingDir),
		Nodes: []*services.GraphNode{},
	}
	for _, subDep := range rootDependencies {
		subDep := &services.GraphNode{
			Id:     pythonPackageTypeIdentifier + subDep,
			Nodes:  []*services.GraphNode{},
			Parent: rootNode,
		}
		populatePythonDependencyTree(subDep, dependenciesGraph)
		rootNode.Nodes = append(rootNode.Nodes, subDep)
	}
	return rootNode, nil
}

func (apec *AuditPipenvCommand) CommandName() string {
	return "xr_audit_pipenv"
}
