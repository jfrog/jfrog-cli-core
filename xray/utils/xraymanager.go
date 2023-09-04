package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientconfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/xray"
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
