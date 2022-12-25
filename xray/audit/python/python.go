package python

import (
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

func BuildDependencyTree(pythonTool pythonutils.PythonTool, requirementsFile string) (dependencyTree []*services.GraphNode, err error) {
	dependenciesGraph, directDependenciesList, err := getDependencies(pythonTool, requirementsFile)
	if err != nil {
		return
	}
	directDependencies := []*services.GraphNode{}
	for _, rootDep := range directDependenciesList {
		directDependency := &services.GraphNode{
			Id:    pythonPackageTypeIdentifier + rootDep,
			Nodes: []*services.GraphNode{},
		}
		populatePythonDependencyTree(directDependency, dependenciesGraph)
		directDependencies = append(directDependencies, directDependency)
	}
	root := &services.GraphNode{
		Id:    pythonPackageTypeIdentifier,
		Nodes: directDependencies,
	}
	return []*services.GraphNode{root}, nil
}

func getDependencies(pythonTool pythonutils.PythonTool, requirementsFile string) (dependenciesGraph map[string][]string, directDependencies []string, err error) {
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
	defer func() {
		e := restoreEnv()
		if err == nil {
			err = e
		}
	}()
	if err != nil {
		return
	}

	localDependenciesPath, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return
	}
	dependenciesGraph, directDependencies, err = pythonutils.GetPythonDependencies(pythonTool, tempDirPath, localDependenciesPath)
	if err != nil {
		if _, innerErr := audit.GetExecutableVersion("python"); innerErr != nil {
			log.Error(innerErr)
		}
		if _, innerErr := audit.GetExecutableVersion(string(pythonTool)); innerErr != nil {
			log.Error(innerErr)
		}
	}
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
		err = runPipInstall(requirementsFile)
		if err != nil && requirementsFile == "" {
			log.Debug(err.Error() + "\ntrying to install using a requirements file.")
			reqErr := runPipInstall("requirements.txt")
			if reqErr != nil {
				// Return Pip install error and log the requirements fallback error.
				log.Debug(reqErr.Error())
			} else {
				err = nil
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
	log.Debug(fmt.Sprintf("Running %q", strings.Join(installCmd.Args, " ")))
	output, err := installCmd.CombinedOutput()
	if err != nil {
		if _, innerErr := audit.GetExecutableVersion(executable); innerErr != nil {
			log.Error(innerErr)
		}
		return errorutils.CheckErrorf("%q command failed: %s - %s", strings.Join(installCmd.Args, " "), err.Error(), output)
	}
	return nil
}

func runPipInstall(requirementsFile string) error {
	// Try getting 'pip3' executable, if not found use 'pip'
	pipExec, _ := exec.LookPath("pip3")
	if pipExec == "" {
		pipExec = "pip"
	}
	if requirementsFile == "" {
		// Run 'pip install .'
		return executeCommand(pipExec, "install", ".")
	} else {
		// Run pip 'install -r requirements <requirementsFile>'
		return executeCommand(pipExec, "install", "-r", requirementsFile)
	}
}

// Execute virtualenv command: "virtualenv venvdir" / "python3 -m venv venvdir" and set path
func SetPipVirtualEnvPath() (restoreEnv func() error, err error) {
	restoreEnv = func() error {
		return nil
	}
	var cmdArgs []string
	pythonPath, windowsPyArg := pythonutils.GetPython3Executable()

	execPath, _ := exec.LookPath("virtualenv")
	if execPath != "" {
		cmdArgs = append(cmdArgs, "-p", pythonPath)
	} else {
		// If virtualenv not exists, try "python3 -m venv"
		execPath = pythonPath
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
	origPathValue := os.Getenv("PATH")
	venvPath, err := filepath.Abs("venvdir")
	if err != nil {
		return
	}
	venvBinPath := ""
	if runtime.GOOS == "windows" {
		venvBinPath = filepath.Join(venvPath, "Scripts")
	} else {
		venvBinPath = filepath.Join(venvPath, "bin")
	}
	err = os.Setenv("PATH", fmt.Sprintf("%s%c%s", venvBinPath, os.PathListSeparator, origPathValue))
	if err != nil {
		return
	}
	restoreEnv = func() error {
		return os.Setenv("PATH", origPathValue)
	}
	return
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
