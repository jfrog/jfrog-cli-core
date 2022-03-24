package audit

import (
	"os"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	_go "github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/go"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/java"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/npm"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/nuget"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/python"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
)

func GenericAudit(xrayGraphScanPrams services.XrayGraphScanParams, serverDetails *config.ServerDetails, excludeTestDeps, useWrapper, insecureTls bool, args []string, technologies ...string) (results []services.ScanResponse, isMultipleRootProject bool, err error) {

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
			break
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
		log.Info("Could not determine the package manager / build tool used by this project.")
		return
	}
	log.Info("Detected: " + detectedTechnologiesString)
	return coreutils.DetectedTechnologiesToSlice(detectedTechnologies), nil
}
