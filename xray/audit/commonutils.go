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
		// Log the scanned module ID
		moduleName := moduleDependencyTree.Id[strings.Index(moduleDependencyTree.Id, "//")+2:]
		log.Info("Scanning module " + moduleName + "...")
		var scanResults *services.ScanResponse
		scanResults, err = xraycommands.RunScanGraphAndGetResults(serverDetails, xrayGraphScanPrams, xrayGraphScanPrams.IncludeVulnerabilities, xrayGraphScanPrams.IncludeLicenses, xrayVersion)
		if err != nil {
			err = errorutils.CheckErrorf("Scanning %s failed with error: %s", moduleName, err.Error())
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

func BuildImpactPaths(scanResult []services.ScanResponse, dependencyTrees []*services.GraphNode) {
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
	return
}

func buildVulnerabilitiesImpactPaths(vulnerabilities []services.Vulnerability, dependencyTrees []*services.GraphNode) {
	vulnerabilitiesMap := setVulnerabilitiesPathsMap(vulnerabilities, dependencyTrees)
	for i := range vulnerabilities {
		for dependencyName := range vulnerabilities[i].Components {
			updateVulnerableComponent(vulnerabilities[i].Components, vulnerabilitiesMap[dependencyName], dependencyName)
		}
	}
}

func buildViolationsImpactPaths(violations []services.Violation, dependencyTrees []*services.GraphNode) {
	violationsMap := setVulnerabilitiesPathsMap(violations, dependencyTrees)
	for i := range violations {
		for dependencyName := range violations[i].Components {
			updateVulnerableComponent(violations[i].Components, violationsMap[dependencyName], dependencyName)
		}
	}
}

func buildLicensesImpactPaths(licenses []services.License, dependencyTrees []*services.GraphNode) {
	licensesMap := setVulnerabilitiesPathsMap(licenses, dependencyTrees)
	for i := range licenses {
		for dependencyName := range licenses[i].Components {
			updateVulnerableComponent(licenses[i].Components, licensesMap[dependencyName], dependencyName)
		}
	}
}

func setVulnerabilitiesPathsMap(issues interface{}, dependencyTrees []*services.GraphNode) map[string][][]services.ImpactPathNode {
	issueMap := make(map[string][][]services.ImpactPathNode)
	switch v := issues.(type) {
	case []services.Vulnerability:
		for _, vulnerability := range v {
			for dependencyName := range vulnerability.Components {
				issueMap[dependencyName] = [][]services.ImpactPathNode{}
			}
		}
	case []services.Violation:
		for _, violation := range v {
			for dependencyName := range violation.Components {
				issueMap[dependencyName] = [][]services.ImpactPathNode{}
			}
		}
	case []services.License:
		for _, license := range v {
			for dependencyName := range license.Components {
				issueMap[dependencyName] = [][]services.ImpactPathNode{}
			}
		}
	}

	for _, dependency := range dependencyTrees {
		setPathsForIssues(dependency, issueMap, []services.ImpactPathNode{})
	}
	return issueMap
}

func updateVulnerableComponent(components map[string]services.Component, impactPaths [][]services.ImpactPathNode, dependencyName string) {
	components[dependencyName] = services.Component{
		FixedVersions: components[dependencyName].FixedVersions,
		ImpactPaths:   impactPaths,
		Cpes:          components[dependencyName].Cpes,
	}
}

func setPathsForIssues(dependency *services.GraphNode, issuesMap map[string][][]services.ImpactPathNode, impactPath []services.ImpactPathNode) {
	impactPath = append(impactPath, services.ImpactPathNode{ComponentId: dependency.Id})
	if _, exists := issuesMap[dependency.Id]; exists {
		issuesMap[dependency.Id] = append(issuesMap[dependency.Id], impactPath)
	}
	for _, depChild := range dependency.Nodes {
		setPathsForIssues(depChild, issuesMap, impactPath)
	}
}
