package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	clientconfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"os"
)

const (
	GraphScanMinXrayVersion           = "3.29.0"
	ScanTypeMinXrayVersion            = "3.37.2"
	BypassArchiveLimitsMinXrayVersion = "3.59.0"
	TotalConcurrentRequests           = 10
)

func getLevelOfSeverity(s string) int {
	severity := utils.GetSeverity(cases.Title(language.Und).String(s), utils.ApplicabilityUndeterminedStringValue)
	return severity.NumValue()
}

type ScanGraphParams struct {
	serverDetails       *config.ServerDetails
	xrayGraphScanParams *services.XrayGraphScanParams
	fixableOnly         bool
	xrayVersion         string
	severityLevel       int
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

func (sgp *ScanGraphParams) SetSeverityLevel(severity string) *ScanGraphParams {
	sgp.severityLevel = getLevelOfSeverity(severity)
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

func (sgp *ScanGraphParams) FixableOnly() bool {
	return sgp.fixableOnly
}

func (sgp *ScanGraphParams) SetFixableOnly(fixable bool) *ScanGraphParams {
	sgp.fixableOnly = fixable
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

func DetectedTechnologies() (technologies []string) {
	wd, err := os.Getwd()
	if errorutils.CheckError(err) != nil {
		return
	}
	detectedTechnologies, err := coreutils.DetectTechnologies(wd, false, false)
	if err != nil {
		return
	}
	detectedTechnologiesString := coreutils.DetectedTechnologiesToString(detectedTechnologies)
	if detectedTechnologiesString == "" {
		log.Info("Couldn't determine a package manager or build tool used by this project in the current path:", wd)
		return
	}
	log.Info("Detected: " + detectedTechnologiesString)
	return coreutils.DetectedTechnologiesToSlice(detectedTechnologies)
}

func DetectNumOfThreads(threadsCount int) (int, error) {
	if threadsCount > TotalConcurrentRequests {
		return 0, errorutils.CheckErrorf("number of threads crossed the maximum, the maximum threads allowed is %v", TotalConcurrentRequests)
	}
	return threadsCount, nil
}
