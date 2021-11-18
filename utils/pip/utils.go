package piputils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

func runPythonCommand(execPath string, cmdArgs []string) (data []byte, err error) {
	cmd := exec.Command(execPath, cmdArgs...)
	log.Debug(fmt.Sprintf("running command: %v", cmd.Args))
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err = errorutils.CheckError(cmd.Run())

	if err != nil {
		return nil, err
	}
	return stdout.Bytes(), err
}

// Execute virtualenv command: "virtualenv {venvDirPath}" / "python3 -m venv {venvDirPath}"
func RunVirtualEnv(venvDirPath string) (err error) {
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
	cmdArgs = append(cmdArgs, venvDirPath)
	_, err = runPythonCommand(execPath, cmdArgs)
	return errorutils.CheckError(err)
}

// Getting the name of the directory inside venv dir that contains the bin files (different name in different OS's)
func venvBinDirByOS() string {
	if coreutils.IsWindows() {
		return "Scripts"
	}

	return "bin"
}

// Execute pip install command. "pip install ."
func RunPipInstall(venvDirPath string) (err error) {
	_, err = runPythonCommand(filepath.Join(venvDirPath, venvBinDirByOS(), "pip"), []string{"install", "."})
	return err
}

// Executes the pip-dependency-map script and returns a dependency map of all the installed pip packages in the current environment to and another list of the top level dependencies
func RunPipDepTree(venvDirPath string) (map[string][]string, []string, error) {
	pipDependencyMapScriptPath, err := GetDepTreeScriptPath()
	if err != nil {
		return nil, nil, err
	}
	data, err := runPythonCommand(filepath.Join(venvDirPath, venvBinDirByOS(), "python"), []string{pipDependencyMapScriptPath, "--json"})
	if err != nil {
		return nil, nil, err
	}
	// Parse the result.
	return parsePipDependencyMapOutput(data)
}

// Parse pip-dependency-map raw output to dependencies map (mapping dependency to his child deps) and top level deps list
func parsePipDependencyMapOutput(data []byte) (map[string][]string, []string, error) {
	// Parse into array.
	packages := make([]pipDependencyPackage, 0)
	if err := json.Unmarshal(data, &packages); err != nil {
		return nil, nil, errorutils.CheckError(err)
	}

	// Create packages map.
	packagesMap := map[string][]string{}
	allSubPackages := map[string]bool{}
	for _, pkg := range packages {
		var subPackages []string
		for _, subPkg := range pkg.Dependencies {
			subPkgFullName := subPkg.Key + ":" + subPkg.InstalledVersion
			subPackages = append(subPackages, subPkgFullName)
			allSubPackages[subPkgFullName] = true
		}
		packagesMap[pkg.Package.Key+":"+pkg.Package.InstalledVersion] = subPackages
	}

	var topLevelPackagesList []string
	for pkgName := range packagesMap {
		if allSubPackages[pkgName] == false {
			topLevelPackagesList = append(topLevelPackagesList, pkgName)
		}
	}
	return packagesMap, topLevelPackagesList, nil
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

// Structs for parsing the pip-dependency-map result.
type pipDependencyPackage struct {
	Package      packageType  `json:"package,omitempty"`
	Dependencies []dependency `json:"dependencies,omitempty"`
}

type packageType struct {
	Key              string `json:"key,omitempty"`
	PackageName      string `json:"package_name,omitempty"`
	InstalledVersion string `json:"installed_version,omitempty"`
}

type dependency struct {
	Key              string `json:"key,omitempty"`
	PackageName      string `json:"package_name,omitempty"`
	InstalledVersion string `json:"installed_version,omitempty"`
}
