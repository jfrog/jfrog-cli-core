package python

import (
	"bytes"
	"fmt"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"os"
	"os/exec"
	"path/filepath"
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
	return apc.ScanDependencyTree([]*services.GraphNode{dependencyTree})
}

func (apc *AuditPipCommand) buildPipDependencyTree() (*services.GraphNode, error) {
	dependenciesGraph, rootDependenciesList, err := apc.getDependencies()
	if err != nil {
		return nil, err
	}
	return CreateDependencyTree(dependenciesGraph, rootDependenciesList)
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
	venvPath, err := pythonutils.RunVirtualEnv()
	if err != nil {
		return
	}
	pipVenvPath := filepath.Join(venvPath, "pip")

	// Run pip install
	var stderr bytes.Buffer
	pipInstall := exec.Command(pipVenvPath, "install", ".")
	pipInstall.Stderr = &stderr
	pipInstall.Stdout = &stderr
	err = pipInstall.Run()
	if err != nil {
		err = errorutils.CheckErrorf("pip install command failed: %s - %s", err.Error(), stderr.String())

		exist, requirementsErr := fileutils.IsFileExists(filepath.Join(tempDirPath, "requirements.txt"), false)
		if requirementsErr != nil || !exist {
			return
		}
		log.Debug("Failed running 'pip install .' , trying 'pip install -r requirements.txt' ")
		// Run pip install -r requirements
		var stderr bytes.Buffer
		pipRequirements := exec.Command(pipVenvPath, "install", "-r", "requirements.txt")
		pipRequirements.Stderr = &stderr
		requirementsErr = pipRequirements.Run()
		if requirementsErr != nil {
			log.Error(fmt.Sprintf("pip install -r requirements.txt command failed: %s - %s", err.Error(), stderr.String()))
			return
		}
	}

	// Run pipdeptree.py to get dependencies tree
	localDependenciesPath, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return
	}
	pythonVenvPath := filepath.Join(venvPath, "python")
	dependenciesGraph, rootDependencies, err = pythonutils.GetPythonDependencies(pythonutils.Pip, pythonVenvPath, localDependenciesPath)
	return
}

func (apc *AuditPipCommand) CommandName() string {
	return "xr_audit_pip"
}
