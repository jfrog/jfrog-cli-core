package python

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

// Execute virtualenv command: "virtualenv {venvDirPath}" / "python3 -m venv {venvDirPath}"
func RunVirtualEnv() (err error) {
	var cmdArgs []string
	execPath, err := exec.LookPath("virtualenv")
	if err != nil || execPath == "" {
		// If virtualenv not installed try "venv"
		if coreutils.IsWindows() {
			// If the OS is Windows try using Py Launcher: "py -3 -m venv"
			execPath, err = exec.LookPath("py")
			cmdArgs = append(cmdArgs, "-3", "-m", "venv")
		} else {
			// If the OS is Linux try using python3 executable: "python3 -m venv"
			execPath, err = exec.LookPath("python3")
			cmdArgs = append(cmdArgs, "-m", "venv")
		}
		if err != nil {
			return errorutils.CheckError(err)
		}
		if execPath == "" {
			return errorutils.CheckErrorf("Could not find python3 or virtualenv executable in PATH")
		}
	}
	cmdArgs = append(cmdArgs, "venv")
	_, err = runPythonCommand(execPath, cmdArgs, "")
	return errorutils.CheckError(err)
}

// Getting the name of the directory inside venv dir that contains the bin files (different name in different OS's)
func venvBinDirByOS() string {
	if coreutils.IsWindows() {
		return "Scripts"
	}

	return "bin"
}

func GetVenvPythonExecPath() string {
	return filepath.Join("venv", venvBinDirByOS(), "python")
}

// Execute pip install command. "pip install ."
func RunPipInstall() (err error) {
	_, err = runPythonCommand(filepath.Join("venv", venvBinDirByOS(), "pip"), []string{"install", "."}, "")
	return err
}

// Execute pip install requirements command. "pip install -r requirements.txt"
func RunPipInstallRequirements() (err error) {
	_, err = runPythonCommand(filepath.Join("venv", venvBinDirByOS(), "pip"), []string{"install", "-r", "requirements.txt"}, "")
	return err
}

// Executes the pip-dependency-map script and returns a dependency map of all the installed pip packages in the current environment to and another list of the top level dependencies
func RunPipDepTree(pythonExecPath string) (map[string][]string, []string, error) {
	pipDependencyMapScriptPath, err := GetDepTreeScriptPath()
	if err != nil {
		return nil, nil, err
	}
	data, err := runPythonCommand(pythonExecPath, []string{pipDependencyMapScriptPath, "--json"}, "")
	if err != nil {
		return nil, nil, err
	}
	// Parse into array.
	packages := make([]pythonDependencyPackage, 0)
	if err = json.Unmarshal(data, &packages); err != nil {
		return nil, nil, errorutils.CheckError(err)
	}

	return parseDependenciesToGraph(packages)
}

// Return path to the dependency-tree script, if not exists it creates the file.
func GetDepTreeScriptPath() (string, error) {
	pipDependenciesPath, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return "", err
	}
	depTreeScriptName := "pipdeptree.py"
	pipDependenciesPath = filepath.Join(pipDependenciesPath, "pip", pipDepTreeVersion)
	depTreeScriptPath := filepath.Join(pipDependenciesPath, depTreeScriptName)
	err = writeScriptIfNeeded(pipDependenciesPath, depTreeScriptName)
	if err != nil {
		return "", err
	}
	return depTreeScriptPath, err
}

// Creates local python script on jfrog dependencies path folder if such not exists
func writeScriptIfNeeded(targetDirPath, scriptName string) error {
	scriptPath := filepath.Join(targetDirPath, scriptName)
	exists, err := fileutils.IsFileExists(scriptPath, false)
	if errorutils.CheckError(err) != nil {
		return err
	}
	if !exists {
		err = os.MkdirAll(targetDirPath, os.ModeDir|os.ModePerm)
		if errorutils.CheckError(err) != nil {
			return err
		}
		err = ioutil.WriteFile(scriptPath, pipDepTreeContent, os.ModePerm)
		if errorutils.CheckError(err) != nil {
			return err
		}
	}
	return nil
}
