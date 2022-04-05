package audit

import (
	"os"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	_go "github.com/jfrog/jfrog-cli-core/v2/xray/audit/go"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/java"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/npm"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/nuget"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/python"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

// GenericAudit audits the project found in the current directory using Xray.
func GenericAudit(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails, excludeTestDeps, useWrapper, insecureTls bool, args []string, technologies ...string) (results []services.ScanResponse, isMultipleRootProject bool, err error) {
	// If no technologies were given, try to detect all types of technologies that in used.
	// Otherwise run audit for requested technologies only.
	if len(technologies) == 0 {
		technologies, err = detectedTechnologies()
		if err != nil {
			return
		}
	}

	for _, tech := range coreutils.ToTechnologies(technologies) {
		switch tech {
		case coreutils.Maven:
			results, isMultipleRootProject, err = java.AuditMvn(xrayGraphScanPrams, serverDetails, insecureTls)
		case coreutils.Gradle:
			results, isMultipleRootProject, err = java.AuditGradle(xrayGraphScanPrams, serverDetails, excludeTestDeps, useWrapper)
		case coreutils.Npm:
			results, isMultipleRootProject, err = npm.AuditNpm(xrayGraphScanPrams, serverDetails, args)
		case coreutils.Go:
			results, isMultipleRootProject, err = _go.AuditGo(xrayGraphScanPrams, serverDetails)
		case coreutils.Pip:
			results, isMultipleRootProject, err = python.AuditPip(xrayGraphScanPrams, serverDetails)
		case coreutils.Pipenv:
			results, isMultipleRootProject, err = python.AuditPipenv(xrayGraphScanPrams, serverDetails)
		case coreutils.Dotnet:
			continue
		case coreutils.Nuget:
			results, isMultipleRootProject, err = nuget.AuditNuget(xrayGraphScanPrams, serverDetails)
		default:
			log.Info(string(tech), " is currently not supported")
		}
	}
	return
}

func detectedTechnologies() (technologies []string, err error) {
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
