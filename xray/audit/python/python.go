package python

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
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

func AuditPython(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails, pythonTool pythonutils.PythonTool) (results []services.ScanResponse, isMultipleRootProject bool, err error) {
	graph, err := BuildDependencyTree(pythonTool)
	if err != nil {
		return
	}
	isMultipleRootProject = len(graph) > 1
	results, err = audit.Scan(graph, xrayGraphScanPrams, serverDetails)
	return
}

func BuildDependencyTree(pythonTool pythonutils.PythonTool) ([]*services.GraphNode, error) {
	dependenciesGraph, rootDependenciesList, err := getDependencies(pythonTool)
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

func getDependencies(pythonTool pythonutils.PythonTool) (dependenciesGraph map[string][]string, rootDependencies []string, err error) {
	wd, err := os.Getwd()
	if err != nil {
		err = errorutils.CheckError(err)
		return
	}

	// Create temp dir to run all work outside users working directory
	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		return
	}

	err = os.Chdir(tempDirPath)
	if err != nil {
		err = errorutils.CheckError(err)
		return
	}

	defer func() {
		e := os.Chdir(wd)
		if err == nil {
			err = errorutils.CheckError(e)
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

	restoreEnv, err := runPythonInstall(tempDirPath, pythonTool)
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
	dependenciesGraph, rootDependencies, err = pythonutils.GetPythonDependencies(pythonTool, tempDirPath, localDependenciesPath)
	return
}

func runPythonInstall(tempDirPath string, pythonTool pythonutils.PythonTool) (restoreEnv func() error, err error) {
	switch pythonTool {
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
			return nil, errors.New("could not find python3 or virtualenv executable in PATH")
		}
	}
	cmdArgs = append(cmdArgs, "venvdir")
	var stderr bytes.Buffer
	pipVenv := exec.Command(execPath, cmdArgs...)
	pipVenv.Stderr = &stderr
	err = pipVenv.Run()
	if err != nil {
		return nil, fmt.Errorf("pipenv install command failed: %s - %s", err.Error(), stderr.String())
	}

	// Keep original value of 'PATH'.
	pathValue, exists := os.LookupEnv("PATH")
	if !exists {
		return nil, errors.New("couldn't find PATH variable")
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
