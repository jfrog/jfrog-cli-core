package audit

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	xraycommands "github.com/jfrog/jfrog-cli-core/v2/xray/commands"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/slices"
)

func BuildXrayDependencyTree(treeHelper map[string][]string, nodeId string) *services.GraphNode {
	return buildXrayDependencyTree(treeHelper, []string{nodeId})
}

func buildXrayDependencyTree(treeHelper map[string][]string, impactPath []string) *services.GraphNode {
	nodeId := impactPath[len(impactPath)-1]
	// Initialize the new node
	xrDependencyTree := &services.GraphNode{}
	xrDependencyTree.Id = nodeId
	xrDependencyTree.Nodes = []*services.GraphNode{}
	if len(impactPath) >= buildinfo.RequestedByMaxLength {
		log.Debug("buildXrayDependencyTree exceeded max tree depth")
		return xrDependencyTree
	}
	// Recursively create & append all node's dependencies.
	for _, dependency := range treeHelper[nodeId] {
		// Prevent circular dependencies parsing
		if slices.Contains(impactPath, dependency) {
			continue
		}
		xrDependencyTree.Nodes = append(xrDependencyTree.Nodes, buildXrayDependencyTree(treeHelper, append(impactPath, dependency)))
	}
	return xrDependencyTree
}

func Audit(modulesDependencyTrees []*services.GraphNode, xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails, progress ioUtils.ProgressMgr, technology coreutils.Technology) (results []services.ScanResponse, err error) {
	if len(modulesDependencyTrees) == 0 {
		err = errorutils.CheckErrorf("No dependencies were found. Please try to build your project and re-run the audit command.")
		return
	}

	if progress != nil {
		progress.SetHeadlineMsg("Scanning for vulnerabilities")
	}

	// Get Xray version
	_, xrayVersion, err := xraycommands.CreateXrayServiceManagerAndGetVersion(serverDetails)
	if err != nil {
		return
	}
	err = coreutils.ValidateMinimumVersion(coreutils.Xray, xrayVersion, xraycommands.GraphScanMinXrayVersion)
	if err != nil {
		return
	}
	log.Info("JFrog Xray version is:", xrayVersion)
	for _, moduleDependencyTree := range modulesDependencyTrees {
		xrayGraphScanPrams.Graph = moduleDependencyTree
		log.Info("Scanning", len(xrayGraphScanPrams.Graph.Nodes), string(technology), "dependencies...")
		var scanResults *services.ScanResponse
		scanResults, err = xraycommands.RunScanGraphAndGetResults(serverDetails, xrayGraphScanPrams, xrayGraphScanPrams.IncludeVulnerabilities, xrayGraphScanPrams.IncludeLicenses, xrayVersion)
		if err != nil {
			err = errorutils.CheckErrorf("scanning failed with error: %s", err.Error())
			return
		}
		for i := range scanResults.Vulnerabilities {
			scanResults.Vulnerabilities[i].Technology = technology.ToString()
		}
		for i := range scanResults.Violations {
			scanResults.Violations[i].Technology = technology.ToString()
		}
		results = append(results, *scanResults)
	}
	return
}

func CreateTestWorkspace(t *testing.T, sourceDir string) (string, func()) {
	tempDirPath, createTempDirCallback := tests.CreateTempDirWithCallbackAndAssert(t)
	assert.NoError(t, fileutils.CopyDir(filepath.Join("..", "..", "commands", "testdata", sourceDir), tempDirPath, true, nil))
	wd, err := os.Getwd()
	assert.NoError(t, err, "Failed to get current dir")
	chdirCallback := testsutils.ChangeDirWithCallback(t, wd, tempDirPath)
	return tempDirPath, func() {
		chdirCallback()
		createTempDirCallback()
	}
}

func GetAndAssertNode(t *testing.T, modules []*services.GraphNode, moduleId string) *services.GraphNode {
	module := GetModule(modules, moduleId)
	assert.NotNil(t, module, "Module '"+moduleId+"' doesn't exist")
	return module
}

// Get a specific module from the provided modules list
func GetModule(modules []*services.GraphNode, moduleId string) *services.GraphNode {
	for _, module := range modules {
		splitIdentifier := strings.Split(module.Id, "//")
		id := splitIdentifier[0]
		if len(splitIdentifier) > 1 {
			id = splitIdentifier[1]
		}
		if id == moduleId {
			return module
		}
	}
	return nil
}

// Gets executable version and prints to the debug log if possible.
// Only supported for package managers that use "--version".
func GetExecutableVersion(executable string) (version string, err error) {
	verBytes, err := exec.Command(executable, "--version").CombinedOutput()
	if err != nil || len(verBytes) == 0 {
		return "", err
	}
	version = strings.TrimSpace(string(verBytes))
	log.Debug(fmt.Sprintf("Used %q version: %s", executable, version))
	return
}

// BuildImpactPathsForScanResponse builds the full impact paths for each vulnerability found in the scanResult argument, using the dependencyTrees argument.
// Returns the updated services.ScanResponse slice.
func BuildImpactPathsForScanResponse(scanResult []services.ScanResponse, dependencyTrees []*services.GraphNode) []services.ScanResponse {
	for _, result := range scanResult {
		if len(result.Vulnerabilities) > 0 {
			buildVulnerabilitiesImpactPaths(result.Vulnerabilities, dependencyTrees)
		}
		if len(result.Violations) > 0 {
			buildViolationsImpactPaths(result.Violations, dependencyTrees)
		}
		if len(result.Licenses) > 0 {
			buildLicensesImpactPaths(result.Licenses, dependencyTrees)
		}
	}
	return scanResult
}

// Initialize map of issues to their components with empty impact paths
func fillImpactPathsMapWithIssues(issuesImpactPathsMap map[string]*services.Component, components map[string]services.Component) {
	for dependencyName := range components {
		emptyPathsComponent := &services.Component{
			ImpactPaths:   [][]services.ImpactPathNode{},
			FixedVersions: components[dependencyName].FixedVersions,
			Cpes:          components[dependencyName].Cpes,
		}
		issuesImpactPathsMap[dependencyName] = emptyPathsComponent
	}
}

// Set the impact paths for each issue in the map
func buildImpactPaths(issuesImpactPathsMap map[string]*services.Component, dependencyTrees []*services.GraphNode) {
	for _, dependency := range dependencyTrees {
		setPathsForIssues(dependency, issuesImpactPathsMap, []services.ImpactPathNode{})
	}
}

func buildVulnerabilitiesImpactPaths(vulnerabilities []services.Vulnerability, dependencyTrees []*services.GraphNode) {
	issuesMap := make(map[string]*services.Component)
	for _, vulnerability := range vulnerabilities {
		fillImpactPathsMapWithIssues(issuesMap, vulnerability.Components)
	}
	buildImpactPaths(issuesMap, dependencyTrees)
	for i := range vulnerabilities {
		updateComponentsWithImpactPaths(vulnerabilities[i].Components, issuesMap)
	}
}

func buildViolationsImpactPaths(violations []services.Violation, dependencyTrees []*services.GraphNode) {
	issuesMap := make(map[string]*services.Component)
	for _, violation := range violations {
		fillImpactPathsMapWithIssues(issuesMap, violation.Components)
	}
	buildImpactPaths(issuesMap, dependencyTrees)
	for i := range violations {
		updateComponentsWithImpactPaths(violations[i].Components, issuesMap)
	}
}

func buildLicensesImpactPaths(licenses []services.License, dependencyTrees []*services.GraphNode) {
	issuesMap := make(map[string]*services.Component)
	for _, license := range licenses {
		fillImpactPathsMapWithIssues(issuesMap, license.Components)
	}
	buildImpactPaths(issuesMap, dependencyTrees)
	for i := range licenses {
		updateComponentsWithImpactPaths(licenses[i].Components, issuesMap)
	}
}

func updateComponentsWithImpactPaths(components map[string]services.Component, issuesMap map[string]*services.Component) {
	for dependencyName := range components {
		components[dependencyName] = *issuesMap[dependencyName]
	}
}

func setPathsForIssues(dependency *services.GraphNode, issuesImpactPathsMap map[string]*services.Component, pathFromRoot []services.ImpactPathNode) {
	pathFromRoot = append(pathFromRoot, services.ImpactPathNode{ComponentId: dependency.Id})
	if _, exists := issuesImpactPathsMap[dependency.Id]; exists {
		issuesImpactPathsMap[dependency.Id].ImpactPaths = append(issuesImpactPathsMap[dependency.Id].ImpactPaths, pathFromRoot)
	}
	for _, depChild := range dependency.Nodes {
		setPathsForIssues(depChild, issuesImpactPathsMap, pathFromRoot)
	}
}
