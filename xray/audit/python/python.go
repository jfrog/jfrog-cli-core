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
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
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

func BuildDependencyTree(auditPython *AuditPython) (dependencyTree []*xrayUtils.GraphNode, err error) {
	dependenciesGraph, directDependenciesList, err := getDependencies(auditPython)
	if err != nil {
		return
	}
	directDependencies := []*xrayUtils.GraphNode{}
	for _, rootDep := range directDependenciesList {
		directDependency := &xrayUtils.GraphNode{
			Id:    pythonPackageTypeIdentifier + rootDep,
			Nodes: []*xrayUtils.GraphNode{},
		}
		populatePythonDependencyTree(directDependency, dependenciesGraph)
		directDependencies = append(directDependencies, directDependency)
	}
	root := &xrayUtils.GraphNode{
		Id:    pythonPackageTypeIdentifier,
		Nodes: directDependencies,
	}
	return []*xrayUtils.GraphNode{root}, nil
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
	err = executeCommand("python", pipInstallArgs...)
	if err != nil && auditPython.PipRequirementsFile == "" {
		log.Debug(err.Error() + "\nTrying to install using a requirements file...")
		pipInstallArgs = getPipInstallArgs("requirements.txt")
		reqErr := executeCommand("python", pipInstallArgs...)
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

func getPipInstallArgs(requirementsFile string) []string {
	args := []string{"-m", "pip", "install"}
	if requirementsFile == "" {
		// Run 'pip install .'
		args = append(args, ".")
	} else {
		// Run pip 'install -r requirements <requirementsFile>'
		args = append(args, "-r", requirementsFile)
	}
	return args
}

func runPipInstallFromRemoteRegistry(server *config.ServerDetails, depsRepoName, pipRequirementsFile string) (err error) {
	rtUrl, err := utils.GetPypiRepoUrl(server, depsRepoName)
	if err != nil {
		return err
	}
	args := getPipInstallArgs(pipRequirementsFile)
	args = append(args, utils.GetPypiRemoteRegistryFlag(pythonutils.Pip), rtUrl.String())
	return executeCommand("python", args...)
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
	venvdirName := "venvdir"
	var cmdArgs []string
	pythonPath, windowsPyArg := pythonutils.GetPython3Executable()
	if windowsPyArg != "" {
		// Add '-3' arg for windows 'py -3' command
		cmdArgs = append(cmdArgs, windowsPyArg)
	}
	cmdArgs = append(cmdArgs, "-m", "venv", venvdirName)
	err = executeCommand(pythonPath, cmdArgs...)
	if err != nil {
		// Failed running 'python -m venv', trying to run 'virtualenv'
		log.Debug("Failed running python venv:", err.Error())
		err = executeCommand("virtualenv", "-p", pythonPath, venvdirName)
		if err != nil {
			return
		}
	}

	// Keep original value of 'PATH'.
	origPathValue := os.Getenv("PATH")
	venvPath, err := filepath.Abs(venvdirName)
	if err != nil {
		return
	}
	var venvBinPath string
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

func populatePythonDependencyTree(currNode *xrayUtils.GraphNode, dependenciesGraph map[string][]string) {
	if currNode.NodeHasLoop() {
		return
	}
	currDepChildren := dependenciesGraph[strings.TrimPrefix(currNode.Id, pythonPackageTypeIdentifier)]
	// Recursively create & append all node's dependencies.
	for _, dependency := range currDepChildren {
		childNode := &xrayUtils.GraphNode{
			Id:     pythonPackageTypeIdentifier + dependency,
			Nodes:  []*xrayUtils.GraphNode{},
			Parent: &xrayUtils.ParentNode{Id: currNode.Id, Parent: currNode.Parent},
		}
		currNode.Nodes = append(currNode.Nodes, childNode)
		populatePythonDependencyTree(childNode, dependenciesGraph)
	}
}
