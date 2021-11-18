package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientconfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/xray"
	"github.com/jfrog/jfrog-client-go/xray/services"
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

func RunScanGraphAndGetResults(serverDetails *config.ServerDetails, params services.XrayGraphScanParams, includeVulnerabilities, includeLicenses bool) (*services.ScanResponse, error) {
	xrayManager, err := CreateXrayServiceManager(serverDetails)
	if err != nil {
		return nil, err
	}
	scanId, err := xrayManager.ScanGraph(params)
	if err != nil {
		return nil, err
	}
	return xrayManager.GetScanGraphResults(scanId, includeVulnerabilities, includeLicenses)
}
