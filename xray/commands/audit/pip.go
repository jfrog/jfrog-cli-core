package audit

import (
	piputils "github.com/jfrog/jfrog-cli-core/v2/utils/pip"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"strings"
)

const (
	pipPackageTypeIdentifier = "pypi://"
)

type AuditPipCommand struct {
	AuditCommand
}

func NewEmptyAuditPipCommand() *AuditPipCommand {
	return &AuditPipCommand{AuditCommand: *NewAuditCommand()}
}

func NewAuditPipCommand(auditCmd AuditCommand) *AuditPipCommand {
	return &AuditPipCommand{AuditCommand: auditCmd}
}

func (auditCmd *AuditPipCommand) Run() (err error) {
	rootNodes, err := auditCmd.buildPipDependencyTree()
	if err != nil {
		return err
	}
	return auditCmd.ScanDependencyTree(rootNodes)
}

func (auditCmd *AuditPipCommand) buildPipDependencyTree() ([]*services.GraphNode, error) {
	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		return nil, err
	}
	defer func() {
		e := fileutils.RemoveTempDir(tempDirPath)
		if err == nil {
			err = e
		}
	}()
	err = piputils.RunVirtualEnv(tempDirPath)
	if err != nil {
		return nil, err
	}

	// pip install project
	err = piputils.RunPipInstall(tempDirPath)
	if err != nil {
		return nil, err
	}

	// Run pipdeptree.py to get dependencies tree
	dependenciesGraph, parents, err := piputils.RunPipDepTree(tempDirPath)
	if err != nil {
		return nil, err
	}
	var modulesDependencyTrees []*services.GraphNode
	for _, parent := range parents {
		parentNode := &services.GraphNode{
			Id:    pipPackageTypeIdentifier + parent,
			Nodes: []*services.GraphNode{},
		}
		populatePipDependencyTree(parentNode, dependenciesGraph)
		modulesDependencyTrees = append(modulesDependencyTrees, parentNode)
	}

	return modulesDependencyTrees, err
}

func populatePipDependencyTree(currNode *services.GraphNode, dependenciesGraph map[string][]string) {
	if currNode.NodeHasLoop() {
		return
	}
	currDepChildren := dependenciesGraph[strings.TrimPrefix(currNode.Id, pipPackageTypeIdentifier)]
	// Recursively create & append all node's dependencies.
	for _, dependency := range currDepChildren {
		childNode := &services.GraphNode{
			Id:     pipPackageTypeIdentifier + dependency,
			Nodes:  []*services.GraphNode{},
			Parent: currNode,
		}
		currNode.Nodes = append(currNode.Nodes, childNode)
		populatePipDependencyTree(childNode, dependenciesGraph)
	}
}

func (auditCmd *AuditPipCommand) CommandName() string {
	return "xr_audit_pip"
}
