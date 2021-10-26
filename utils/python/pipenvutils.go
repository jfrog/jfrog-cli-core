package python

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"os/exec"
	"strings"
)

func getPipenvEnv(venvDir string) string {
	if venvDir != "" {
		return fmt.Sprintf("WORKON_HOME='%s'", venvDir)
	}

	return ""
}

// Execute pipenv install command. "pipenv install"
func GetPipenvVenv(venvDir string) (string, error) {
	output, err := runPythonCommand("pipenv", []string{"--venv"}, getPipenvEnv(venvDir))
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(output), "\n"), err
}

// Executes the pipenv graph and returns a dependency map of all the installed pip packages in the current environment to and another list of the top level dependencies
func GetPipenvDependenciesList(venvDir string) (map[string]bool, error) {
	packages, err := runPipenvGraph(venvDir)
	if err != nil {
		return nil, err
	}

	// Parse the result.
	return parseDependenciesToList(packages)
}

func runPipenvInstall(venvDir string) error {
	_, err := runPythonCommand("pipenv", []string{"install"}, getPipenvEnv(venvDir))
	if err != nil {
		pythonPath, err := exec.LookPath("python3")
		if err != nil && pythonPath != "" {
			return err
		}

		_, err = runPythonCommand("pipenv", []string{"install", "--python", pythonPath}, getPipenvEnv(venvDir))
		if err != nil {
			return err
		}
	}
	return nil
}

// Executes the pipenv graph and returns a dependency map of all the installed pip packages in the current environment to and another list of the top level dependencies
func runPipenvGraph(venvDir string) ([]pythonDependencyPackage, error) {
	data, err := runPythonCommand("pipenv", []string{"graph", "--json"}, getPipenvEnv(venvDir))
	if err != nil {
		return nil, err
	}
	// Parse into array.
	packages := make([]pythonDependencyPackage, 0)
	if err := json.Unmarshal(data, &packages); err != nil {
		return nil, errorutils.CheckError(err)
	}
	// Parse the result.
	return packages, nil
}

// Executes the pipenv graph and returns a dependency map of all the installed pip packages in the current environment to and another list of the top level dependencies
func GetPipenvDependenciesGraph(venvDir string) (map[string][]string, []string, error) {
	err := runPipenvInstall(venvDir)
	if err != nil {
		return nil, nil, err
	}
	packages, err := runPipenvGraph(venvDir)
	if err != nil {
		return nil, nil, err
	}
	// Parse the result.
	return parseDependenciesToGraph(packages)
}

// Parse pip-dependency-map raw output to dependencies map (mapping dependency to his child deps) and top level deps list
func parseDependenciesToList(packages []pythonDependencyPackage) (map[string]bool, error) {
	// Create packages map.
	allPackages := map[string]bool{}
	for _, pkg := range packages {
		allPackages[pkg.Package.PackageName] = true
		for _, subPkg := range pkg.Dependencies {
			allPackages[subPkg.PackageName] = true
		}
	}

	return allPackages, nil
}
