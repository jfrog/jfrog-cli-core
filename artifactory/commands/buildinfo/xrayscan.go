package buildinfo

import (
	"encoding/json"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Deprecated BuildScan Command. The new build scan command is "xray/commands/scan/buildscan"
type BuildScanLegacyCommand struct {
	buildConfiguration *build.BuildConfiguration
	failBuild          bool
	serverDetails      *config.ServerDetails
}

func NewBuildScanLegacyCommand() *BuildScanLegacyCommand {
	return &BuildScanLegacyCommand{}
}

func (bsc *BuildScanLegacyCommand) SetServerDetails(serverDetails *config.ServerDetails) *BuildScanLegacyCommand {
	bsc.serverDetails = serverDetails
	return bsc
}

func (bsc *BuildScanLegacyCommand) SetFailBuild(failBuild bool) *BuildScanLegacyCommand {
	bsc.failBuild = failBuild
	return bsc
}

func (bsc *BuildScanLegacyCommand) SetBuildConfiguration(buildConfiguration *build.BuildConfiguration) *BuildScanLegacyCommand {
	bsc.buildConfiguration = buildConfiguration
	return bsc
}

func (bsc *BuildScanLegacyCommand) CommandName() string {
	return "rt_build_scan_legacy"
}

func (bsc *BuildScanLegacyCommand) ServerDetails() (*config.ServerDetails, error) {
	return bsc.serverDetails, nil
}

func (bsc *BuildScanLegacyCommand) Run() error {
	log.Info("Triggered Xray build scan... The scan may take a few minutes.")
	servicesManager, err := utils.CreateServiceManager(bsc.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}

	xrayScanParams, err := getXrayScanParams(*bsc.buildConfiguration)
	if err != nil {
		return err
	}
	result, err := servicesManager.XrayScanBuild(xrayScanParams)
	if err != nil {
		return err
	}

	var scanResults scanResult
	err = json.Unmarshal(result, &scanResults)
	if errorutils.CheckError(err) != nil {
		return err
	}

	log.Info("Xray scan completed.")
	log.Output(clientutils.IndentJson(result))

	// Check if should fail build
	if bsc.failBuild && scanResults.Summary.FailBuild {
		// We're specifically returning the 'buildScanError' and not a regular error
		// to indicate that Xray indeed scanned the build, and the failure is not due to
		// networking connectivity or other issues.
		return errorutils.CheckError(utils.GetBuildScanError())
	}

	return err
}

// To unmarshal xray scan summary result
type scanResult struct {
	Summary scanSummary `json:"summary,omitempty"`
}

type scanSummary struct {
	TotalAlerts int    `json:"total_alerts,omitempty"`
	FailBuild   bool   `json:"fail_build,omitempty"`
	Message     string `json:"message,omitempty"`
	Url         string `json:"more_details_url,omitempty"`
}

func getXrayScanParams(buildConfiguration build.BuildConfiguration) (services.XrayScanParams, error) {
	xrayScanParams := services.NewXrayScanParams()
	buildName, err := buildConfiguration.GetBuildName()
	if err != nil {
		return xrayScanParams, err
	}
	buildNumber, err := buildConfiguration.GetBuildNumber()
	if err != nil {
		return xrayScanParams, err
	}
	xrayScanParams.BuildName = buildName
	xrayScanParams.BuildNumber = buildNumber
	xrayScanParams.ProjectKey = buildConfiguration.GetProject()

	return xrayScanParams, nil
}
