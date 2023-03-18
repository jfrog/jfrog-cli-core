package audit

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/java"
	"github.com/jfrog/jfrog-client-go/auth"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"

	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	_go "github.com/jfrog/jfrog-cli-core/v2/xray/audit/go"
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
	depsRepo            string
	requirementsFile    string
	technologies        []string
	workingDirs         []string
	args                []string
	installFunc         func(tech string) error
}

func NewAuditParams() *Params {
	return &Params{}
}

func (params *Params) InstallFunc() func(tech string) error {
	return params.installFunc
}

func (params *Params) XrayGraphScanParams() services.XrayGraphScanParams {
	return params.xrayGraphScanParams
}

func (params *Params) ServerDetails() *config.ServerDetails {
	return params.serverDetails
}

func (params *Params) Progress() ioUtils.ProgressMgr {
	return params.progress
}

func (params *Params) IgnoreConfigFile() bool {
	return params.ignoreConfigFile
}

func (params *Params) ExcludeTestDeps() bool {
	return params.excludeTestDeps
}

func (params *Params) InsecureTls() bool {
	return params.insecureTls
}

func (params *Params) UseWrapper() bool {
	return params.useWrapper
}

func (params *Params) DepsRepo() string {
	return params.depsRepo
}

func (params *Params) RequirementsFile() string {
	return params.requirementsFile
}

func (params *Params) Technologies() []string {
	return params.technologies
}

func (params *Params) WorkingDirs() []string {
	return params.workingDirs
}

func (params *Params) Args() []string {
	return params.args
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

func (params *Params) SetDepsRepo(depsRepo string) *Params {
	params.depsRepo = depsRepo
	return params
}

func (params *Params) SetInstallFunc(installFunc func(tech string) error) *Params {
	params.installFunc = installFunc
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
		err = errorutils.CheckError(errors.New(errorList.String()))
	}

	return
}

// Audits the project found in the current directory using Xray.
func doAudit(params *Params) (results []services.ScanResponse, isMultipleRoot bool, err error) {
	// If no technologies were given, try to detect all types of technologies used.
	// Otherwise, run audit for requested technologies only.
	technologies := params.technologies
	if len(technologies) == 0 {
		technologies, err = detectedTechnologies()
		if err != nil {
			return
		}
	}
	var errorList strings.Builder
	for _, tech := range coreutils.ToTechnologies(technologies) {
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
	case coreutils.Maven, coreutils.Gradle:
		dependencyTrees, e = getJavaDependencyTree(params, tech)
	case coreutils.Npm:
		dependencyTrees, e = npm.BuildDependencyTree(params.args)
	case coreutils.Yarn:
		dependencyTrees, e = yarn.BuildDependencyTree()
	case coreutils.Go:
		dependencyTrees, e = _go.BuildDependencyTree(params.serverDetails, params.depsRepo)
	case coreutils.Pipenv, coreutils.Pip, coreutils.Poetry:
		dependencyTrees, e = python.BuildDependencyTree(&python.AuditPython{
			Server:              params.serverDetails,
			Tool:                pythonutils.PythonTool(tech),
			RemotePypiRepo:      params.depsRepo,
			PipRequirementsFile: params.requirementsFile})
	case coreutils.Nuget:
		dependencyTrees, e = nuget.BuildDependencyTree()
	default:
		e = errorutils.CheckError(fmt.Errorf("%s is currently not supported", string(tech)))
	}

	return dependencyTrees, e
}

func getJavaDependencyTree(params *Params, tech coreutils.Technology) ([]*services.GraphNode, error) {
	var javaProps map[string]any
	if params.DepsRepo() != "" {
		javaProps = createJavaProps(params.DepsRepo(), params.ServerDetails())
	}
	return java.BuildDependencyTree(&java.DependencyTreeParams{
		Tool:             tech,
		InsecureTls:      params.insecureTls,
		IgnoreConfigFile: params.ignoreConfigFile,
		ExcludeTestDeps:  params.excludeTestDeps,
		UseWrapper:       params.useWrapper,
		JavaProps:        javaProps,
	})
}

func createJavaProps(depsRepo string, serverDetails *config.ServerDetails) map[string]any {
	authPass := serverDetails.Password
	if serverDetails.AccessToken != "" {
		authPass = serverDetails.AccessToken
	}
	authUser := serverDetails.User
	if authUser == "" {
		authUser = auth.ExtractUsernameFromAccessToken(serverDetails.AccessToken)
	}
	return map[string]any{
		"resolver.username":     authUser,
		"resolver.password":     authPass,
		"resolver.url":          serverDetails.ArtifactoryUrl,
		"resolver.releaseRepo":  depsRepo,
		"resolver.repo":         depsRepo,
		"resolver.snapshotRepo": depsRepo,
		"buildInfoConfig.artifactoryResolutionEnabled": true,
	}
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
