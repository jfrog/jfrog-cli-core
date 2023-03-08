package python

import (
	"fmt"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	utils "github.com/jfrog/jfrog-cli-core/v2/utils/python"
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

type AuditPython struct {
	Server              *config.ServerDetails
	Tool                pythonutils.PythonTool
	RemotePypiRepo      string
	PipRequirementsFile string
}

func BuildDependencyTree(auditPython *AuditPython) (dependencyTree []*services.GraphNode, err error) {
	dependenciesGraph, directDependenciesList, err := getDependencies(auditPython)
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

func getDependencies(auditPython *AuditPython) (dependenciesGraph map[string][]string, directDependencies []string, err error) {
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

	restoreEnv, err := runPythonInstall(auditPython)
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
	dependenciesGraph, directDependencies, err = pythonutils.GetPythonDependencies(auditPython.Tool, tempDirPath, localDependenciesPath)
	if err != nil {
		if _, innerErr := audit.GetExecutableVersion("python"); innerErr != nil {
			log.Error(innerErr)
		}
		if _, innerErr := audit.GetExecutableVersion(string(auditPython.Tool)); innerErr != nil {
			log.Error(innerErr)
		}
	}
	return
}

func runPythonInstall(auditPython *AuditPython) (restoreEnv func() error, err error) {
	switch auditPython.Tool {
	case pythonutils.Pip:
		return installPipDeps(auditPython)
	case pythonutils.Pipenv:
		return installPipenvDeps(auditPython)
	case pythonutils.Poetry:
		return installPoetryDeps(auditPython)
	}
	return
}

func installPoetryDeps(auditPython *AuditPython) (restoreEnv func() error, err error) {
	restoreEnv = func() error {
		return nil
	}
	if auditPython.RemotePypiRepo != "" {
		rtUrl, username, password, err := utils.GetPypiRepoUrlWithCredentials(auditPython.Server, auditPython.RemotePypiRepo)
		if err != nil {
			return restoreEnv, err
		}
		if password != "" {
			err = utils.ConfigPoetryRepo(rtUrl.Scheme+"://"+rtUrl.Host+rtUrl.Path, username, password, auditPython.RemotePypiRepo)
			if err != nil {
				return restoreEnv, err
			}
		}
	}
	// Run 'poetry install'
	return restoreEnv, executeCommand("poetry", "install")
}

func installPipenvDeps(auditPython *AuditPython) (restoreEnv func() error, err error) {
	// Set virtualenv path to venv dir
	err = os.Setenv("WORKON_HOME", ".jfrog")
	if err != nil {
		return
	}
	restoreEnv = func() error {
		return os.Unsetenv("WORKON_HOME")
	}
	if auditPython.RemotePypiRepo != "" {
		return restoreEnv, runPipenvInstallFromRemoteRegistry(auditPython.Server, auditPython.RemotePypiRepo)
	}
	// Run 'pipenv install -d'
	return restoreEnv, executeCommand("pipenv", "install", "-d")
}

func installPipDeps(auditPython *AuditPython) (restoreEnv func() error, err error) {
	restoreEnv, err = SetPipVirtualEnvPath()
	if err != nil {
		return
	}
	if auditPython.RemotePypiRepo != "" {
		return restoreEnv, runPipInstallFromRemoteRegistry(auditPython.Server, auditPython.RemotePypiRepo, auditPython.PipRequirementsFile)
	}
	pipInstallArgs := getPipInstallArgs(auditPython.PipRequirementsFile)
	err = runPipInstall(pipInstallArgs...)
	if err != nil && auditPython.PipRequirementsFile == "" {
		log.Debug(err.Error() + "\ntrying to install using a requirements file.")
		reqErr := runPipInstall("requirements.txt")
		if reqErr != nil {
			// Return Pip install error and log the requirements fallback error.
			log.Debug(reqErr.Error())
		} else {
			err = nil
		}
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

func runPipInstall(args ...string) error {
	// Try getting 'pip3' executable, if not found use 'pip'
	pipExec := getPipExec()
	return executeCommand(pipExec, args...)
}

func getPipInstallArgs(requirementsFile string) []string {
	if requirementsFile == "" {
		// Run 'pip install .'
		return []string{"install", "."}
	}
	// Run pip 'install -r requirements <requirementsFile>'
	return []string{"install", "-r", requirementsFile}
}

func getPipExec() string {
	pipExec, _ := exec.LookPath("pip3")
	if pipExec == "" {
		pipExec = "pip"
	}
	return pipExec
}

func runPipInstallFromRemoteRegistry(server *config.ServerDetails, depsRepoName, pipRequirementsFile string) (err error) {
	rtUrl, err := utils.GetPypiRepoUrl(server, depsRepoName)
	if err != nil {
		return err
	}
	args := getPipInstallArgs(pipRequirementsFile)
	args = append(args, utils.GetPypiRemoteRegistryFlag(pythonutils.Pip), rtUrl.String())
	return runPipInstall(args...)
}

func runPipenvInstallFromRemoteRegistry(server *config.ServerDetails, depsRepoName string) (err error) {
	rtUrl, err := utils.GetPypiRepoUrl(server, depsRepoName)
	if err != nil {
		return err
	}
	args := []string{"install", "-d", utils.GetPypiRemoteRegistryFlag(pythonutils.Pipenv), rtUrl.String()}
	return executeCommand("pipenv", args...)
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
