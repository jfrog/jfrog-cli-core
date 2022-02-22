package python

import (
	"bytes"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"os"
	"os/exec"
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
	rootNode, err := apec.buildPipenvDependencyTree()
	if err != nil {
		return err
	}
	return apec.ScanDependencyTree([]*services.GraphNode{rootNode})
}

func (apec *AuditPipenvCommand) buildPipenvDependencyTree() (rootNode *services.GraphNode, err error) {
	dependenciesGraph, rootDependencies, err := apec.getDependencies()
	if err != nil {
		return nil, err
	}
	return CreateDependencyTree(dependenciesGraph, rootDependencies)
}

func (apec *AuditPipenvCommand) getDependencies() (dependenciesGraph map[string][]string, rootDependencies []string, err error) {
	// Set virtualenv path to venv dir
	err = os.Setenv("WORKON_HOME", ".jfrog")
	if err != nil {
		return
	}
	defer func() {
		e := os.Unsetenv("WORKON_HOME")
		if err == nil {
			err = e
		}
	}()
	// Run pipenv install
	var stderr bytes.Buffer
	pipenvInstall := exec.Command("pipenv", "install")
	pipenvInstall.Stderr = &stderr
	err = pipenvInstall.Run()
	if err != nil {
		return nil, nil, errorutils.CheckErrorf("pipenv install command failed: %s - %s", err.Error(), stderr.String())
	}
	// Run pipenv graph to get dependencies tree
	return pythonutils.GetPythonDependencies(pythonutils.Pipenv, "", "")
}

func (apec *AuditPipenvCommand) CommandName() string {
	return "xr_audit_pipenv"
}
