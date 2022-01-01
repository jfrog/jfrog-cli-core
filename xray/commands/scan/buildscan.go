package scan

import (
	"fmt"
	"strings"

	rtutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	BuildScanMinVersion = "3.37.0"
)

type BuildScanCommand struct {
	serverDetails          *config.ServerDetails
	outputFormat           xrutils.OutputFormat
	buildConfiguration     *rtutils.BuildConfiguration
	includeVulnerabilities bool
	failBuild              bool
}

func NewBuildScanCommand() *BuildScanCommand {
	return &BuildScanCommand{}
}

func (bsc *BuildScanCommand) SetServerDetails(server *config.ServerDetails) *BuildScanCommand {
	bsc.serverDetails = server
	return bsc
}

func (bsc *BuildScanCommand) SetOutputFormat(format xrutils.OutputFormat) *BuildScanCommand {
	bsc.outputFormat = format
	return bsc
}

func (bsc *BuildScanCommand) ServerDetails() (*config.ServerDetails, error) {
	return bsc.serverDetails, nil
}

func (bsc *BuildScanCommand) SetBuildConfiguration(buildConfiguration *rtutils.BuildConfiguration) *BuildScanCommand {
	bsc.buildConfiguration = buildConfiguration
	return bsc
}

func (bsc *BuildScanCommand) SetIncludeVulnerabilities(include bool) *BuildScanCommand {
	bsc.includeVulnerabilities = include
	return bsc
}

func (bsc *BuildScanCommand) SetFailBuild(failBuild bool) *BuildScanCommand {
	bsc.failBuild = failBuild
	return bsc
}

// Scan published builds with Xray
func (bsc *BuildScanCommand) Run() (err error) {
	xrayManager, xrayVersion, err := commands.CreateXrayServiceManagerAndGetVersion(bsc.serverDetails)
	if err != nil {
		return err
	}
	err = commands.ValidateXrayMinimumVersion(xrayVersion, BuildScanMinVersion)
	if err != nil {
		return err
	}
	buildName, err := bsc.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := bsc.buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}
	params := services.XrayBuildParams{
		BuildName:   buildName,
		BuildNumber: buildNumber,
		Project:     bsc.buildConfiguration.GetProject(),
	}

	isFailBuildResponse, err := bsc.runBuildScanAndPrintResults(xrayManager, params)
	if err != nil {
		if strings.Contains(err.Error(), services.XrayScanBuildNoFailBuildPolicy) {
			// If the error is: "No Xray “Fail build in case of a violation” policy rule has been defined on this build",
			// We still continue to build summery if needed
			log.Info(err.Error())
		} else {
			return err
		}
	}
	defer func() {
		// If failBuild flag is true and also got fail build response from Xray
		if bsc.failBuild && isFailBuildResponse {
			// Deferred so if build summary fails, it will still return a fail build error if needed
			if err != nil {
				log.Error(err)
			}
			err = xrutils.NewFailBuildError()
		}
	}()

	if bsc.includeVulnerabilities {
		// If vulnerabilities flag is true, get vulnerabilities from xray with build-summary and print to output
		log.Info("Getting the build-summary from Xray...")
		err = bsc.runBuildSummaryAndPrintResults(xrayManager, params)
		if err != nil {
			return err
		}
	}
	return nil
}

func (bsc *BuildScanCommand) runBuildScanAndPrintResults(xrayManager *xray.XrayServicesManager, params services.XrayBuildParams) (bool, error) {
	buildScanResults, err := xrayManager.BuildScan(params)
	if err != nil {
		return false, err
	}
	scanResponseArray := []services.ScanResponse{{Violations: buildScanResults.Violations, XrayDataUrl: buildScanResults.MoreDetailsUrl}}
	fmt.Println("The scan data is available at: " + buildScanResults.MoreDetailsUrl)
	err = xrutils.PrintScanResults(scanResponseArray, bsc.outputFormat == xrutils.Table, false, false, false)
	if err != nil {
		return false, err
	}
	return xrutils.CheckIfFailBuild(scanResponseArray), nil
}

func (bsc *BuildScanCommand) runBuildSummaryAndPrintResults(xrayManager *xray.XrayServicesManager, params services.XrayBuildParams) error {
	summaryResponse, err := xrayManager.BuildSummary(params)
	if err != nil {
		return err
	}
	scanResponse := services.ScanResponse{Vulnerabilities: convertIssuesToVulnerabilities(summaryResponse.Issues, params)}
	return xrutils.PrintScanResults([]services.ScanResponse{scanResponse}, bsc.outputFormat == xrutils.Table, true, false, false)
}

func convertIssuesToVulnerabilities(issues []services.Issue, params services.XrayBuildParams) []services.Vulnerability {
	var vulnerabilities []services.Vulnerability
	for _, issue := range issues {
		vuln := services.Vulnerability{
			Summary:    issue.Summary,
			Severity:   issue.Severity,
			Cves:       getCvesField(issue.Cves),
			Components: getComponentsField(issue.Components, issue.ImpactPath, params.BuildName),
			IssueId:    issue.IssueId,
		}
		vulnerabilities = append(vulnerabilities, vuln)
	}

	return vulnerabilities
}

func getCvesField(summaryCves []services.SummeryCve) []services.Cve {
	// The build-summary API response includes both the score and the vector. We're taking the score only
	// Example: "4.0/CVSS:2.0/AV:N/AC:L/Au:S/C:N/I:N/A:P"  >> "4.0"
	var cves []services.Cve
	for _, summaryCve := range summaryCves {
		cve := services.Cve{
			Id:          summaryCve.Id,
			CvssV2Score: strings.Split(summaryCve.CvssV2Score, "/")[0],
			CvssV3Score: strings.Split(summaryCve.CvssV3Score, "/")[0],
		}
		cves = append(cves, cve)
	}
	return cves
}

func getComponentsField(summaryComponents []services.SummeryComponent, impactPaths []string, buildName string) map[string]services.Component {
	components := map[string]services.Component{}
	for _, component := range summaryComponents {
		componentImpactPaths := getComponentImpactPaths(component.ComponentId, buildName, impactPaths)
		if len(componentImpactPaths) > 0 {
			components[component.ComponentId] = services.Component{
				FixedVersions: component.FixedVersions,
				ImpactPaths:   componentImpactPaths,
			}
		}
	}
	return components
}

func getRootComponentFromImpactPath(impactPath, buildName string) string {
	// impactedPath example: "default/builds/buildName/bill.jar/x/x/component/x"
	// root component - bill.jar
	trimPrefix := impactPath[strings.Index(impactPath, "/")+1:]
	trimBuild := strings.TrimPrefix(trimPrefix, "builds/"+buildName+"/")
	rootComponent := strings.Split(trimBuild, "/")[0]
	return rootComponent
}

func getComponentImpactPaths(componentId, buildName string, impactPaths []string) [][]services.ImpactPathNode {
	// componentShortName example: "com.fasterxml.jackson.core:jackson-databind" >> "jackson-databind"
	componentShortName := componentId[strings.LastIndex(componentId, ":")+1:]

	var componentImpactPaths [][]services.ImpactPathNode
	for _, impactPath := range impactPaths {
		// Search for all impact paths that contain the package
		if strings.Contains(impactPath, componentShortName) {
			pathNode := []services.ImpactPathNode{{ComponentId: getRootComponentFromImpactPath(impactPath, buildName)}}
			componentImpactPaths = append(componentImpactPaths, pathNode)
		}
	}
	return componentImpactPaths
}

func (bsc *BuildScanCommand) CommandName() string {
	return "xr_build_scan"
}
