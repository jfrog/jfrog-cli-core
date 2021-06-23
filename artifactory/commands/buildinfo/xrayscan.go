package buildinfo

import (
	"encoding/json"

	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type BuildScanCommand struct {
	buildConfiguration *utils.BuildConfiguration
	failBuild          bool
	serverDetails      *config.ServerDetails
}

func NewBuildScanCommand() *BuildScanCommand {
	return &BuildScanCommand{}
}

func (bsc *BuildScanCommand) SetServerDetails(serverDetails *config.ServerDetails) *BuildScanCommand {
	bsc.serverDetails = serverDetails
	return bsc
}

func (bsc *BuildScanCommand) SetFailBuild(failBuild bool) *BuildScanCommand {
	bsc.failBuild = failBuild
	return bsc
}

func (bsc *BuildScanCommand) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *BuildScanCommand {
	bsc.buildConfiguration = buildConfiguration
	return bsc
}

func (bsc *BuildScanCommand) CommandName() string {
	return "rt_build_scan"
}

func (bsc *BuildScanCommand) ServerDetails() (*config.ServerDetails, error) {
	return bsc.serverDetails, nil
}

func (bsc *BuildScanCommand) Run() error {
	log.Info("Triggered Xray build scan... The scan may take a few minutes.")
	servicesManager, err := utils.CreateServiceManager(bsc.serverDetails, -1, false)
	if err != nil {
		return err
	}

	xrayScanParams := getXrayScanParams(*bsc.buildConfiguration)
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

func getXrayScanParams(buildConfiguration utils.BuildConfiguration) services.XrayScanParams {
	xrayScanParams := services.NewXrayScanParams()
	xrayScanParams.BuildName = buildConfiguration.BuildName
	xrayScanParams.BuildNumber = buildConfiguration.BuildNumber
	xrayScanParams.ProjectKey = buildConfiguration.Project

	return xrayScanParams
}
