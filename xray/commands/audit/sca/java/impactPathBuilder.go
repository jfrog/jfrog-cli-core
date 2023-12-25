package java

import (
	"fmt"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca"
	"github.com/jfrog/jfrog-client-go/xray/services"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"golang.org/x/exp/slices"
)

const (
	impactPathLimit         = 20
	directDependencyPathLen = 2
)

// TODO add a test file + tests for the logics in this file

func BuildJavaImpactedPathsForScanResponse(scanResult []services.ScanResponse, dependenciesWithChildren []*xrayUtils.GraphNode) []services.ScanResponse {
	for _, result := range scanResult {
		childrenToParentsMap := getParentsMap(dependenciesWithChildren)
		projectRoots := getRootsWithParentsStatus(dependenciesWithChildren, childrenToParentsMap)

		if len(result.Vulnerabilities) > 0 {
			buildJavaVulnerabilitiesImpactPaths(result.Vulnerabilities, childrenToParentsMap, projectRoots, &dependenciesWithChildren)
		}
		if len(result.Violations) > 0 {
			buildJavaViolationsImpactPaths(result.Violations, childrenToParentsMap)
		}
		if len(result.Licenses) > 0 {
			buildJavaLicensesImpactPaths(result.Licenses, childrenToParentsMap)
		}
	}
	return scanResult
}

// Returns a map from a dependency to a slice with all its parents in the project (modules or other packages)
func getParentsMap(dependenciesWithChildren []*xrayUtils.GraphNode) map[string][]string {
	childrenToParentsMapSets := make(map[string]*datastructures.Set[string])
	for _, module := range dependenciesWithChildren {
		// Every entry (module) in dependenciesWithChildren has itself with its direct dependencies as one of its Nodes, therefore direct dependencies to the module itself will be added as well
		moduleDependencies := module.Nodes
		for _, parentDependency := range moduleDependencies {
			parentName := parentDependency.Id
			for _, child := range parentDependency.Nodes {
				childName := child.Id
				if _, exists := childrenToParentsMapSets[childName]; !exists {
					childrenToParentsMapSets[childName] = datastructures.MakeSet[string]()
				}
				childrenToParentsMapSets[childName].Add(parentName)
			}
		}
	}

	childrenToParentsMap := make(map[string][]string)
	for childName, parentsSet := range childrenToParentsMapSets {
		childrenToParentsMap[childName] = parentsSet.ToSlice()
	}
	return childrenToParentsMap
}

// TODO delete if not used
// Returns a map of the project's modules with a status weather the module has parents or not
func getRootsWithParentsStatus(dependenciesWithChildren []*xrayUtils.GraphNode, childrenToParentsMap map[string][]string) map[string]bool {
	rootsWithParentsExistenceStatus := make(map[string]bool)
	for _, module := range dependenciesWithChildren {
		moduleName := module.Id
		if _, exists := childrenToParentsMap[moduleName]; exists {
			rootsWithParentsExistenceStatus[moduleName] = true
		} else {
			rootsWithParentsExistenceStatus[moduleName] = false
		}
	}
	return rootsWithParentsExistenceStatus
}

func buildJavaVulnerabilitiesImpactPaths(vulnerabilities []services.Vulnerability, childrenToParentsMap map[string][]string, projectRoots map[string]bool, dependenciesWithChildren *[]*xrayUtils.GraphNode) {
	issuesMap := make(map[string][][]services.ImpactPathNode)
	for _, vulnerability := range vulnerabilities {
		sca.FillIssuesMapWithEmptyImpactPaths(issuesMap, vulnerability.Components)
	}
	buildJavaImpactedPaths(issuesMap, childrenToParentsMap, projectRoots)
	detectDirectImpactPathsIfExists(issuesMap, childrenToParentsMap, projectRoots, dependenciesWithChildren)
	for i := range vulnerabilities {
		sca.UpdateComponentsWithImpactPaths(vulnerabilities[i].Components, issuesMap)
	}
}

func buildJavaViolationsImpactPaths(violations []services.Violation, childrenToParentsMap map[string][]string) {
	issuesMap := make(map[string][][]services.ImpactPathNode)
	for _, violation := range violations {
		sca.FillIssuesMapWithEmptyImpactPaths(issuesMap, violation.Components)
	}
	//buildJavaImpactedPaths(issuesMap, childrenToParentsMap)
	for i := range violations {
		sca.UpdateComponentsWithImpactPaths(violations[i].Components, issuesMap)
	}
}

func buildJavaLicensesImpactPaths(licenses []services.License, childrenToParentsMap map[string][]string) {
	issuesMap := make(map[string][][]services.ImpactPathNode)
	for _, license := range licenses {
		sca.FillIssuesMapWithEmptyImpactPaths(issuesMap, license.Components)
	}
	//buildJavaImpactedPaths(issuesMap, childrenToParentsMap)
	for i := range licenses {
		sca.UpdateComponentsWithImpactPaths(licenses[i].Components, issuesMap)
	}
}

// Builds impacted paths to all dependencies in issuesMap. Each entry holds maximum of 20 impacted paths.
func buildJavaImpactedPaths(issuesMap map[string][][]services.ImpactPathNode, childrenToParentsMap map[string][]string, projectRoots map[string]bool) {
	for packageId := range issuesMap {
		pathStart := services.ImpactPathNode{ComponentId: packageId}
		var curPath []services.ImpactPathNode
		curPath = append(curPath, pathStart)
		setPathsForIssue(packageId, packageId, issuesMap, &childrenToParentsMap, &projectRoots, curPath) //[]services.ImpactPathNode{})
	}
}

func setPathsForIssue(leafPackageId string, curPackageId string, issuesMap map[string][][]services.ImpactPathNode, childrenToParentsMap *map[string][]string, projectRoots *map[string]bool, pathFromDependency []services.ImpactPathNode) {
	if len(issuesMap[leafPackageId]) >= impactPathLimit {
		return
	}

	if _, exists := (*projectRoots)[curPackageId]; exists {
		// When we arrive at a root, we add the path to the path's list of leafPackageId
		pathCopy := make([]services.ImpactPathNode, len(pathFromDependency))
		copy(pathCopy, pathFromDependency)
		slices.Reverse(pathCopy)
		if len(pathCopy) == directDependencyPathLen {
			// We want to add direct dependencies at the beginning for the detection of direct impact paths later
			slices.Reverse(issuesMap[leafPackageId])
			issuesMap[leafPackageId] = append(issuesMap[leafPackageId], pathCopy)
			slices.Reverse(issuesMap[leafPackageId])
		} else {
			issuesMap[leafPackageId] = append(issuesMap[leafPackageId], pathCopy)
		}

		// TODO do we need to continue building the path if we got to a root but there is a module above it? if so- use the value about the parents from projectRoots map. if not- return only a set of modules and not a map
		// If current root has parents we continue building the path up to the top root as well
	} else {
		curPackageParents := (*childrenToParentsMap)[curPackageId]
		for _, parent := range curPackageParents {
			// Check for a cycle in the path
			if pathAlreadyContainsPackage(pathFromDependency, parent) {
				continue
			}
			pathWithCurParent := make([]services.ImpactPathNode, len(pathFromDependency))
			copy(pathWithCurParent, pathFromDependency)
			pathWithCurParent = append(pathWithCurParent, services.ImpactPathNode{ComponentId: parent})
			setPathsForIssue(leafPackageId, parent, issuesMap, childrenToParentsMap, projectRoots, pathWithCurParent)
		}
	}
}

func pathAlreadyContainsPackage(pathFromDependency []services.ImpactPathNode, parentName string) bool {
	for _, pathNode := range pathFromDependency {
		if pathNode.ComponentId == parentName {
			return true
		}
	}
	return false
}

func detectDirectImpactPathsIfExists(issuesMap map[string][][]services.ImpactPathNode, childrenToParentsMap map[string][]string, projectRoots map[string]bool, dependenciesWithChildren *[]*xrayUtils.GraphNode) {
	for _, module := range *dependenciesWithChildren {
		fmt.Println(module.Id) //TODO del
		moduleDependencies := module.Nodes

		// Get self dependency
		var selfDependency *xrayUtils.GraphNode
		for _, dependency := range moduleDependencies {
			// Every module at top level in dependenciesWithChildren has itself with its direct dependencies as one of its Nodes
			if dependency.Id == module.Id {
				selfDependency = dependency
				break
			}
		}

		// TODO cehck Michael solution before continuing!!!! limit the tree build to 10 times (of visiting the same dep)
		for _, directDependency := range selfDependency.Nodes {
			fmt.Println(directDependency)

		}
	}
}
