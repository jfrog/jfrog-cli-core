package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	clientconfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/xray"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"strings"
)

const (
	GraphScanMinXrayVersion           = "3.29.0"
	ScanTypeMinXrayVersion            = "3.37.2"
	BypassArchiveLimitsMinXrayVersion = "3.59.0"
)

type FilterLevel int

const (
	NoFilter FilterLevel = 0
	Low      FilterLevel = 1
	Medium   FilterLevel = 2
	High     FilterLevel = 3
	Critical FilterLevel = 4
)

var mapFilterNameToLevel = map[string]FilterLevel{
	"low":      Low,
	"medium":   Medium,
	"high":     High,
	"critical": Critical,
}

func GetFilterLevelFromSeverity(s string) FilterLevel {
	severity, exists := mapFilterNameToLevel[strings.ToLower(s)]
	if !exists {
		severity = NoFilter
	}
	return severity
}

type ScanGraphParams struct {
	serverDetails        *config.ServerDetails
	xrayGraphScanParams  *services.XrayGraphScanParams
	withFixVersionFilter bool
	xrayVersion          string
	filterLevel          FilterLevel
}

func NewScanGraphParams() *ScanGraphParams {
	return &ScanGraphParams{}
}

func (sgp *ScanGraphParams) SetServerDetails(serverDetails *config.ServerDetails) *ScanGraphParams {
	sgp.serverDetails = serverDetails
	return sgp
}

func (sgp *ScanGraphParams) SetXrayGraphScanParams(params *services.XrayGraphScanParams) *ScanGraphParams {
	sgp.xrayGraphScanParams = params
	return sgp
}

func (sgp *ScanGraphParams) SetXrayVersion(xrayVersion string) *ScanGraphParams {
	sgp.xrayVersion = xrayVersion
	return sgp
}

func (sgp *ScanGraphParams) SetFilterLevel(filterLevel string) *ScanGraphParams {
	sgp.filterLevel = GetFilterLevelFromSeverity(filterLevel)
	return sgp
}

func (sgp *ScanGraphParams) XrayGraphScanParams() *services.XrayGraphScanParams {
	return sgp.xrayGraphScanParams
}

func (sgp *ScanGraphParams) XrayVersion() string {
	return sgp.xrayVersion
}

func (sgp *ScanGraphParams) ServerDetails() *config.ServerDetails {
	return sgp.serverDetails
}

func (sgp *ScanGraphParams) WithFixVersionFilter() bool {
	return sgp.withFixVersionFilter
}

func (sgp *ScanGraphParams) SetWithFixVersionFilter(withFixVersionFilter bool) *ScanGraphParams {
	sgp.withFixVersionFilter = withFixVersionFilter
	return sgp
}

func CreateXrayServiceManager(serviceDetails *config.ServerDetails) (*xray.XrayServicesManager, error) {
	xrayDetails, err := serviceDetails.CreateXrayAuthConfig()
	if err != nil {
		return nil, err
	}
	serviceConfig, err := clientconfig.NewConfigBuilder().
		SetServiceDetails(xrayDetails).
		Build()
	if err != nil {
		return nil, err
	}
	return xray.New(serviceConfig)
}

func RunScanGraphAndGetResults(params *ScanGraphParams) (*services.ScanResponse, error) {
	xrayManager, err := CreateXrayServiceManager(params.serverDetails)
	if err != nil {
		return nil, err
	}

	err = coreutils.ValidateMinimumVersion(coreutils.Xray, params.xrayVersion, ScanTypeMinXrayVersion)
	if err != nil {
		// Remove scan type param if Xray version is under the minimum supported version
		params.xrayGraphScanParams.ScanType = ""
	}
	scanId, err := xrayManager.ScanGraph(*params.xrayGraphScanParams)
	if err != nil {
		return nil, err
	}
	scanResult, err := xrayManager.GetScanGraphResults(scanId, params.XrayGraphScanParams().IncludeVulnerabilities, params.XrayGraphScanParams().IncludeLicenses)
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
	return params.filterLevel != NoFilter || params.withFixVersionFilter
}

func filterViolations(violations []services.Violation, params *ScanGraphParams) []services.Violation {
	var filteredViolations []services.Violation
	for _, violation := range violations {
		if params.withFixVersionFilter {
			violation.Components = getWithFixVersionFilteredComponents(violation.Components)
			if len(violation.Components) == 0 {
				// All the components were filtered, filter this violation
				continue
			}
		}
		if GetFilterLevelFromSeverity(violation.Severity) >= params.filterLevel {
			filteredViolations = append(filteredViolations, violation)
		}
	}
	return filteredViolations
}

func filterVulnerabilities(vulnerabilities []services.Vulnerability, params *ScanGraphParams) []services.Vulnerability {
	var filteredVulnerabilities []services.Vulnerability
	for _, vulnerability := range vulnerabilities {
		if params.withFixVersionFilter {
			vulnerability.Components = getWithFixVersionFilteredComponents(vulnerability.Components)
			if len(vulnerability.Components) == 0 {
				// All the components were filtered, filter this violation
				continue
			}
		}
		if GetFilterLevelFromSeverity(vulnerability.Severity) >= params.filterLevel {
			filteredVulnerabilities = append(filteredVulnerabilities, vulnerability)
		}
	}
	return filteredVulnerabilities
}

func getWithFixVersionFilteredComponents(components map[string]services.Component) map[string]services.Component {
	withFixVersionsComponents := make(map[string]services.Component)
	for vulnKey, vulnDetails := range components {
		if len(vulnDetails.FixedVersions) > 0 {
			withFixVersionsComponents[vulnKey] = vulnDetails
		}
	}
	return withFixVersionsComponents
}

func CreateXrayServiceManagerAndGetVersion(serviceDetails *config.ServerDetails) (*xray.XrayServicesManager, string, error) {
	xrayManager, err := CreateXrayServiceManager(serviceDetails)
	if err != nil {
		return nil, "", err
	}
	xrayVersion, err := xrayManager.GetVersion()
	if err != nil {
		return nil, "", err
	}
	return xrayManager, xrayVersion, nil
}
