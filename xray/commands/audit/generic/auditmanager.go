package audit

import (
	"errors"
	"fmt"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/yarn"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
	"os"
	"strings"

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
func GenericAudit(
	xrayGraphScanParams services.XrayGraphScanParams,
	serverDetails *config.ServerDetails,
	excludeTestDeps,
	useWrapper,
	insecureTls bool,
	args []string,
	progress ioUtils.ProgressMgr,
	requirementsFile string,
	ignoreConfigFile bool,
	technologies ...string) (results []services.ScanResponse, isMultipleRoot bool, err error) {

	// If no technologies were given, try to detect all types of technologies used.
	// Otherwise, run audit for requested technologies only.
	if len(technologies) == 0 {
		technologies, err = detectedTechnologies()
		if err != nil {
			return
		}
	}
	var errorList []string
	for _, tech := range coreutils.ToTechnologies(technologies) {
		var techResults []services.ScanResponse
		var isMultipleRootProject bool
		var e error
		if progress != nil {
			progress.SetHeadlineMsg(fmt.Sprintf("Calculating %v dependencies", tech))
		}
		switch tech {
		case coreutils.Maven:
			techResults, isMultipleRootProject, e = java.AuditMvn(xrayGraphScanParams, serverDetails, insecureTls, ignoreConfigFile, progress)
		case coreutils.Gradle:
			techResults, isMultipleRootProject, e = java.AuditGradle(xrayGraphScanParams, serverDetails, excludeTestDeps, useWrapper, ignoreConfigFile, progress)
		case coreutils.Npm:
			techResults, isMultipleRootProject, e = npm.AuditNpm(xrayGraphScanParams, serverDetails, args, progress)
		case coreutils.Yarn:
			techResults, isMultipleRootProject, e = yarn.AuditYarn(xrayGraphScanParams, serverDetails, progress)
		case coreutils.Go:
			techResults, isMultipleRootProject, e = _go.AuditGo(xrayGraphScanParams, serverDetails, progress)
		case coreutils.Pip:
			techResults, isMultipleRootProject, e = python.AuditPython(xrayGraphScanParams, serverDetails, pythonutils.Pip, progress, requirementsFile)
		case coreutils.Pipenv:
			techResults, isMultipleRootProject, e = python.AuditPython(xrayGraphScanParams, serverDetails, pythonutils.Pipenv, progress, "")
		case coreutils.Dotnet:
			continue
		case coreutils.Nuget:
			techResults, isMultipleRootProject, e = nuget.AuditNuget(xrayGraphScanParams, serverDetails, progress)
		default:
			e = errors.New(string(tech) + " is currently not supported")
		}
		if e != nil {
			// Save the error but continue to audit the next tech
			errorList = append(errorList, fmt.Sprintf("'%s' audit command failed: %s", tech, e.Error()))
		} else {
			results = append(results, techResults...)
			isMultipleRoot = isMultipleRootProject
		}
	}
	if len(errorList) > 0 {
		err = errors.New(strings.Join(errorList, "\n"))
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
