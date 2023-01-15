package audit

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	"os"
	"path/filepath"
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

type Params struct {
	xrayGraphScanParams services.XrayGraphScanParams
	serverDetails       *config.ServerDetails
	progress            ioUtils.ProgressMgr
	ignoreConfigFile    bool
	excludeTestDeps     bool
	insecureTls         bool
	useWrapper          bool
	requirementsFile    string
	technologies        []string
	workingDirs         []string
	args                []string
}

func NewAuditParams() *Params {
	return &Params{}
}

func (params *Params) SetXrayGraphScanParams(xrayGraphScanParams services.XrayGraphScanParams) *Params {
	params.xrayGraphScanParams = xrayGraphScanParams
	return params
}

func (params *Params) SetServerDetails(serverDetails *config.ServerDetails) *Params {
	params.serverDetails = serverDetails
	return params
}

func (params *Params) SetExcludeTestDeps(excludeTestDeps bool) *Params {
	params.excludeTestDeps = excludeTestDeps
	return params
}

func (params *Params) SetUseWrapper(useWrapper bool) *Params {
	params.useWrapper = useWrapper
	return params
}

func (params *Params) SetInsecureTLS(insecureTls bool) *Params {
	params.insecureTls = insecureTls
	return params
}

func (params *Params) SetArgs(args []string) *Params {
	params.args = args
	return params
}

func (params *Params) SetProgressBar(progress ioUtils.ProgressMgr) *Params {
	params.progress = progress
	return params
}

func (params *Params) SetRequirementsFile(requirementsFile string) *Params {
	params.requirementsFile = requirementsFile
	return params
}

func (params *Params) SetIgnoreConfigFile(ignoreConfigFile bool) *Params {
	params.ignoreConfigFile = ignoreConfigFile
	return params
}

func (params *Params) SetWorkingDirs(workingDirs []string) *Params {
	params.workingDirs = workingDirs
	return params
}

func (params *Params) SetTechnologies(technologies ...string) *Params {
	params.technologies = append(params.technologies, technologies...)
	return params
}

// GenericAudit audits all the projects found in the given workingDirs
func GenericAudit(params *Params) (results []services.ScanResponse, isMultipleRoot bool, err error) {
	if len(params.workingDirs) == 0 {
		log.Info("Auditing project: ")
		return doAudit(params)
	}

	return auditMultipleWorkingDirs(params)
}

func auditMultipleWorkingDirs(params *Params) (results []services.ScanResponse, isMultipleRoot bool, err error) {
	projectDir, err := os.Getwd()
	if errorutils.CheckError(err) != nil {
		return
	}
	defer func() {
		e := os.Chdir(projectDir)
		if err == nil {
			err = e
		}
	}()
	var errorList strings.Builder
	for _, wd := range params.workingDirs {
		absWd, e := filepath.Abs(wd)
		if e != nil {
			errorList.WriteString(fmt.Sprintf("the audit command couldn't find the following path: %s\n%s\n", wd, e.Error()))
			continue
		}
		log.Info("Auditing project:", absWd)
		e = os.Chdir(absWd)
		if e != nil {
			errorList.WriteString(fmt.Sprintf("the audit command couldn't change the current working directory to the following path: %s\n%s\n", absWd, e.Error()))
			continue
		}

		techResults, isMultipleRootProject, e := doAudit(params)
		if e != nil {
			errorList.WriteString(fmt.Sprintf("audit command in %s failed:\n%s\n", absWd, e.Error()))
			continue
		}

		results = append(results, techResults...)
		isMultipleRoot = isMultipleRootProject
	}

	if errorList.Len() > 0 {
		err = errors.New(errorList.String())
	}

	return
}

// Audits the project found in the current directory using Xray.
func doAudit(params *Params) (results []services.ScanResponse, isMultipleRoot bool, err error) {
	// If no technologies were given, try to detect all types of technologies used.
	// Otherwise, run audit for requested technologies only.
	if len(params.technologies) == 0 {
		params.technologies, err = detectedTechnologies()
		if err != nil {
			return
		}
	}
	var errorList strings.Builder
	for _, tech := range coreutils.ToTechnologies(params.technologies) {
		if tech == coreutils.Dotnet {
			continue
		}
		dependencyTrees, e := getTechDependencyTree(params, tech)
		if e != nil {
			errorList.WriteString(fmt.Sprintf("'%s' audit failed when building dependency tree:\n%s\n", tech, e.Error()))
			continue
		}
		techResults, e := audit.Audit(dependencyTrees, params.xrayGraphScanParams, params.serverDetails, params.progress, tech)
		if e != nil {
			errorList.WriteString(fmt.Sprintf("'%s' audit command failed:\n%s\n", tech, e.Error()))
			continue
		}
		results = append(results, techResults...)
		isMultipleRoot = len(dependencyTrees) > 1
	}
	if errorList.Len() > 0 {
		err = errors.New(errorList.String())
	}
	return
}

func getTechDependencyTree(params *Params, tech coreutils.Technology) (dependencyTrees []*services.GraphNode, e error) {
	if params.progress != nil {
		params.progress.SetHeadlineMsg(fmt.Sprintf("Calculating %v dependencies", tech.ToFormal()))
	}
	switch tech {
	case coreutils.Maven:
		dependencyTrees, e = java.BuildMvnDependencyTree(params.insecureTls, params.ignoreConfigFile)
	case coreutils.Gradle:
		dependencyTrees, e = java.BuildGradleDependencyTree(params.excludeTestDeps, params.useWrapper, params.ignoreConfigFile)
	case coreutils.Npm:
		dependencyTrees, e = npm.BuildDependencyTree(params.args)
	case coreutils.Yarn:
		dependencyTrees, e = yarn.BuildDependencyTree()
	case coreutils.Go:
		dependencyTrees, e = _go.BuildDependencyTree()
	case coreutils.Pipenv, coreutils.Pip, coreutils.Poetry:
		dependencyTrees, e = python.BuildDependencyTree(pythonutils.PythonTool(tech), params.requirementsFile)
	case coreutils.Nuget:
		dependencyTrees, e = nuget.BuildDependencyTree()
	default:
		e = errors.New(string(tech) + " is currently not supported")
	}

	return dependencyTrees, e
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
