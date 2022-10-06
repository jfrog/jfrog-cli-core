package python

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	clientLog "github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	pythonPackageTypeIdentifier = "pypi://"
)

func AuditPython(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails, pythonTool pythonutils.PythonTool, progress ioUtils.ProgressMgr, requirementsFile string) (results []services.ScanResponse, isMultipleRootProject bool, err error) {
	graph, err := BuildDependencyTree(pythonTool, requirementsFile)
	if err != nil {
		return
	}
	isMultipleRootProject = len(graph) > 1
	results, err = audit.Scan(graph, xrayGraphScanPrams, serverDetails, progress, coreutils.Technology(pythonTool))
	return
}

func BuildDependencyTree(pythonTool pythonutils.PythonTool, requirementsFile string) ([]*services.GraphNode, error) {
	dependenciesGraph, rootDependenciesList, err := getDependencies(pythonTool, requirementsFile)
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

func getDependencies(pythonTool pythonutils.PythonTool, requirementsFile string) (dependenciesGraph map[string][]string, rootDependencies []string, err error) {
	wd, err := os.Getwd()
	if errorutils.CheckError(err) != nil {
		return
	}

	// Create temp dir to run all work outside users working directory
	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		return
	}

	err = os.Chdir(tempDirPath)
	if errorutils.CheckError(err) != nil {
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

	restoreEnv, err := runPythonInstall(pythonTool, requirementsFile)
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

func runPythonInstall(pythonTool pythonutils.PythonTool, requirementsFile string) (restoreEnv func() error, err error) {
	restoreEnv = func() error {
		return nil
	}
	switch pythonTool {
	case pythonutils.Pip:
		restoreEnv, err = SetPipVirtualEnvPath()
		if err != nil {
			return
		}
		// Try getting 'pip3' executable, if not found use 'pip'
		pipExec, _ := exec.LookPath("pip3")
		if pipExec == "" {
			pipExec = "pip"
		}
		var pipInstallErr error
		if requirementsFile == "" {
			// Run pip install
			pipInstallErr = executeCommand(pipExec, "install", ".")
			if pipInstallErr != nil {
				clientLog.Debug(err.Error() + "\ntrying to install using a requirements file.")
				requirementsFile = "requirements.txt"
			}
		}
		// If running pip install failed or requirementsFile is assigned, run pip install -r
		if requirementsFile != "" {
			err = requirementsFileExists(requirementsFile)
			if err == nil {
				err = executeCommand(pipExec, "install", "-r", requirementsFile)
			}
			if pipInstallErr != nil {
				// Return Pip install error and log the requirements fallback error.
				clientLog.Debug(err.Error())
				err = pipInstallErr
			}
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
		// Run 'pipenv install -d'
		err = executeCommand("pipenv", "install", "-d")

	case pythonutils.Poetry:
		// Run 'poetry install'
		err = executeCommand("poetry", "install")
	}
	return
}

func executeCommand(executable string, args ...string) error {
	installCmd := exec.Command(executable, args...)
	clientLog.Debug(fmt.Sprintf("Running %q", strings.Join(installCmd.Args, " ")))
	output, err := installCmd.CombinedOutput()
	if err != nil {
		audit.LogExecutableVersion(executable)
		return errorutils.CheckErrorf("%q command failed: %s - %s", strings.Join(installCmd.Args, " "), err.Error(), output)
	}
	return nil
}

func requirementsFileExists(requirementsFile string) (err error) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	exists, err := fileutils.IsFileExists(filepath.Join(wd, requirementsFile), false)
	if err != nil || !exists {
		errString := fmt.Sprintf("requirements file: %q couldn't be found in the root directory", requirementsFile)
		if err != nil {
			errString = errString + " - " + err.Error()
		}
		err = errors.New(errString)
	}

	return
}

// Execute virtualenv command: "virtualenv venvdir" / "python3 -m venv venvdir" and set path
func SetPipVirtualEnvPath() (restoreEnv func() error, err error) {
	restoreEnv = func() error {
		return nil
	}
	var cmdArgs []string
	execPath, _ := exec.LookPath("virtualenv")
	if execPath == "" {
		// If virtualenv not exists, try "python3 -m venv"
		windowsPyArg := ""
		execPath, windowsPyArg = pythonutils.GetPython3Executable()
		if windowsPyArg != "" {
			// Add '-3' arg for windows 'py -3' command
			cmdArgs = append(cmdArgs, windowsPyArg)
		}
		cmdArgs = append(cmdArgs, "-m", "venv")
	}

	cmdArgs = append(cmdArgs, "venvdir")
	err = executeCommand(execPath, cmdArgs...)
	if err != nil {
		return
	}

	// Keep original value of 'PATH'.
	origPathEnv := os.Getenv("PATH")
	var newPathEnv string
	var virtualEnvPath string
	if runtime.GOOS == "windows" {
		virtualEnvPath, err = filepath.Abs(filepath.Join("venvdir", "Scripts"))
		newPathEnv = virtualEnvPath + ";" + origPathEnv
	} else {
		virtualEnvPath, err = filepath.Abs(filepath.Join("venvdir", "bin"))
		newPathEnv = virtualEnvPath + ":" + origPathEnv
	}
	if err != nil {
		return
	}
	err = os.Setenv("PATH", newPathEnv)
	if err != nil {
		return
	}
	return func() error {
		return os.Setenv("PATH", origPathEnv)
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
