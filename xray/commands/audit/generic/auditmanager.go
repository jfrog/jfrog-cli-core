package audit

import (
	"github.com/jfrog/build-info-go/utils/pythonutils"
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
	// Otherwise, run audit for requested technologies only.
	if len(technologies) == 0 {
		technologies, err = detectedTechnologies()
		if err != nil {
			return
		}
	}

	for _, tech := range coreutils.ToTechnologies(technologies) {
		var techResults []services.ScanResponse
		var e error
		switch tech {
		case coreutils.Maven:
			techResults, isMultipleRootProject, e = java.AuditMvn(xrayGraphScanPrams, serverDetails, insecureTls)
		case coreutils.Gradle:
			techResults, isMultipleRootProject, e = java.AuditGradle(xrayGraphScanPrams, serverDetails, excludeTestDeps, useWrapper)
		case coreutils.Npm:
			techResults, isMultipleRootProject, e = npm.AuditNpm(xrayGraphScanPrams, serverDetails, args)
		case coreutils.Go:
			techResults, isMultipleRootProject, e = _go.AuditGo(xrayGraphScanPrams, serverDetails)
		case coreutils.Pip:
			techResults, isMultipleRootProject, e = python.AuditPython(xrayGraphScanPrams, serverDetails, pythonutils.Pip)
		case coreutils.Pipenv:
			techResults, isMultipleRootProject, e = python.AuditPython(xrayGraphScanPrams, serverDetails, pythonutils.Pipenv)
		case coreutils.Dotnet:
			continue
		case coreutils.Nuget:
			techResults, isMultipleRootProject, e = nuget.AuditNuget(xrayGraphScanPrams, serverDetails)
		default:
			log.Info(string(tech), " is currently not supported")
		}
		if e != nil {
			// Save the error but continue to audit the next tech
			err = e
		} else {
			results = append(results, techResults...)
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
