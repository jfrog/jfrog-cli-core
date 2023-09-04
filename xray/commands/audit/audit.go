package audit

import (
	"errors"
	"fmt"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/gofrog/datastructures"
	rtutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit"
	_go "github.com/jfrog/jfrog-cli-core/v2/xray/audit/go"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/jas"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/java"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/npm"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/nuget"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/python"
	"github.com/jfrog/jfrog-cli-core/v2/xray/audit/yarn"
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/xray/commands/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray"
	xrayCmdUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"golang.org/x/sync/errgroup"
	"os"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/xray/services"

	"encoding/json"
	xrayutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
)

type AuditCommand struct {
	watches                []string
	projectKey             string
	targetRepoPath         string
	IncludeVulnerabilities bool
	IncludeLicenses        bool
	Fail                   bool
	PrintExtendedTable     bool
	AuditParams
}

func NewGenericAuditCommand() *AuditCommand {
	return &AuditCommand{AuditParams: *NewAuditParams()}
}

func (auditCmd *AuditCommand) SetWatches(watches []string) *AuditCommand {
	auditCmd.watches = watches
	return auditCmd
}

func (auditCmd *AuditCommand) SetProject(project string) *AuditCommand {
	auditCmd.projectKey = project
	return auditCmd
}

func (auditCmd *AuditCommand) SetTargetRepoPath(repoPath string) *AuditCommand {
	auditCmd.targetRepoPath = repoPath
	return auditCmd
}

func (auditCmd *AuditCommand) SetIncludeVulnerabilities(include bool) *AuditCommand {
	auditCmd.IncludeVulnerabilities = include
	return auditCmd
}

func (auditCmd *AuditCommand) SetIncludeLicenses(include bool) *AuditCommand {
	auditCmd.IncludeLicenses = include
	return auditCmd
}

func (auditCmd *AuditCommand) SetFail(fail bool) *AuditCommand {
	auditCmd.Fail = fail
	return auditCmd
}

func (auditCmd *AuditCommand) SetPrintExtendedTable(printExtendedTable bool) *AuditCommand {
	auditCmd.PrintExtendedTable = printExtendedTable
	return auditCmd
}

func (auditCmd *AuditCommand) CreateXrayGraphScanParams() *services.XrayGraphScanParams {
	params := &services.XrayGraphScanParams{
		RepoPath: auditCmd.targetRepoPath,
		Watches:  auditCmd.watches,
		ScanType: services.Dependency,
	}
	if auditCmd.projectKey == "" {
		params.ProjectKey = os.Getenv(coreutils.Project)
	} else {
		params.ProjectKey = auditCmd.projectKey
	}
	params.IncludeVulnerabilities = auditCmd.IncludeVulnerabilities
	params.IncludeLicenses = auditCmd.IncludeLicenses
	return params
}

func (auditCmd *AuditCommand) Run() (err error) {
	workingDirs, err := xrayutils.GetFullPathsWorkingDirs(auditCmd.workingDirs)
	if err != nil {
		return
	}
	auditParams := NewAuditParams().
		SetXrayGraphScanParams(auditCmd.CreateXrayGraphScanParams()).
		SetWorkingDirs(workingDirs).
		SetMinSeverityFilter(auditCmd.minSeverityFilter).
		SetFixableOnly(auditCmd.fixableOnly).
		SetGraphBasicParams(auditCmd.GraphBasicParams)
	auditResults, err := RunAudit(auditParams)
	if err != nil {
		return
	}
	if auditCmd.Progress() != nil {
		if err = auditCmd.Progress().Quit(); err != nil {
			return
		}
	}
	var messages []string
	if !auditResults.ExtendedScanResults.EntitledForJas {
		messages = []string{coreutils.PrintTitle("The ‘jf audit’ command also supports JFrog Advanced Security features, such as 'Contextual Analysis', 'Secret Detection', 'IaC Scan' and ‘SAST’.\nThis feature isn't enabled on your system. Read more - ") + coreutils.PrintLink("https://jfrog.com/xray/")}
	}
	// Print Scan results on all cases except if errors accrued on SCA scan and no security/license issues found.
	printScanResults := !(auditResults.ScaError != nil && xrayutils.IsEmptyScanResponse(auditResults.ExtendedScanResults.XrayResults))
	if printScanResults {
		if err = xrayutils.NewResultsWriter(auditResults.ExtendedScanResults).
			SetIsMultipleRootProject(auditResults.IsMultipleRootProject).
			SetIncludeVulnerabilities(auditCmd.IncludeVulnerabilities).
			SetIncludeLicenses(auditCmd.IncludeLicenses).
			SetOutputFormat(auditCmd.OutputFormat()).
			SetPrintExtendedTable(auditCmd.PrintExtendedTable).
			SetExtraMessages(messages).
			SetScanType(false). //this is binary
			PrintScanResults(); err != nil {
			return
		}
	}
	if err = errors.Join(auditResults.ScaError, auditResults.JasError); err != nil {
		return
	}

	// Only in case Xray's context was given (!auditCmd.IncludeVulnerabilities), and the user asked to fail the build accordingly, do so.
	if auditCmd.Fail && !auditCmd.IncludeVulnerabilities && xrayutils.CheckIfFailBuild(auditResults.ExtendedScanResults.XrayResults) {
		err = xrayutils.NewFailBuildError()
	}
	return
}

func (auditCmd *AuditCommand) CommandName() string {
	return "generic_audit"
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
func RunAudit(auditParams *AuditParams) (results *Results, err error) {
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
	if err = clientutils.ValidateMinimumVersion(clientutils.Xray, auditParams.xrayVersion, commandsutils.GraphScanMinXrayVersion); err != nil {
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
		results.JasError = jas.RunScannersAndSetResults(results.ExtendedScanResults, auditParams.DirectDependencies(), serverDetails, auditParams.workingDirs, auditParams.Progress())
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

func runScaScan(params *AuditParams, results *Results) (err error) {
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
func runScaScanOnWorkingDir(params *AuditParams, results *Results, workingDir, rootDir string) (err error) {
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
		flattenTree, fullDependencyTrees, techErr := GetTechDependencyTree(params.GraphBasicParams, tech)
		if techErr != nil {
			err = errors.Join(err, fmt.Errorf("failed while building '%s' dependency tree:\n%s\n", tech, techErr.Error()))
			continue
		}
		if len(flattenTree.Nodes) == 0 {
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
		techResults = audit.BuildImpactPathsForScanResponse(techResults, fullDependencyTrees)
		var directDependencies []string
		if tech == coreutils.Pip {
			// When building pip dependency tree using pipdeptree, some of the direct dependencies are recognized as transitive and missed by the CA scanner.
			// Our solution for this case is to send all dependencies to the CA scanner.
			directDependencies = getDirectDependenciesFromTree([]*xrayCmdUtils.GraphNode{flattenTree})
		} else {
			directDependencies = getDirectDependenciesFromTree(fullDependencyTrees)
		}
		params.AppendDirectDependencies(directDependencies)

		results.ExtendedScanResults.XrayResults = append(results.ExtendedScanResults.XrayResults, techResults...)
		if !results.IsMultipleRootProject {
			results.IsMultipleRootProject = len(fullDependencyTrees) > 1
		}
		results.ExtendedScanResults.ScannedTechnologies = append(results.ExtendedScanResults.ScannedTechnologies, tech)
	}
	return
}

// This function retrieves the dependency trees of the scanned project and extracts a set that contains only the direct dependencies.
func getDirectDependenciesFromTree(dependencyTrees []*xrayCmdUtils.GraphNode) []string {
	directDependencies := datastructures.MakeSet[string]()
	for _, tree := range dependencyTrees {
		for _, node := range tree.Nodes {
			directDependencies.Add(node.Id)
		}
	}
	return directDependencies.ToSlice()
}

func GetTechDependencyTree(params *xrayutils.GraphBasicParams, tech coreutils.Technology) (flatTree *xrayCmdUtils.GraphNode, fullDependencyTrees []*xrayCmdUtils.GraphNode, err error) {
	if params.Progress() != nil {
		params.Progress().SetHeadlineMsg(fmt.Sprintf("Calculating %v dependencies", tech.ToFormal()))
	}
	serverDetails, err := params.ServerDetails()
	if err != nil {
		return
	}
	var uniqueDeps []string
	startTime := time.Now()
	switch tech {
	case coreutils.Maven, coreutils.Gradle:
		fullDependencyTrees, uniqueDeps, err = java.BuildDependencyTree(params, tech)
	case coreutils.Npm:
		fullDependencyTrees, uniqueDeps, err = npm.BuildDependencyTree(params.Args())
	case coreutils.Yarn:
		fullDependencyTrees, uniqueDeps, err = yarn.BuildDependencyTree()
	case coreutils.Go:
		fullDependencyTrees, uniqueDeps, err = _go.BuildDependencyTree(serverDetails, params.DepsRepo())
	case coreutils.Pipenv, coreutils.Pip, coreutils.Poetry:
		fullDependencyTrees, uniqueDeps, err = python.BuildDependencyTree(&python.AuditPython{
			Server:              serverDetails,
			Tool:                pythonutils.PythonTool(tech),
			RemotePypiRepo:      params.DepsRepo(),
			PipRequirementsFile: params.PipRequirementsFile()})
	case coreutils.Nuget:
		fullDependencyTrees, uniqueDeps, err = nuget.BuildDependencyTree()
	default:
		err = errorutils.CheckErrorf("%s is currently not supported", string(tech))
	}
	if err != nil {
		return
	}
	log.Debug(fmt.Sprintf("Created '%s' dependency tree with %d nodes. Elapsed time: %.1f seconds.", tech.ToFormal(), len(uniqueDeps), time.Since(startTime).Seconds()))
	flatTree, err = createFlatTree(uniqueDeps)
	return
}

func createFlatTree(uniqueDeps []string) (*xrayCmdUtils.GraphNode, error) {
	if log.GetLogger().GetLogLevel() == log.DEBUG {
		// Avoid printing and marshalling if not on DEBUG mode.
		jsonList, err := json.Marshal(uniqueDeps)
		if errorutils.CheckError(err) != nil {
			return nil, err
		}
		log.Debug("Unique dependencies list:\n" + clientutils.IndentJsonArray(jsonList))
	}
	uniqueNodes := []*xrayCmdUtils.GraphNode{}
	for _, uniqueDep := range uniqueDeps {
		uniqueNodes = append(uniqueNodes, &xrayCmdUtils.GraphNode{Id: uniqueDep})
	}
	return &xrayCmdUtils.GraphNode{Id: "root", Nodes: uniqueNodes}, nil
}
