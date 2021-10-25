package audit

import (
	pipenvutils "github.com/jfrog/jfrog-cli-core/v2/utils/python"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"os"
	"path/filepath"
)

type AuditPipenvCommand struct {
	AuditCommand
}

func NewEmptyAuditPipenvCommand() *AuditPipenvCommand {
	return &AuditPipenvCommand{AuditCommand: *NewAuditCommand()}
}

func NewAuditPipenvCommand(auditCmd AuditCommand) *AuditPipenvCommand {
	return &AuditPipenvCommand{AuditCommand: auditCmd}
}

func (apec *AuditPipenvCommand) Run() (err error) {
	rootNode, err := apec.buildPipenvDependencyTree()
	if err != nil {
		return err
	}
	return apec.runScanGraph([]*services.GraphNode{rootNode})
}

func (apec *AuditPipenvCommand) buildPipenvDependencyTree() (*services.GraphNode, error) {
	// Run pipenv graph to get dependencies tree
	dependenciesGraph, rootDependencies, err := pipenvutils.GetPipenvDependenciesGraph(".jfrog")
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
