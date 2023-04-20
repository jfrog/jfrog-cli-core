package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	clientconfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"os"
)

const (
	GraphScanMinXrayVersion           = "3.29.0"
	ScanTypeMinXrayVersion            = "3.37.2"
	BypassArchiveLimitsMinXrayVersion = "3.59.0"
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

func RunScanGraphAndGetResults(serverDetails *config.ServerDetails, params services.XrayGraphScanParams, includeVulnerabilities, includeLicenses bool, xrayVersion string) (*services.ScanResponse, error) {
	xrayManager, err := CreateXrayServiceManager(serverDetails)
	if err != nil {
		return nil, err
	}

	err = coreutils.ValidateMinimumVersion(coreutils.Xray, xrayVersion, ScanTypeMinXrayVersion)
	if err != nil {
		// Remove scan type param if Xray version is under minimum supported version
		params.ScanType = ""
	}
	scanId, err := xrayManager.ScanGraph(params)
	if err != nil {
		return nil, err
	}
	return xrayManager.GetScanGraphResults(scanId, includeVulnerabilities, includeLicenses)
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

func DetectedTechnologies() (technologies []string, err error) {
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
		return nil, errorutils.CheckErrorf("could not determine the package manager / build tool used by this project.")
	}
	log.Info("Detected: " + detectedTechnologiesString)
	return coreutils.DetectedTechnologiesToSlice(detectedTechnologies), nil
}
