package python

import (
	piputils "github.com/jfrog/jfrog-cli-core/v2/utils/python"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

type AuditPipCommand struct {
	audit.AuditCommand
}

func NewEmptyAuditPipCommand() *AuditPipCommand {
	return &AuditPipCommand{AuditCommand: *audit.NewAuditCommand()}
}

func NewAuditPipCommand(auditCmd audit.AuditCommand) *AuditPipCommand {
	return &AuditPipCommand{AuditCommand: auditCmd}
}

func (apc *AuditPipCommand) Run() error {
	dependencyTree, err := apc.buildPipDependencyTree()
	if err != nil {
		return err
	}
	return apc.ScanDependencyTree(dependencyTree)
}

func (apc *AuditPipCommand) buildPipDependencyTree() ([]*services.GraphNode, error) {
	dependenciesGraph, rootDependenciesList, err := apc.getDependencies()
	if err != nil {
		return nil, err
	}
	var dependencyTree []*services.GraphNode
	for _, rootDep := range rootDependenciesList {
		parentNode := &services.GraphNode{
			Id:    pythonPackageTypeIdentifier + rootDep,
			Nodes: []*services.GraphNode{},
		}
		populatePythonDependencyTree(parentNode, dependenciesGraph)
		dependencyTree = append(dependencyTree, parentNode)
	}
	return dependencyTree, nil
}

func (apc *AuditPipCommand) getDependencies() (dependenciesGraph map[string][]string, rootDependencies []string, err error) {
	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		return
	}
	defer func() {
		log.Error("err:" + err.Error())
		e := fileutils.RemoveTempDir(tempDirPath)
		log.Error("err:" + e.Error())
		if err == nil {
			err = e
		}
	}()
	err = piputils.RunVirtualEnv(tempDirPath)
	if err != nil {
		return
	}

	// pip install project
	err = piputils.RunPipInstall(tempDirPath)
	if err != nil {
		return
	}

	// Run pipdeptree.py to get dependencies tree
	dependenciesGraph, rootDependencies, err = piputils.RunPipDepTree(tempDirPath)
	return
}

func (apc *AuditPipCommand) CommandName() string {
	return "xr_audit_pip"
}
