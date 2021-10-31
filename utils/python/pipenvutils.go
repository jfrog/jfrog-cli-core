package python

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"strings"
)

func getPipenvEnvironmentString(venvDir string) string {
	if venvDir != "" {
		return fmt.Sprintf("WORKON_HOME=%s", venvDir)
	}
	return ""
}

// Execute "pipenv --venv" to get the pipenv virtual env path
func GetPipenvVenv(venvDir string) (string, error) {
	output, err := runPythonCommand("pipenv", []string{"--venv"}, getPipenvEnvironmentString(venvDir))
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

// Executes the pipenv install and pipenv graph
// Returns a dependency map of all the installed pip packages in the current environment to and another list of the top level dependencies
func GetPipenvDependenciesGraph(venvDir string) (map[string][]string, []string, error) {
	// run pipenv install
	_, err := runPythonCommand("pipenv", []string{"install"}, getPipenvEnvironmentString(venvDir))
	if err != nil {
		return nil, nil, err
	}

	// run pipenv graph
	packages, err := runPipenvGraph(venvDir)
	if err != nil {
		return nil, nil, err
	}

	// Parse the result.
	return parseDependenciesToGraph(packages)
}

// Executes the pipenv graph and returns a dependency map of all the installed pip packages in the current environment to and another list of the top level dependencies
func runPipenvGraph(venvDir string) ([]pythonDependencyPackage, error) {
	data, err := runPythonCommand("pipenv", []string{"graph", "--json"}, getPipenvEnvironmentString(venvDir))
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
