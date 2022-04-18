package scan

import (
	"errors"
	rtutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	BuildScanMinVersion                       = "3.37.0"
	BuildScanIncludeVulnerabilitiesMinVersion = "3.40.0"
)

type BuildScanCommand struct {
	serverDetails          *config.ServerDetails
	outputFormat           xrutils.OutputFormat
	buildConfiguration     *rtutils.BuildConfiguration
	includeVulnerabilities bool
	failBuild              bool
	printExtendedTable     bool
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

func (bsc *BuildScanCommand) SetPrintExtendedTable(printExtendedTable bool) *BuildScanCommand {
	bsc.printExtendedTable = printExtendedTable
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
	if bsc.includeVulnerabilities {
		err = commands.ValidateXrayMinimumVersion(xrayVersion, BuildScanIncludeVulnerabilitiesMinVersion)
		if err != nil {
			return errors.New("build-scan command with '--vuln' flag is not supported on your current Xray version. " + err.Error())
		}
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
		return err
	}
	// If failBuild flag is true and also got fail build response from Xray
	if bsc.failBuild && isFailBuildResponse {
		return xrutils.NewFailBuildError()
	}
	return
}

func (bsc *BuildScanCommand) runBuildScanAndPrintResults(xrayManager *xray.XrayServicesManager, params services.XrayBuildParams) (isFailBuildResponse bool, err error) {
	buildScanResults, noFailBuildPolicy, err := xrayManager.BuildScan(params, bsc.includeVulnerabilities)
	if err != nil {
		return false, err
	}
	log.Info("The scan data is available at: " + buildScanResults.MoreDetailsUrl)
	isFailBuildResponse = buildScanResults.FailBuild

	scanResponse := []services.ScanResponse{{
		Violations:      buildScanResults.Violations,
		Vulnerabilities: buildScanResults.Vulnerabilities,
		XrayDataUrl:     buildScanResults.MoreDetailsUrl,
	}}

	if bsc.outputFormat == xrutils.Json || bsc.outputFormat == xrutils.SimpleJson {
		// Print the violations and/or vulnerabilities as part of one JSON.
		err = xrutils.PrintScanResults(scanResponse, nil, bsc.outputFormat, false, false, false, bsc.printExtendedTable)
	} else {
		// Print two different tables for violations and vulnerabilities (if needed)

		// If "No Xray Fail build policy...." error received, no need to print violations
		if !noFailBuildPolicy {
			err = xrutils.PrintScanResults(scanResponse, nil, bsc.outputFormat, false, false, false, bsc.printExtendedTable)
			if err != nil {
				return false, err
			}
		}
		if bsc.includeVulnerabilities {
			err = xrutils.PrintScanResults(scanResponse, nil, bsc.outputFormat, true, false, false, bsc.printExtendedTable)
			if err != nil {
				return false, err
			}
		}
	}
	return
}

func (bsc *BuildScanCommand) CommandName() string {
	return "xr_build_scan"
}
