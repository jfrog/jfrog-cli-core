package scan

import (
	rtutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"strings"
)

type BuildScanV2Command struct {
	serverDetails          *config.ServerDetails
	outputFormat           xrutils.OutputFormat
	buildConfiguration     *rtutils.BuildConfiguration
	includeVulnerabilities bool
	failBuild              bool
}

func NewBuildScanV2Command() *BuildScanV2Command {
	return &BuildScanV2Command{}
}

func (bsc *BuildScanV2Command) SetServerDetails(server *config.ServerDetails) *BuildScanV2Command {
	bsc.serverDetails = server
	return bsc
}

func (bsc *BuildScanV2Command) SetOutputFormat(format xrutils.OutputFormat) *BuildScanV2Command {
	bsc.outputFormat = format
	return bsc
}

func (bsc *BuildScanV2Command) ServerDetails() (*config.ServerDetails, error) {
	return bsc.serverDetails, nil
}

func (bsc *BuildScanV2Command) SetBuildConfiguration(buildConfiguration *rtutils.BuildConfiguration) *BuildScanV2Command {
	bsc.buildConfiguration = buildConfiguration
	return bsc
}

func (bsc *BuildScanV2Command) SetIncludeVulnerabilities(include bool) *BuildScanV2Command {
	bsc.includeVulnerabilities = include
	return bsc
}

func (bsc *BuildScanV2Command) SetFailBuild(failBuild bool) *BuildScanV2Command {
	bsc.failBuild = failBuild
	return bsc
}

func (bsc *BuildScanV2Command) Run() (err error) {
	xrayManager, err := commands.CreateXrayServiceManager(bsc.serverDetails)
	if err != nil {
		return err
	}
	params := services.XrayBuildParams{
		BuildName:   bsc.buildConfiguration.BuildName,
		BuildNumber: bsc.buildConfiguration.BuildNumber,
		Project:     bsc.buildConfiguration.Project,
	}

	failBuild, err := bsc.runBuildScanAndPrintResults(xrayManager, params)
	if err != nil {
		return err
	}
	defer func() {
		if failBuild {
			// deferred so if build summery fails it will still throw build error if needed
			if err != nil {
				log.Error(err)
			}
			err = xrutils.ThrowFailBuildError()
		}
	}()
	if bsc.includeVulnerabilities {
		// if vulnerabilities flag is true, get vulnerabilities from xray with build-summery and print to output
		err = bsc.runBuildSummaryAndPrintVulnerabilities(xrayManager, params)
		if err != nil {
			return err
		}
	}
	return nil
}

func (bsc *BuildScanV2Command) runBuildScanAndPrintResults(xrayManager *xray.XrayServicesManager, params services.XrayBuildParams) (bool, error) {
	err := xrayManager.BuildScanV2(params)
	if err != nil {
		return false, err
	}
	buildScanResults, err := xrayManager.GetBuildScanResults(params)
	if err != nil {
		return false, err
	}
	scanResponseArray := []services.ScanResponse{{Violations: buildScanResults.Violations}}
	err = xrutils.PrintScanResults(scanResponseArray, bsc.outputFormat == xrutils.Table, false, false, false)
	if err != nil {
		return false, err
	}
	return xrutils.CheckIfFailBuild(true, scanResponseArray), nil
}

func (bsc *BuildScanV2Command) runBuildSummaryAndPrintVulnerabilities(xrayManager *xray.XrayServicesManager, params services.XrayBuildParams) error {
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

func getCvesField(summeryCves []services.SummeryCve) []services.Cve {
	// The response from summery is score combined with vector, so we take the score only
	// example: "4.0/CVSS:2.0/AV:N/AC:L/Au:S/C:N/I:N/A:P"
	var cves []services.Cve
	for _, summeryCve := range summeryCves {
		cve := services.Cve{
			Id:          summeryCve.Id,
			CvssV2Score: strings.Split(summeryCve.CvssV2Score, "/")[0],
			CvssV3Score: strings.Split(summeryCve.CvssV3Score, "/")[0],
		}
		cves = append(cves, cve)
	}
	return cves
}

func getComponentsField(summeryComponents []services.SummeryComponent, impactPaths []string, buildName string) map[string]services.Component {
	components := map[string]services.Component{}

	for _, component := range summeryComponents {

		// example: "com.fasterxml.jackson.core:jackson-databind" >> "jackson-databind"
		componentShortName := component.ComponentId[strings.LastIndex(component.ComponentId, ":")+1:]

		var componentImpactPaths [][]services.ImpactPathNode
		for _, impactPath := range impactPaths {
			// search for all impact paths that contain the package
			if strings.Contains(strings.ToLower(impactPath), strings.ToLower(componentShortName)) {
				pathNode := []services.ImpactPathNode{{ComponentId: getRootComponentFromImpactPath(impactPath, buildName)}}
				componentImpactPaths = append(componentImpactPaths, pathNode)
			}
		}

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

func (bsc *BuildScanV2Command) CommandName() string {
	return "xr_build_scan_v2"
}
