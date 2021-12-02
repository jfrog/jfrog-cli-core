package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientconfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/version"
	"github.com/jfrog/jfrog-client-go/xray"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

const (
	GraphScanMinVersion = "3.29.0"
	ScanTypeMinVersion  = "3.37.2"
)

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

func ValidateXrayMinimumVersion(currentVersion, minimumVersion string) error {
	xrayVersion := version.NewVersion(currentVersion)
	if !xrayVersion.AtLeast(minimumVersion) {
		return errorutils.CheckErrorf("You are using Xray version " +
			string(xrayVersion.GetVersion()) + ", while this operation requires Xray version " + minimumVersion + " or higher.")
	}
	return nil
}

func RunScanGraphAndGetResults(serverDetails *config.ServerDetails, params services.XrayGraphScanParams, includeVulnerabilities, includeLicenses bool, xrayVersion string) (*services.ScanResponse, error) {
	xrayManager, err := CreateXrayServiceManager(serverDetails)
	if err != nil {
		return nil, err
	}
	// Remove scan type param if Xray version is under minimum supported version
	err = ValidateXrayMinimumVersion(xrayVersion, ScanTypeMinVersion)
	if err != nil {
		params.ScanType = ""
	}

	scanId, err := xrayManager.ScanGraph(params)
	if err != nil {
		return nil, err
	}
	return xrayManager.GetScanGraphResults(scanId, includeVulnerabilities, includeLicenses)
}
