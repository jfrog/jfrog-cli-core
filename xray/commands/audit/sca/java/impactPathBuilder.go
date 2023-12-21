package java

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca"
	"github.com/jfrog/jfrog-client-go/xray/services"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
)

// TODO add a test file + tests for the logics in this file

func BuildJavaImpactedPathsForScanResponse(scanResult []services.ScanResponse, dependenciesWithChildren []*xrayUtils.GraphNode) []services.ScanResponse {
	for _, result := range scanResult {
		/* TODO consider building impacted paths only once if possible

		if len(result.Vulnerabilities) > 0 || len(result.Violations) > 0 || len(result.Licenses) > 0 {
			buildVulnerabilitiesImpactedPaths()
		}
		*/
		childrenToParentsMap := getParentsMap(dependenciesWithChildren)

		if len(result.Vulnerabilities) > 0 {
			buildJavaVulnerabilitiesImpactPaths(result.Vulnerabilities)
		}
		/* TODO dont forget to complete these two
		if len(result.Violations) > 0 {
			buildViolationsImpactPaths(result.Violations, dependencyTree)
		}
		if len(result.Licenses) > 0 {
			buildLicensesImpactPaths(result.Licenses, dependencyTree)
		}

		*/
	}
	return scanResult
}

func getParentsMap(dependenciesWithChildren []*xrayUtils.GraphNode) map[string][]string {
	childrenToParentsMap := make(map[string][]string)
	for _, module := range dependenciesWithChildren {
		moduleName := module.Id
		childrenToParentsMap[moduleName] = nil
		moduleDependencies := module.Nodes
		for _, parentDependency := range moduleDependencies {
			parentName := parentDependency.Id
			for _, child := range parentDependency.Nodes {
				childName := child.Id
				childrenToParentsMap[childName] = append(childrenToParentsMap[parentName], parentName)
			}
			//TODO modules can have parents. should we add a special val to their parents list to notify they are modules? should we put in in a struct with indicator field if it is a module?
		}
	}
	return childrenToParentsMap
}

func buildJavaVulnerabilitiesImpactPaths(vulnerabilities []services.Vulnerability) {
	issuesMap := make(map[string][][]services.ImpactPathNode)
	for _, vulnerability := range vulnerabilities {
		sca.FillIssuesMapWithEmptyImpactPaths(issuesMap, vulnerability.Components)
	}
	//buildImpactPaths(issuesMap, dependencyTrees)
	for i := range vulnerabilities {
		sca.UpdateComponentsWithImpactPaths(vulnerabilities[i].Components, issuesMap)
	}
}

func buildVulnerabilitiesImpactedPaths(issuesMap map[string][][]services.ImpactPathNode) {
	panic("stop")
}
