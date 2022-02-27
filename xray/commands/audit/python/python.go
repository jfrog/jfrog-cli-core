package python

import (
	"fmt"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/python"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	pythonPackageTypeIdentifier = "pypi://"
)

type AuditPythonCommand struct {
	pythonTool pythonutils.PythonTool
	audit.AuditCommand
}

func NewEmptyAuditPythonCommand(projectType utils.ProjectType) *AuditPythonCommand {
	return &AuditPythonCommand{AuditCommand: *audit.NewAuditCommand(), pythonTool: python.GetPythonTool(projectType)}
}

func (apc *AuditPythonCommand) Run() error {
	dependencyTree, err := apc.buildDependencyTree()
	if err != nil {
		return err
	}
	return apc.ScanDependencyTree([]*services.GraphNode{dependencyTree})
}

func (apc *AuditPythonCommand) buildDependencyTree() (*services.GraphNode, error) {
	dependenciesGraph, rootDependenciesList, err := apc.getDependencies()
	if err != nil {
		return nil, err
	}
	return createDependencyTree(dependenciesGraph, rootDependenciesList)
}

func (apc *AuditPythonCommand) getDependencies() (dependenciesGraph map[string][]string, rootDependencies []string, err error) {
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

	restoreEnv, err := apc.RunPythonInstall(tempDirPath)
	if err != nil {
		return
	}
	defer func() {
		e := restoreEnv()
		if err == nil {
			err = e
		}
	}()

	localDependenciesPath, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return
	}
	dependenciesGraph, rootDependencies, err = pythonutils.GetPythonDependencies(apc.pythonTool, tempDirPath, localDependenciesPath)
	return
}

func (apc *AuditPythonCommand) RunPythonInstall(tempDirPath string) (restoreEnv func() error, err error) {
	switch apc.pythonTool {
	case pythonutils.Pip:
		restoreEnv, err = pythonutils.SetVirtualEnvPath()
		if err != nil {
			return
		}
		// Run pip install
		var output []byte
		output, err = exec.Command("pip", "install", ".").CombinedOutput()
		if err != nil {
			err = errorutils.CheckErrorf("pip install command failed: %s - %s", err.Error(), output)
			exist, requirementsErr := fileutils.IsFileExists(filepath.Join(tempDirPath, "requirements.txt"), false)
			if requirementsErr != nil || !exist {
				return
			}
			log.Debug("Failed running 'pip install .' , trying 'pip install -r requirements.txt' ")
			// Run pip install -r requirements
			output, requirementsErr = exec.Command("pip", "install", "-r", "requirements.txt").CombinedOutput()
			if requirementsErr != nil {
				log.Error(fmt.Sprintf("pip install -r requirements.txt command failed: %s - %s", err.Error(), output))
				return
			}
			err = nil
		}
	case pythonutils.Pipenv:
		// Set virtualenv path to venv dir
		err = os.Setenv("WORKON_HOME", ".jfrog")
		if err != nil {
			return
		}
		restoreEnv = func() error {
			return os.Unsetenv("WORKON_HOME")
		}
		// Run pipenv install
		var output []byte
		output, err = exec.Command("pipenv", "install").CombinedOutput()
		if err != nil {
			err = errorutils.CheckErrorf("pipenv install command failed: %s - %s", err.Error(), output)
		}
	}
	return
}

func createDependencyTree(dependenciesGraph map[string][]string, rootDependencies []string) (*services.GraphNode, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	rootNode := &services.GraphNode{
		Id:    pythonPackageTypeIdentifier + filepath.Base(workingDir),
		Nodes: []*services.GraphNode{},
	}
	dependenciesGraph[filepath.Base(workingDir)] = rootDependencies
	populatePythonDependencyTree(rootNode, dependenciesGraph)

	return rootNode, nil
}

func populatePythonDependencyTree(currNode *services.GraphNode, dependenciesGraph map[string][]string) {
	if currNode.NodeHasLoop() {
		return
	}
	currDepChildren := dependenciesGraph[strings.TrimPrefix(currNode.Id, pythonPackageTypeIdentifier)]
	// Recursively create & append all node's dependencies.
	for _, dependency := range currDepChildren {
		childNode := &services.GraphNode{
			Id:     pythonPackageTypeIdentifier + dependency,
			Nodes:  []*services.GraphNode{},
			Parent: currNode,
		}
		currNode.Nodes = append(currNode.Nodes, childNode)
		populatePythonDependencyTree(childNode, dependenciesGraph)
	}
}

func (apc *AuditPythonCommand) CommandName() string {
	return "xr_audit_python"
}
