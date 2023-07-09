package audit

import (
	"errors"
	"fmt"
	"github.com/jfrog/gofrog/version"
	rtutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/jas"
	"golang.org/x/sync/errgroup"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	_go "github.com/jfrog/jfrog-cli-core/v2/xray/audit/go"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/java"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/npm"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/nuget"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/python"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/yarn"
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/xray/commands/utils"
	clientUtils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	xrayCmdUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
)

type Params struct {
	xrayGraphScanParams *services.XrayGraphScanParams
	workingDirs         []string
	installFunc         func(tech string) error
	fixableOnly         bool
	minSeverityFilter   string
	*clientUtils.GraphBasicParams
	xrayVersion string
}

func NewAuditParams() *Params {
	return &Params{
		xrayGraphScanParams: &services.XrayGraphScanParams{},
		GraphBasicParams:    &clientUtils.GraphBasicParams{},
	}
}

func (params *Params) InstallFunc() func(tech string) error {
	return params.installFunc
}

func (params *Params) XrayGraphScanParams() *services.XrayGraphScanParams {
	return params.xrayGraphScanParams
}

func (params *Params) WorkingDirs() []string {
	return params.workingDirs
}

func (params *Params) XrayVersion() string {
	return params.xrayVersion
}

func (params *Params) SetXrayGraphScanParams(xrayGraphScanParams *services.XrayGraphScanParams) *Params {
	params.xrayGraphScanParams = xrayGraphScanParams
	return params
}

func (params *Params) SetGraphBasicParams(gbp *clientUtils.GraphBasicParams) *Params {
	params.GraphBasicParams = gbp
	return params
}

func (params *Params) SetWorkingDirs(workingDirs []string) *Params {
	params.workingDirs = workingDirs
	return params
}

func (params *Params) SetInstallFunc(installFunc func(tech string) error) *Params {
	params.installFunc = installFunc
	return params
}

func (params *Params) FixableOnly() bool {
	return params.fixableOnly
}

func (params *Params) SetFixableOnly(fixable bool) *Params {
	params.fixableOnly = fixable
	return params
}

func (params *Params) MinSeverityFilter() string {
	return params.minSeverityFilter
}

func (params *Params) SetMinSeverityFilter(minSeverityFilter string) *Params {
	params.minSeverityFilter = minSeverityFilter
	return params
}

func (params *Params) SetXrayVersion(version string) *Params {
	params.xrayVersion = version
	return params
}

// Runs an audit scan based on the provided auditParams.
// Returns an audit Results object containing all the scan results.
// If the current server is entitled for JAS, the advanced security results will be included in the scan results.
func RunAudit(auditParams *Params) (results *Results, err error) {
	serverDetails, err := auditParams.ServerDetails()
	if err != nil {
		return
	}
	isEntitled, xrayVersion, err := isEntitledForJas(serverDetails)
	if err != nil {
		return
	}
	auditParams.SetXrayVersion(xrayVersion)

	errGroup := new(errgroup.Group)
	if isEntitled {
		// Download (if needed) the analyzer manager in a background routine.
		errGroup.Go(rtutils.DownloadAnalyzerManagerIfNeeded)
	}

	// The audit scan doesn't require the analyzer manager, so it can run separately from the analyzer manager download routine.
	scanResults, isMultipleRootProject, auditError := genericAudit(auditParams)

	// Wait for the Download of the AnalyzerManager to complete.
	if err = errGroup.Wait(); err != nil {
		return
	}

	extendedScanResults := &clientUtils.ExtendedScanResults{XrayResults: scanResults}
	// Try to run contextual analysis only if the user is entitled for advance security
	if isEntitled {
		extendedScanResults, err = jas.GetExtendedScanResults(scanResults, auditParams.FullDependenciesTree(), serverDetails)
		if err != nil {
			return
		}
	}
	results = &Results{
		IsMultipleRootProject: isMultipleRootProject,
		AuditError:            auditError,
		ExtendedScanResults:   extendedScanResults,
	}
	return
}

func isEntitledForJas(serverDetails *config.ServerDetails) (entitled bool, xrayVersion string, err error) {
	xrayManager, xrayVersion, err := commandsutils.CreateXrayServiceManagerAndGetVersion(serverDetails)
	if err != nil {
		return
	}
	if !version.NewVersion(xrayVersion).AtLeast(clientUtils.EntitlementsMinVersion) {
		log.Debug("Entitlements check for ‘Advanced Security’ package failed:")
		log.Debug(coreutils.MinimumVersionMsg, coreutils.Xray, xrayVersion, clientUtils.EntitlementsMinVersion)
		return
	}
	entitled, err = xrayManager.IsEntitled(clientUtils.ApplicabilityFeatureId)
	return
}

// genericAudit audits all the projects found in the given workingDirs
func genericAudit(params *Params) (results []services.ScanResponse, isMultipleRoot bool, err error) {
	if err = coreutils.ValidateMinimumVersion(coreutils.Xray, params.xrayVersion, commandsutils.GraphScanMinXrayVersion); err != nil {
		return
	}
	log.Info("JFrog Xray version is:", params.xrayVersion)

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
		err = errors.Join(err, os.Chdir(projectDir))
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
	technologies := params.Technologies()
	if len(technologies) == 0 {
		technologies, err = commandsutils.DetectedTechnologies()
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
		flattenTree, e := GetTechDependencyTree(params.GraphBasicParams, tech)
		if e != nil {
			errorList.WriteString(fmt.Sprintf("audit failed while building %s dependency tree:\n%s\n", tech, e.Error()))
			continue
		}

		scanGraphParams := commandsutils.NewScanGraphParams().
			SetServerDetails(serverDetails).
			SetXrayGraphScanParams(params.xrayGraphScanParams).
			SetXrayVersion(params.xrayVersion).
			SetFixableOnly(params.fixableOnly).
			SetSeverityLevel(params.minSeverityFilter)
		techResults, e := audit.Audit(flattenTree, params.Progress(), tech, scanGraphParams)
		if e != nil {
			errorList.WriteString(fmt.Sprintf("'%s' audit request failed:\n%s\n", tech, e.Error()))
			continue
		}
		techResults = audit.BuildImpactPathsForScanResponse(techResults, params.FullDependenciesTree())
		results = append(results, techResults...)
		isMultipleRoot = len(flattenTree) > 1
	}
	if errorList.Len() > 0 {
		err = errorutils.CheckErrorf(errorList.String())
	}
	return
}

func GetTechDependencyTree(params *clientUtils.GraphBasicParams, tech coreutils.Technology) (flatTree []*xrayCmdUtils.GraphNode, err error) {
	if params.Progress() != nil {
		params.Progress().SetHeadlineMsg(fmt.Sprintf("Calculating %v dependencies", tech.ToFormal()))
	}
	serverDetails, err := params.ServerDetails()
	if err != nil {
		return
	}
	var dependencyTrees []*xrayCmdUtils.GraphNode
	switch tech {
	case coreutils.Maven, coreutils.Gradle:
		dependencyTrees, err = getJavaDependencyTree(params, tech)
	case coreutils.Npm:
		dependencyTrees, err = npm.BuildDependencyTree(params.Args())
	case coreutils.Yarn:
		dependencyTrees, err = yarn.BuildDependencyTree()
	case coreutils.Go:
		dependencyTrees, err = _go.BuildDependencyTree(serverDetails, params.DepsRepo())
	case coreutils.Pipenv, coreutils.Pip, coreutils.Poetry:
		dependencyTrees, err = python.BuildDependencyTree(&python.AuditPython{
			Server:              serverDetails,
			Tool:                pythonutils.PythonTool(tech),
			RemotePypiRepo:      params.DepsRepo(),
			PipRequirementsFile: params.PipRequirementsFile()})
	case coreutils.Nuget:
		dependencyTrees, err = nuget.BuildDependencyTree()
	default:
		err = errorutils.CheckErrorf("%s is currently not supported", string(tech))
	}
	if err != nil {
		return nil, err
	}
	// Save the full dependencyTree to build impact paths for vulnerable dependencies
	params.SetFullDependenciesTree(dependencyTrees)

	// Flatten the graph to speed up the ScanGraph request
	return services.FlattenGraph(dependencyTrees)
}

func getJavaDependencyTree(params *clientUtils.GraphBasicParams, tech coreutils.Technology) ([]*xrayCmdUtils.GraphNode, error) {
	var javaProps map[string]any
	serverDetails, err := params.ServerDetails()
	if err != nil {
		return nil, err
	}
	if params.DepsRepo() != "" && tech == coreutils.Maven {
		javaProps = CreateJavaProps(params.DepsRepo(), serverDetails)
	}
	return java.BuildDependencyTree(&java.DependencyTreeParams{
		Tool:             tech,
		InsecureTls:      params.InsecureTls(),
		IgnoreConfigFile: params.IgnoreConfigFile(),
		ExcludeTestDeps:  params.ExcludeTestDependencies(),
		UseWrapper:       params.UseWrapper(),
		JavaProps:        javaProps,
		Server:           serverDetails,
		DepsRepo:         params.DepsRepo(),
		ReleasesRepo:     params.ReleasesRepo(),
	})
}

func CreateJavaProps(depsRepo string, serverDetails *config.ServerDetails) map[string]any {
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
