package audit

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"os"
	"strings"

	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	_go "github.com/jfrog/jfrog-cli-core/v2/xray/audit/go"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/java"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/npm"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/nuget"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/python"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/yarn"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
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
		var dependencyTrees []*services.GraphNode
		var e error
		if progress != nil {
			progress.SetHeadlineMsg(fmt.Sprintf("Calculating %v dependencies", tech.ToFormal()))
		}
		switch tech {
		case coreutils.Maven:
			dependencyTrees, e = java.BuildMvnDependencyTree(insecureTls, ignoreConfigFile)
		case coreutils.Gradle:
			dependencyTrees, e = java.BuildGradleDependencyTree(excludeTestDeps, useWrapper, ignoreConfigFile)
		case coreutils.Npm:
			dependencyTrees, e = npm.BuildDependencyTree(args)
		case coreutils.Yarn:
			dependencyTrees, e = yarn.BuildDependencyTree()
		case coreutils.Go:
			dependencyTrees, e = _go.BuildDependencyTree()
		case coreutils.Pipenv, coreutils.Pip, coreutils.Poetry:
			dependencyTrees, e = python.BuildDependencyTree(pythonutils.PythonTool(tech), requirementsFile)
		case coreutils.Dotnet:
			continue
		case coreutils.Nuget:
			dependencyTrees, e = nuget.BuildDependencyTree()
		default:
			e = errors.New(string(tech) + " is currently not supported")
		}

		var techResults services.ScanResponse
		if e == nil {
			// If building the dependency tree was successful, run Xray scan.
			techResults, e = audit.Audit(dependencyTrees, xrayGraphScanParams, serverDetails, progress, tech)
		}

		if e != nil {
			// Save the error but continue to audit the next tech
			errorList = append(errorList, fmt.Sprintf("'%s' audit command failed:\n%s", tech, e.Error()))
		} else {
			results = append(results, techResults)
			isMultipleRoot = len(dependencyTrees) > 1
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
