package piputils

import (
	"encoding/json"
	"errors"
	"fmt"
	gofrogcmd "github.com/jfrog/gofrog/io"
	piputils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/pip"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io"
	"io/ioutil"
	"os/exec"
	"path"
	"path/filepath"
)

type Cmd struct {
	Go           string
	Command      []string
	CommandFlags []string
	Dir          string
	StrWriter    io.WriteCloser
	ErrWriter    io.WriteCloser
}

func (config *Cmd) GetCmd() (cmd *exec.Cmd) {
	var cmdStr []string
	cmdStr = append(cmdStr, config.Go)
	cmdStr = append(cmdStr, config.Command...)
	cmdStr = append(cmdStr, config.CommandFlags...)
	cmd = exec.Command(cmdStr[0], cmdStr[1:]...)
	cmd.Dir = config.Dir
	return
}

func (config *Cmd) GetEnv() map[string]string {
	return map[string]string{}
}

func (config *Cmd) GetStdWriter() io.WriteCloser {
	return config.StrWriter
}

func (config *Cmd) GetErrWriter() io.WriteCloser {
	return config.ErrWriter
}

// Get executable path.
// If run inside a virtual-env, this should return the path for the correct executable.
func GetExecutablePath(executableName string) (string, error) {
	executablePath, err := exec.LookPath(executableName)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	if executablePath == "" {
		return "", errorutils.CheckError(errors.New(fmt.Sprintf("Could not find '%s' executable", executableName)))
	}

	log.Debug(fmt.Sprintf("Found %s executable at: %s", executableName, executablePath))
	return executablePath, nil
}

// Execute pip-dependency-map script, return dependency map of all installed pip packages in current environment.
// pythonExecPath - Execution path python.
func runPythonCommand(execPath, command string, cmdArgs []string) (data []byte, err error) {
	pipeReader, pipeWriter := io.Pipe()
	defer func() {
		e := pipeReader.Close()
		if err == nil {
			err = e
		}
	}()
	log.Debug(fmt.Sprintf("Running python command: %s %s %v", execPath, command, cmdArgs))
	// Execute the python pip-dependency-map script.
	pipDependencyMapCmd := &piputils.PipCmd{
		Executable:  execPath,
		Command:     command,
		CommandArgs: cmdArgs,
		StrWriter:   pipeWriter,
	}
	var pythonErr error
	go func() {
		pythonErr = gofrogcmd.RunCmd(pipDependencyMapCmd)
	}()
	data, err = ioutil.ReadAll(pipeReader)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	if pythonErr != nil {
		return nil, errorutils.CheckError(pythonErr)
	}
	return data, err
}

// Execute virtualenv command. "virtualenv .jfrog"
func RunVirtualEnv(venvDirPath string) (err error) {
	execPath, err := GetExecutablePath("virtualenv")
	_, err = runPythonCommand(execPath, venvDirPath, []string{})
	return err
}

// Execute pip install command. "pip install ."
func RunPipInstall(venvDirPath string) (err error) {
	_, err = runPythonCommand(filepath.Join(venvDirPath, "bin", "pip"), "install", []string{"."})
	return err
}

// Execute pip-dependency-map script, return dependency map of all installed pip packages in current environment.
func RunPipDepTree(venvDirPath string) (map[string][]string, []string, error) {
	pipDependencyMapScriptPath, err := GetDepTreeScriptPath()
	if err != nil {
		return nil, nil, err
	}
	data, err := runPythonCommand(filepath.Join(venvDirPath, "bin", "python"), pipDependencyMapScriptPath, []string{"--json"})

	// Parse the result.
	return parsePipDependencyMapOutput(data)
}

// Parse pip-dependency-map raw output to dependencies map.
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
	pipDependenciesPath = filepath.Join(pipDependenciesPath, "pip", "2")
	depTreeScriptPath := path.Join(pipDependenciesPath, "pipdeptree.py")

	return depTreeScriptPath, err
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
