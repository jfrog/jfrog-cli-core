package scangraph

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	GraphScanMinXrayVersion = "3.29.0"
	ScanTypeMinXrayVersion  = "3.37.2"
)

func RunScanGraphAndGetResults(params *ScanGraphParams) (*services.ScanResponse, error) {
	xrayManager, err := utils.CreateXrayServiceManager(params.serverDetails)
	if err != nil {
		return nil, err
	}

	err = clientutils.ValidateMinimumVersion(clientutils.Xray, params.xrayVersion, ScanTypeMinXrayVersion)
	if err != nil {
		// Remove scan type param if Xray version is under the minimum supported version
		params.xrayGraphScanParams.ScanType = ""
	}

	if params.xrayGraphScanParams.XscGitInfoContext != nil {
		if params.xrayGraphScanParams.XscVersion, err = xrayManager.XscEnabled(); err != nil {
			return nil, err
		}
	}

	scanId, err := xrayManager.ScanGraph(*params.xrayGraphScanParams)
	if err != nil {
		return nil, err
	}

	xscEnabled := params.xrayGraphScanParams.XscVersion != ""
	scanResult, err := xrayManager.GetScanGraphResults(scanId, params.XrayGraphScanParams().IncludeVulnerabilities, params.XrayGraphScanParams().IncludeLicenses, xscEnabled)
	if err != nil {
		return nil, err
	}
	return filterResultIfNeeded(scanResult, params), nil
}

func filterResultIfNeeded(scanResult *services.ScanResponse, params *ScanGraphParams) *services.ScanResponse {
	if !shouldFilterResults(params) {
		return scanResult
	}

	scanResult.Violations = filterViolations(scanResult.Violations, params)
	scanResult.Vulnerabilities = filterVulnerabilities(scanResult.Vulnerabilities, params)
	return scanResult
}

func shouldFilterResults(params *ScanGraphParams) bool {
	return params.severityLevel > 0 || params.fixableOnly
}

func filterViolations(violations []services.Violation, params *ScanGraphParams) []services.Violation {
	var filteredViolations []services.Violation
	for _, violation := range violations {
		if params.fixableOnly {
			violation.Components = getFixableComponents(violation.Components)
			if len(violation.Components) == 0 {
				// All the components were filtered, filter this violation
				continue
			}
		}
		if getLevelOfSeverity(violation.Severity) >= params.severityLevel {
			filteredViolations = append(filteredViolations, violation)
		}
	}
	return filteredViolations
}

func filterVulnerabilities(vulnerabilities []services.Vulnerability, params *ScanGraphParams) []services.Vulnerability {
	var filteredVulnerabilities []services.Vulnerability
	for _, vulnerability := range vulnerabilities {
		if params.fixableOnly {
			vulnerability.Components = getFixableComponents(vulnerability.Components)
			if len(vulnerability.Components) == 0 {
				// All the components were filtered, filter this violation
				continue
			}
		}
		if getLevelOfSeverity(vulnerability.Severity) >= params.severityLevel {
			filteredVulnerabilities = append(filteredVulnerabilities, vulnerability)
		}
	}
	return filteredVulnerabilities
}

func getFixableComponents(components map[string]services.Component) map[string]services.Component {
	fixableComponents := make(map[string]services.Component)
	for vulnKey, vulnDetails := range components {
		if len(vulnDetails.FixedVersions) > 0 {
			fixableComponents[vulnKey] = vulnDetails
		}
	}
	return fixableComponents
}

func getLevelOfSeverity(s string) int {
	severity := utils.GetSeverity(cases.Title(language.Und).String(s), utils.ApplicabilityUndetermined)
	return severity.NumValue()
}
