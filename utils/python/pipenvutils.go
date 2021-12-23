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
func GetPipenvVenv() (string, error) {
	output, err := runPythonCommand("pipenv", []string{"--venv"}, "")
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(string(output), "\n"), err
}

// Get simple list of all dependencies (using pipenv graph)
func GetPipenvDependenciesList(venvDir string) (map[string]bool, error) {
	packages, err := runPipenvGraph(venvDir)
	if err != nil {
		return nil, err
	}
	return parseDependenciesToList(packages)
}

// Executes pipenv install and pipenv graph.
// Returns a dependency map of all the installed pip packages in the current environment to and another list of the top level dependencies
func GetPipenvDependenciesGraph(venvDir string) (map[string][]string, []string, error) {
	// Run pipenv install
	_, err := runPythonCommand("pipenv", []string{"install"}, getPipenvEnvironmentString(venvDir))
	if err != nil {
		return nil, nil, err
	}

	// Run pipenv graph
	packages, err := runPipenvGraph(venvDir)
	if err != nil {
		return nil, nil, err
	}
	return parseDependenciesToGraph(packages)
}

// Executes pipenv graph
// Returns a dependency map of all the installed pipenv packages and another list of the top level dependencies
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
	return packages, nil
}

// Parse pythonDependencyPackage list to dependencies map (mapping dependency to his child deps)
// also returns a list of top level dependencies
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
