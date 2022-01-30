package python

import (
	"os"
	"path/filepath"

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
	wd, err := os.Getwd()
	if err != nil {
		return
	}

	// Create temp dir to run all work outside users working directory
	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		return
	}

	err = os.Chdir(tempDirPath)
	if err != nil {
		return
	}

	defer func() {
		e := os.Chdir(wd)
		if err == nil {
			err = e
		}

		e = fileutils.RemoveTempDir(tempDirPath)
		if err == nil {
			err = e
		}
	}()

	err = fileutils.CopyDir(wd, tempDirPath, true, nil)
	if err != nil {
		return
	}

	// 'virtualenv venv'
	err = piputils.RunVirtualEnv()
	if err != nil {
		return
	}

	// 'pip install .'
	err = piputils.RunPipInstall()
	if err != nil {
		exist, requirementsErr := fileutils.IsFileExists(filepath.Join(tempDirPath, "requirements.txt"), false)
		if requirementsErr != nil || !exist {
			return
		}

		log.Debug("Failed running 'pip install .' , trying 'pip install -r requirements.txt' ")
		requirementsErr = piputils.RunPipInstallRequirements()
		if requirementsErr != nil {
			log.Error(requirementsErr)
			return
		}
	}

	// Run pipdeptree.py to get dependencies tree
	dependenciesGraph, rootDependencies, err = piputils.RunPipDepTree(piputils.GetVenvPythonExecPath())
	return
}

func (apc *AuditPipCommand) CommandName() string {
	return "xr_audit_pip"
}
