package audit

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/java"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/utils"
	cmdUtils "github.com/jfrog/jfrog-cli-core/v2/xray/commands/utils"
	"github.com/jfrog/jfrog-client-go/auth"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"

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
	progress            ioUtils.ProgressMgr
	dependencyTrees     []*xrayUtils.GraphNode
	releasesRepo        string
	workingDirs         []string
	installFunc         func(tech string) error
	*cmdUtils.GraphBasicParams
	xrayVersion string
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

func (params *Params) Progress() ioUtils.ProgressMgr {
	return params.progress
}

func (params *Params) WorkingDirs() []string {
	return params.workingDirs
}

func (params *Params) XrayVersion() string {
	return params.xrayVersion
}

func (params *Params) SetXrayGraphScanParams(xrayGraphScanParams services.XrayGraphScanParams) *Params {
	params.xrayGraphScanParams = xrayGraphScanParams
	return params
}

func (params *Params) SetProgressBar(progress ioUtils.ProgressMgr) *Params {
	params.progress = progress
	return params
}

func (params *Params) SetWorkingDirs(workingDirs []string) *Params {
	params.workingDirs = workingDirs
	return params
}

func (params *Params) SetReleasesRepo(releasesRepo string) *Params {
	params.releasesRepo = releasesRepo
	return params
}

func (params *Params) SetInstallFunc(installFunc func(tech string) error) *Params {
	params.installFunc = installFunc
	return params
}

// GenericAudit audits all the projects found in the given workingDirs
func GenericAudit(params *Params) (results []services.ScanResponse, isMultipleRoot bool, err error) {
	// Get Xray version
	serverDetails, err := params.ServerDetails()
	if err != nil {
		return
	}
	_, xrayVersion, err := utils.CreateXrayServiceManagerAndGetVersion(serverDetails)
	if err != nil {
		return
	}
	if err = coreutils.ValidateMinimumVersion(coreutils.Xray, xrayVersion, utils.GraphScanMinXrayVersion); err != nil {
		return
	}
	params.xrayVersion = xrayVersion
	log.Info("JFrog Xray version is:", xrayVersion)

	if len(params.workingDirs) == 0 {
		log.Info("Auditing project...")
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
		log.Info("Auditing project:", absWd, "...")
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
		err = errorutils.CheckErrorf(errorList.String())
	}

	return
}

// Audits the project found in the current directory using Xray.
func doAudit(params *Params) (results []services.ScanResponse, isMultipleRoot bool, err error) {
	// If no technologies were given, try to detect all types of technologies used.
	// Otherwise, run audit for requested technologies only.
	technologies := params.Technologies
	if len(technologies) == 0 {
		technologies, err = utils.DetectedTechnologies()
		if err != nil {
			return
		}
	}
	var errorList strings.Builder
	serverDetails, err := params.ServerDetails()
	if err != nil {
		return
	}
	for _, tech := range coreutils.ToTechnologies(technologies) {
		if tech == coreutils.Dotnet {
			continue
		}
		dependencyTrees, e := GetTechDependencyTree(params.GraphBasicParams, tech)
		if e != nil {
			errorList.WriteString(fmt.Sprintf("audit failed while building %s dependency tree:\n%s\n", tech, e.Error()))
			continue
		}
		techResults, e := audit.Audit(dependencyTrees, params.xrayGraphScanParams, serverDetails, params.progress, tech, params.xrayVersion)
		if e != nil {
			errorList.WriteString(fmt.Sprintf("'%s' audit request failed:\n%s\n", tech, e.Error()))
			continue
		}
		techResults = audit.BuildImpactPathsForScanResponse(techResults, params.dependencyTrees)
		results = append(results, techResults...)
		isMultipleRoot = len(dependencyTrees) > 1
	}
	if errorList.Len() > 0 {
		err = errorutils.CheckErrorf(errorList.String())
	}
	return
}

func GetTechDependencyTree(params *cmdUtils.GraphBasicParams, tech coreutils.Technology) (flatTree []*xrayUtils.GraphNode, err error) {
	if params.Progress != nil {
		params.Progress.SetHeadlineMsg(fmt.Sprintf("Calculating %v dependencies", tech.ToFormal()))
	}
	var dependencyTrees []*xrayUtils.GraphNode
	switch tech {
	case coreutils.Maven, coreutils.Gradle:
		dependencyTrees, err = getJavaDependencyTree(params, tech)
	case coreutils.Npm:
		dependencyTrees, err = npm.BuildDependencyTree(params.Args)
	case coreutils.Yarn:
		dependencyTrees, err = yarn.BuildDependencyTree()
	case coreutils.Go:
		serverDetails, err := params.ServerDetails()
		if err != nil {
			return nil, err
		}
		dependencyTrees, err = _go.BuildDependencyTree(serverDetails, params.DepsRepo())
	case coreutils.Pipenv, coreutils.Pip, coreutils.Poetry:
		serverDetails, e := params.ServerDetails()
		if e != nil {
			return nil, e
		}
		dependencyTrees, err = python.BuildDependencyTree(&python.AuditPython{
			Server:              serverDetails,
			Tool:                pythonutils.PythonTool(tech),
			RemotePypiRepo:      params.DepsRepo(),
			PipRequirementsFile: params.RequirementsFile})
	case coreutils.Nuget:
		dependencyTrees, err = nuget.BuildDependencyTree()
	default:
		err = errorutils.CheckErrorf("%s is currently not supported", string(tech))
	}
	if err != nil {
		return nil, err
	}
	// Save the full dependencyTree to build impact paths for vulnerable dependencies
	params.DependencyTrees = dependencyTrees

	// Flatten the graph to speed up the ScanGraph request
	return services.FlattenGraph(dependencyTrees)
}

func getJavaDependencyTree(params *cmdUtils.GraphBasicParams, tech coreutils.Technology) ([]*xrayUtils.GraphNode, error) {
	var javaProps map[string]any
	serverDetails, err := params.ServerDetails()
	if err != nil {
		return nil, err
	}
	if params.DepsRepo() != "" && tech == coreutils.Maven {
		javaProps = createJavaProps(params.DepsRepo(), serverDetails)
	}
	return java.BuildDependencyTree(&java.DependencyTreeParams{
		Tool:             tech,
		InsecureTls:      params.InsecureTls,
		IgnoreConfigFile: params.IgnoreConfigFile,
		ExcludeTestDeps:  params.ExcludeTestDependencies,
		UseWrapper:       params.UseWrapper,
		JavaProps:        javaProps,
		Server:           serverDetails,
		DepsRepo:         params.DepsRepo(),
		ReleasesRepo:     params.ReleasesRepo,
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
