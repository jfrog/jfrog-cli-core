package audit

import (
	"errors"
	"fmt"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	rtutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	_go "github.com/jfrog/jfrog-cli-core/v2/xray/audit/go"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/jas"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/java"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/npm"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/nuget"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/python"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/yarn"
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/xray/commands/utils"
	xrayutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray"
	"github.com/jfrog/jfrog-client-go/xray/services"
	xrayCmdUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"golang.org/x/sync/errgroup"
	"os"
)

type Params struct {
	xrayGraphScanParams *services.XrayGraphScanParams
	workingDirs         []string
	installFunc         func(tech string) error
	fixableOnly         bool
	minSeverityFilter   string
	*xrayutils.GraphBasicParams
	xrayVersion string
}

func NewAuditParams() *Params {
	return &Params{
		xrayGraphScanParams: &services.XrayGraphScanParams{},
		GraphBasicParams:    &xrayutils.GraphBasicParams{},
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

func (params *Params) SetGraphBasicParams(gbp *xrayutils.GraphBasicParams) *Params {
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

type Results struct {
	IsMultipleRootProject bool
	AuditError            error
	ExtendedScanResults   *xrayutils.ExtendedScanResults
}

func NewAuditResults() *Results {
	return &Results{ExtendedScanResults: &xrayutils.ExtendedScanResults{}}
}

func (r *Results) SetAuditError(err error) *Results {
	r.AuditError = err
	return r
}

// Runs an audit scan based on the provided auditParams.
// Returns an audit Results object containing all the scan results.
// If the current server is entitled for JAS, the advanced security results will be included in the scan results.
func RunAudit(auditParams *Params) (results *Results, err error) {
	serverDetails, err := auditParams.ServerDetails()
	if err != nil {
		return
	}
	xrayManager, xrayVersion, err := commandsutils.CreateXrayServiceManagerAndGetVersion(serverDetails)
	if err != nil {
		return
	}
	if err = clientutils.ValidateMinimumVersion(clientutils.Xray, xrayVersion, commandsutils.GraphScanMinXrayVersion); err != nil {
		return
	}
	isEntitled, err := isEntitledForJas(xrayManager, xrayVersion)
	if err != nil {
		return
	}

	auditParams.SetXrayVersion(xrayVersion)

	errGroup := new(errgroup.Group)
	if isEntitled {
		// Download (if needed) the analyzer manager in a background routine.
		errGroup.Go(rtutils.DownloadAnalyzerManagerIfNeeded)
	}
	// The sca scan doesn't require the analyzer manager, so it can run separately from the analyzer manager download routine.
	results = runScaScan(auditParams)

	// Wait for the Download of the AnalyzerManager to complete.
	if err = errGroup.Wait(); err != nil {
		return
	}

	// Run scanners only if the user is entitled for Advanced Security
	if isEntitled {
		results.ExtendedScanResults.EntitledForJas = true
		err = jas.RunScannersAndSetResults(results.ExtendedScanResults, auditParams.FullDependenciesTree(), serverDetails, auditParams.workingDirs, auditParams.Progress())
	}
	return
}

func isEntitledForJas(xrayManager *xray.XrayServicesManager, xrayVersion string) (entitled bool, err error) {
	if e := clientutils.ValidateMinimumVersion(clientutils.Xray, xrayVersion, xrayutils.EntitlementsMinVersion); e != nil {
		log.Debug(e)
		return
	}
	entitled, err = xrayManager.IsEntitled(xrayutils.ApplicabilityFeatureId)
	return
}

func runScaScan(params *Params) (results *Results) {
	results = NewAuditResults()
	rootDir, err := os.Getwd()
	if errorutils.CheckError(err) != nil {
		return results.SetAuditError(err)
	}
	for _, wd := range params.workingDirs {
		if len(params.workingDirs) > 1 {
			log.Info("Running SCA scan for vulnerable dependencies scan in", wd, "directory...")
		} else {
			log.Info("Running SCA scan vulnerable dependencies...")
		}
		scaResults := runScaScanOnWorkingDir(params, wd, rootDir)
		if scaResults.AuditError != nil {
			err = errors.Join(err, fmt.Errorf("audit command in '%s' failed:\n%s\n", wd, scaResults.AuditError.Error()))
			continue
		}
		results.ExtendedScanResults.XrayResults =
			append(results.ExtendedScanResults.XrayResults, scaResults.ExtendedScanResults.XrayResults...)
		if !results.IsMultipleRootProject {
			results.IsMultipleRootProject = scaResults.IsMultipleRootProject
		}
		results.ExtendedScanResults.ScannedTechnologies = append(results.ExtendedScanResults.ScannedTechnologies, scaResults.ExtendedScanResults.ScannedTechnologies...)
	}
	results.SetAuditError(err)
	return
}

// Audits the project found in the current directory using Xray.
func runScaScanOnWorkingDir(params *Params, workingDir, rootDir string) (results *Results) {
	results = NewAuditResults()
	err := os.Chdir(workingDir)
	if err != nil {
		results.SetAuditError(err)
		return
	}
	defer func() {
		results.SetAuditError(errors.Join(results.AuditError, os.Chdir(rootDir)))
	}()

	// If no technologies were given, try to detect all types of technologies used.
	// Otherwise, run audit for requested technologies only.
	technologies := params.Technologies()
	if len(technologies) == 0 {
		technologies = commandsutils.DetectedTechnologies()
		if len(technologies) == 0 {
			log.Info("Skipping vulnerable dependencies scanning...")
			return
		}
	}
	serverDetails, err := params.ServerDetails()
	if err != nil {
		results.SetAuditError(err)
		return
	}

	for _, tech := range coreutils.ToTechnologies(technologies) {
		if tech == coreutils.Dotnet {
			continue
		}
		flattenTree, e := GetTechDependencyTree(params.GraphBasicParams, tech)
		if e != nil {
			err = errors.Join(err, fmt.Errorf("audit failed while building %s dependency tree:\n%s\n", tech, e.Error()))
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
			err = errors.Join(err, fmt.Errorf("'%s' audit request failed:\n%s\n", tech, e.Error()))
			continue
		}
		techResults = audit.BuildImpactPathsForScanResponse(techResults, params.FullDependenciesTree())
		results.ExtendedScanResults.XrayResults = append(results.ExtendedScanResults.XrayResults, techResults...)
		if !results.IsMultipleRootProject {
			results.IsMultipleRootProject = len(flattenTree) > 1
		}
		results.ExtendedScanResults.ScannedTechnologies = append(results.ExtendedScanResults.ScannedTechnologies, tech)
	}
	results.SetAuditError(err)
	return
}

func GetTechDependencyTree(params *xrayutils.GraphBasicParams, tech coreutils.Technology) (flatTree []*xrayCmdUtils.GraphNode, err error) {
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

func getJavaDependencyTree(params *xrayutils.GraphBasicParams, tech coreutils.Technology) ([]*xrayCmdUtils.GraphNode, error) {
	serverDetails, err := params.ServerDetails()
	if err != nil {
		return nil, err
	}
	return java.BuildDependencyTree(&java.DependencyTreeParams{
		Tool:             tech,
		InsecureTls:      params.InsecureTls(),
		IgnoreConfigFile: params.IgnoreConfigFile(),
		ExcludeTestDeps:  params.ExcludeTestDependencies(),
		UseWrapper:       params.UseWrapper(),
		Server:           serverDetails,
		DepsRepo:         params.DepsRepo(),
	})
}
