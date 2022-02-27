package python

import (
	"bytes"
	"errors"
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
	"runtime"
	"strings"
)

const (
	pythonPackageTypeIdentifier = "pypi://"
)

type AuditPythonCommand struct {
	pythonTool pythonutils.PythonTool
	audit.AuditCommand
}

func NewEmptyPythonCommand(pythonTool pythonutils.PythonTool) *AuditPythonCommand {
	return &AuditPythonCommand{AuditCommand: *audit.NewAuditCommand(), pythonTool: pythonTool}
}

func NewAuditPythonCommand(auditCmd audit.AuditCommand, pythonTool pythonutils.PythonTool) *AuditPythonCommand {
	return &AuditPythonCommand{AuditCommand: auditCmd, pythonTool: pythonTool}
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
		restoreEnv, err = SetPipVirtualEnvPath()
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

// Execute virtualenv command: "virtualenv venvdir" / "python3 -m venv venvdir" and set path
func SetPipVirtualEnvPath() (func() error, error) {
	var cmdArgs []string
	execPath, err := exec.LookPath("virtualenv")
	if err != nil || execPath == "" {
		// If virtualenv not installed try "venv"
		if runtime.GOOS == "windows" {
			// If the OS is Windows try using Py Launcher: "py -3 -m venv"
			execPath, err = exec.LookPath("py")
			cmdArgs = append(cmdArgs, "-3", "-m", "venv")
		} else {
			// If the OS is Linux try using python3 executable: "python3 -m venv"
			execPath, err = exec.LookPath("python3")
			cmdArgs = append(cmdArgs, "-m", "venv")
		}
		if err != nil {
			return nil, err
		}
		if execPath == "" {
			return nil, errors.New("Could not find python3 or virtualenv executable in PATH")
		}
	}
	cmdArgs = append(cmdArgs, "venvdir")
	var stderr bytes.Buffer
	pipVenv := exec.Command(execPath, cmdArgs...)
	pipVenv.Stderr = &stderr
	err = pipVenv.Run()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("pipenv install command failed: %s - %s", err.Error(), stderr.String()))
	}

	// Keep original value of 'PATH'.
	pathValue, exists := os.LookupEnv("PATH")
	if !exists {
		return nil, errors.New(fmt.Sprintf("couldn't find PATH variable."))
	}
	var newPathValue string
	var virtualEnvPath string
	if runtime.GOOS == "windows" {
		virtualEnvPath, err = filepath.Abs(filepath.Join("venvdir", "Scripts"))
		newPathValue = fmt.Sprintf("%s;", virtualEnvPath)
	} else {
		virtualEnvPath, err = filepath.Abs(filepath.Join("venvdir", "bin"))
		newPathValue = fmt.Sprintf("%s:", virtualEnvPath)
	}
	if err != nil {
		return nil, err
	}
	err = os.Setenv("PATH", newPathValue)
	if err != nil {
		return nil, err
	}
	return func() error {
		return os.Setenv("PATH", pathValue)
	}, nil
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
