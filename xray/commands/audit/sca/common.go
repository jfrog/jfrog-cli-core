package sca

import (
	"fmt"
	biutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	xraycommands "github.com/jfrog/jfrog-cli-core/v2/xray/commands/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
	"github.com/jfrog/jfrog-client-go/utils/log"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/jfrog/jfrog-client-go/xray/services"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const maxUniqueAppearances = 10

func BuildXrayDependencyTree(treeHelper map[string][]string, nodeId string) (*xrayUtils.GraphNode, []string) {
	rootNode := &xrayUtils.GraphNode{
		Id:    nodeId,
		Nodes: []*xrayUtils.GraphNode{},
	}
	dependencyAppearances := map[string]int8{}
	populateXrayDependencyTree(rootNode, treeHelper, &dependencyAppearances)
	return rootNode, maps.Keys(dependencyAppearances)
}

func populateXrayDependencyTree(currNode *xrayUtils.GraphNode, treeHelper map[string][]string, dependencyAppearances *map[string]int8) {
	(*dependencyAppearances)[currNode.Id]++
	// Recursively create & append all node's dependencies.
	for _, childDepId := range treeHelper[currNode.Id] {
		childNode := &xrayUtils.GraphNode{
			Id:     childDepId,
			Nodes:  []*xrayUtils.GraphNode{},
			Parent: currNode,
		}
		if (*dependencyAppearances)[childDepId] >= maxUniqueAppearances || childNode.NodeHasLoop() {
			continue
		}
		currNode.Nodes = append(currNode.Nodes, childNode)
		populateXrayDependencyTree(childNode, treeHelper, dependencyAppearances)
	}
}

func RunXrayDependenciesTreeScanGraph(dependencyTree *xrayUtils.GraphNode, progress ioUtils.ProgressMgr, technology coreutils.Technology, scanGraphParams *xraycommands.ScanGraphParams) (results []services.ScanResponse, err error) {
	scanGraphParams.XrayGraphScanParams().DependenciesGraph = dependencyTree
	scanMessage := fmt.Sprintf("Scanning %d %s dependencies", len(dependencyTree.Nodes), technology)
	if progress != nil {
		progress.SetHeadlineMsg(scanMessage)
	}
	log.Info(scanMessage + "...")
	var scanResults *services.ScanResponse
	scanResults, err = xraycommands.RunScanGraphAndGetResults(scanGraphParams)
	if err != nil {
		err = errorutils.CheckErrorf("scanning %s dependencies failed with error: %s", string(technology), err.Error())
		return
	}
	for i := range scanResults.Vulnerabilities {
		scanResults.Vulnerabilities[i].Technology = technology.ToString()
	}
	for i := range scanResults.Violations {
		scanResults.Violations[i].Technology = technology.ToString()
	}
	results = append(results, *scanResults)
	return
}

func CreateTestWorkspace(t *testing.T, sourceDir string) (string, func()) {
	tempDirPath, createTempDirCallback := tests.CreateTempDirWithCallbackAndAssert(t)
	assert.NoError(t, biutils.CopyDir(filepath.Join("..", "..", "..", "testdata", sourceDir), tempDirPath, true, nil))
	wd, err := os.Getwd()
	assert.NoError(t, err, "Failed to get current dir")
	chdirCallback := testsutils.ChangeDirWithCallback(t, wd, tempDirPath)
	return tempDirPath, func() {
		chdirCallback()
		createTempDirCallback()
	}
}

func GetAndAssertNode(t *testing.T, modules []*xrayUtils.GraphNode, moduleId string) *xrayUtils.GraphNode {
	module := GetModule(modules, moduleId)
	assert.NotNil(t, module, "Module '"+moduleId+"' doesn't exist")
	return module
}

// GetModule gets a specific module from the provided modules list
func GetModule(modules []*xrayUtils.GraphNode, moduleId string) *xrayUtils.GraphNode {
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

// GetExecutableVersion gets an executable version and prints to the debug log if possible.
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
func BuildImpactPathsForScanResponse(scanResult []services.ScanResponse, dependencyTree []*xrayUtils.GraphNode) []services.ScanResponse {
	for _, result := range scanResult {
		if len(result.Vulnerabilities) > 0 {
			buildVulnerabilitiesImpactPaths(result.Vulnerabilities, dependencyTree)
		}
		if len(result.Violations) > 0 {
			buildViolationsImpactPaths(result.Violations, dependencyTree)
		}
		if len(result.Licenses) > 0 {
			buildLicensesImpactPaths(result.Licenses, dependencyTree)
		}
	}
	return scanResult
}

// Initialize a map of issues to their components with empty impact paths
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
func buildImpactPaths(issuesImpactPathsMap map[string]*services.Component, dependencyTrees []*xrayUtils.GraphNode) {
	for _, dependency := range dependencyTrees {
		setPathsForIssues(dependency, issuesImpactPathsMap, []services.ImpactPathNode{})
	}
}

func buildVulnerabilitiesImpactPaths(vulnerabilities []services.Vulnerability, dependencyTrees []*xrayUtils.GraphNode) {
	issuesMap := make(map[string]*services.Component)
	for _, vulnerability := range vulnerabilities {
		fillImpactPathsMapWithIssues(issuesMap, vulnerability.Components)
	}
	buildImpactPaths(issuesMap, dependencyTrees)
	for i := range vulnerabilities {
		updateComponentsWithImpactPaths(vulnerabilities[i].Components, issuesMap)
	}
}

func buildViolationsImpactPaths(violations []services.Violation, dependencyTrees []*xrayUtils.GraphNode) {
	issuesMap := make(map[string]*services.Component)
	for _, violation := range violations {
		fillImpactPathsMapWithIssues(issuesMap, violation.Components)
	}
	buildImpactPaths(issuesMap, dependencyTrees)
	for i := range violations {
		updateComponentsWithImpactPaths(violations[i].Components, issuesMap)
	}
}

func buildLicensesImpactPaths(licenses []services.License, dependencyTrees []*xrayUtils.GraphNode) {
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

func setPathsForIssues(dependency *xrayUtils.GraphNode, issuesImpactPathsMap map[string]*services.Component, pathFromRoot []services.ImpactPathNode) {
	pathFromRoot = append(pathFromRoot, services.ImpactPathNode{ComponentId: dependency.Id})
	if _, exists := issuesImpactPathsMap[dependency.Id]; exists {
		issuesImpactPathsMap[dependency.Id].ImpactPaths = append(issuesImpactPathsMap[dependency.Id].ImpactPaths, pathFromRoot)
	}
	for _, depChild := range dependency.Nodes {
		setPathsForIssues(depChild, issuesImpactPathsMap, pathFromRoot)
	}
}
