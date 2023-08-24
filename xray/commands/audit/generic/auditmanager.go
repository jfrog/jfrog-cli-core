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

type Results struct {
	IsMultipleRootProject bool
	ScaError              error
	JasError              error
	ExtendedScanResults   *xrayutils.ExtendedScanResults
}

func NewAuditResults() *Results {
	return &Results{ExtendedScanResults: &xrayutils.ExtendedScanResults{}}
}

// Runs an audit scan based on the provided auditParams.
// Returns an audit Results object containing all the scan results.
// If the current server is entitled for JAS, the advanced security results will be included in the scan results.
func RunAudit(auditParams *Params) (results *Results, err error) {
	// Initialize Results struct
	results = NewAuditResults()

	serverDetails, err := auditParams.ServerDetails()
	if err != nil {
		return
	}
	var xrayManager *xray.XrayServicesManager
	xrayManager, auditParams.xrayVersion, err = commandsutils.CreateXrayServiceManagerAndGetVersion(serverDetails)
	if err != nil {
		return
	}
	if err = coreutils.ValidateMinimumVersion(coreutils.Xray, auditParams.xrayVersion, commandsutils.GraphScanMinXrayVersion); err != nil {
		return
	}
	results.ExtendedScanResults.EntitledForJas, err = isEntitledForJas(xrayManager, auditParams.xrayVersion)
	if err != nil {
		return
	}

	errGroup := new(errgroup.Group)
	if results.ExtendedScanResults.EntitledForJas {
		// Download (if needed) the analyzer manager in a background routine.
		errGroup.Go(rtutils.DownloadAnalyzerManagerIfNeeded)
	}

	// The sca scan doesn't require the analyzer manager, so it can run separately from the analyzer manager download routine.
	results.ScaError = runScaScan(auditParams, results)

	// Wait for the Download of the AnalyzerManager to complete.
	if err = errGroup.Wait(); err != nil {
		return
	}

	// Run scanners only if the user is entitled for Advanced Security
	if results.ExtendedScanResults.EntitledForJas {
		results.JasError = jas.RunScannersAndSetResults(results.ExtendedScanResults, auditParams.FullDependenciesTree(), serverDetails, auditParams.workingDirs, auditParams.Progress())
	}
	return
}

func isEntitledForJas(xrayManager *xray.XrayServicesManager, xrayVersion string) (entitled bool, err error) {
	if e := coreutils.ValidateMinimumVersion(coreutils.Xray, xrayVersion, xrayutils.EntitlementsMinVersion); e != nil {
		log.Debug(e)
		return
	}
	entitled, err = xrayManager.IsEntitled(xrayutils.ApplicabilityFeatureId)
	return
}

func runScaScan(params *Params, results *Results) (err error) {
	rootDir, err := os.Getwd()
	if errorutils.CheckError(err) != nil {
		return
	}
	for _, wd := range params.workingDirs {
		if len(params.workingDirs) > 1 {
			log.Info("Running SCA scan for vulnerable dependencies scan in", wd, "directory...")
		} else {
			log.Info("Running SCA scan for vulnerable dependencies...")
		}
		wdScanErr := runScaScanOnWorkingDir(params, results, wd, rootDir)
		if wdScanErr != nil {
			err = errors.Join(err, fmt.Errorf("audit command in '%s' failed:\n%s\n", wd, wdScanErr.Error()))
			continue
		}
	}
	return
}

// Audits the project found in the current directory using Xray.
func runScaScanOnWorkingDir(params *Params, results *Results, workingDir, rootDir string) (err error) {
	err = os.Chdir(workingDir)
	if err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, os.Chdir(rootDir))
	}()

	var technologies []string
	requestedTechnologies := params.Technologies()
	if len(requestedTechnologies) != 0 {
		technologies = requestedTechnologies
	} else {
		technologies = commandsutils.DetectedTechnologies()
	}
	if len(technologies) == 0 {
		log.Info("Couldn't determine a package manager or build tool used by this project. Skipping the SCA scan...")
		return
	}
	serverDetails, err := params.ServerDetails()
	if err != nil {
		return
	}

	for _, tech := range coreutils.ToTechnologies(technologies) {
		if tech == coreutils.Dotnet {
			continue
		}
		flattenTree, techErr := GetTechDependencyTree(params.GraphBasicParams, tech)
		if techErr != nil {
			err = errors.Join(err, fmt.Errorf("failed while building '%s' dependency tree:\n%s\n", tech, techErr.Error()))
			continue
		}
		if len(flattenTree) == 0 {
			err = errors.Join(err, errors.New("no dependencies were found. Please try to build your project and re-run the audit command"))
			continue
		}

		scanGraphParams := commandsutils.NewScanGraphParams().
			SetServerDetails(serverDetails).
			SetXrayGraphScanParams(params.xrayGraphScanParams).
			SetXrayVersion(params.xrayVersion).
			SetFixableOnly(params.fixableOnly).
			SetSeverityLevel(params.minSeverityFilter)
		techResults, techErr := audit.RunXrayDependenciesTreeScanGraph(flattenTree, params.Progress(), tech, scanGraphParams)
		if techErr != nil {
			err = errors.Join(err, fmt.Errorf("'%s' Xray dependency tree scan request failed:\n%s\n", tech, techErr.Error()))
			continue
		}
		techResults = audit.BuildImpactPathsForScanResponse(techResults, params.FullDependenciesTree())
		results.ExtendedScanResults.XrayResults = append(results.ExtendedScanResults.XrayResults, techResults...)
		if !results.IsMultipleRootProject {
			results.IsMultipleRootProject = len(flattenTree) > 1
		}
		results.ExtendedScanResults.ScannedTechnologies = append(results.ExtendedScanResults.ScannedTechnologies, tech)
	}
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
