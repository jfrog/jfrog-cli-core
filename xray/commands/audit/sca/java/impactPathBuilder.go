package java

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

// TODO add a test file + tests for the logics in this file

func BuildJavaImpactedPathsForScanResponse(scanResult []services.ScanResponse) []services.ScanResponse {
	for _, result := range scanResult {
		/* TODO consider building impacted paths only once if possible
		if len(result.Vulnerabilities) > 0 || len(result.Violations) > 0 || len(result.Licenses) > 0 {
			buildVulnerabilitiesImpactedPaths()
		}
		*/

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
