package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientconfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/xray/manager"
)

func CreateXrayServiceManager(serviceDetails *config.ServerDetails) (xrayManager manager.SecurityServiceManager, err error) {
	xrayDetails, err := serviceDetails.CreateXrayAuthConfig()
	if err != nil {
		return
	}
	serviceConfig, err := clientconfig.NewConfigBuilder().
		SetServiceDetails(xrayDetails).
		Build()
	if err != nil {
		return
	}
	return manager.New(serviceConfig)
}

func CreateXrayServiceManagerAndGetVersion(serviceDetails *config.ServerDetails) (xrayManager manager.SecurityServiceManager, xrayVersion string, err error) {
	xrayManager, err = CreateXrayServiceManager(serviceDetails)
	if err != nil {
		return
	}
	xrayVersion, err = xrayManager.GetVersion()
	if err != nil {
		return
	}
	return
}
