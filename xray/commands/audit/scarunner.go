package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca"
	_go "github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca/go"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca/java"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca/npm"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca/nuget"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca/python"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca/yarn"
	"github.com/jfrog/jfrog-cli-core/v2/xray/scangraph"
	xrayutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/artifactory/services/fspatterns"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	xrayCmdUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
)

var defaultExcludePatterns = []string{"*node_modules*", "*target*", "*venv*", "*test*"}

func runScaScan(params *AuditParams, results *xrayutils.Results) (err error) {
	// Prepare
	currentWorkingDir, err := os.Getwd()
	if errorutils.CheckError(err) != nil {
		return
	}
	serverDetails, err := params.ServerDetails()
	if err != nil {
		return
	}
	scans := getScaScansToPreform(currentWorkingDir, params)
	if len(scans) == 0 {
		log.Info("Couldn't determine a package manager or build tool used by this project. Skipping the SCA scan...")
		return
	}
	printScansInformation(scans)

	defer func() {
		// Make sure to return to the original working directory, executeScaScan may change it
		err = errors.Join(err, os.Chdir(currentWorkingDir))
	}()
	for _, scan := range scans {
		// Run the scan
		log.Info("Running SCA scan for", scan.Technology, "vulnerable dependencies in", scan.WorkingDirectory, "directory...")
		if wdScanErr := executeScaScan(serverDetails, params, scan); wdScanErr != nil {
			err = errors.Join(err, fmt.Errorf("audit command in '%s' failed:\n%s", scan.WorkingDirectory, wdScanErr.Error()))
			continue
		}
		// Add the scan to the results
		results.ScaResults = append(results.ScaResults, *scan)
	}
	return
}

// Calculate the scans to preform
func getScaScansToPreform(currentWorkingDir string, params *AuditParams) (scansToPreform []*xrayutils.ScaScanResult) {
	recursive := len(currentWorkingDir) > 0
	for _, requestedDirectory := range getRequestedDirectoriesToScan(currentWorkingDir, params) {
		// Detect descriptors and technologies in the requested directory.
		techToWorkingDirs := coreutils.DetectTechnologiesDescriptors(requestedDirectory, recursive, params.Technologies(), getRequestedDescriptors(params), getExcludePattern(params, recursive))
		// Create scans to preform
		for tech, workingDirs := range techToWorkingDirs {
			if tech == coreutils.Dotnet {
				// We detect Dotnet and Nuget the same way, if one detected so does the other.
				// We don't need to scan for both and get duplicate results.
				continue
			}
			if len(workingDirs) == 0 {
				// Requested technology (from params) descriptors/indicators was not found, scan only requested directory for this technology.
				scansToPreform = append(scansToPreform, &xrayutils.ScaScanResult{WorkingDirectory: requestedDirectory, Technology: tech})
			}
			for workingDir, descriptors := range workingDirs {
				// Add scan for each detected working directory.
				scansToPreform = append(scansToPreform, &xrayutils.ScaScanResult{WorkingDirectory: workingDir, Technology: tech, Descriptors: descriptors})
			}
		}
	}
	return
}

func getRequestedDescriptors(params *AuditParams) map[coreutils.Technology][]string {
	requestedDescriptors := map[coreutils.Technology][]string{}
	if params.PipRequirementsFile() != "" {
		requestedDescriptors[coreutils.Pip] = []string{params.PipRequirementsFile()}
	}
	return requestedDescriptors
}

func getExcludePattern(params *AuditParams, recursive bool) string {
	exclusions := params.Exclusions()
	if len(exclusions) == 0 {
		exclusions = append(exclusions, defaultExcludePatterns...)
	}
	return fspatterns.PrepareExcludePathPattern(exclusions, clientutils.WildCardPattern, recursive)
}

func printScansInformation(scans []*xrayutils.ScaScanResult) {
	scansJson, err := json.MarshalIndent(scans, "", "  ")
	if err == nil {
		log.Info(fmt.Sprintf("Preforming %d SCA scans:\n%s", len(scans), string(scansJson)))
	}
}

func getRequestedDirectoriesToScan(currentWorkingDir string, params *AuditParams) []string {
	workingDirs := datastructures.MakeSet[string]()
	for _, wd := range params.workingDirs {
		workingDirs.Add(wd)
	}
	if workingDirs.Size() == 0 {
		workingDirs.Add(currentWorkingDir)
	}
	return workingDirs.ToSlice()
}

// Preform the SCA scan for the given scan information.
// This method will change the working directory to the scan's working directory.
func executeScaScan(serverDetails *config.ServerDetails, params *AuditParams, scan *xrayutils.ScaScanResult) (err error) {
	// Get the dependency tree for the technology in the working directory.
	if err = os.Chdir(scan.WorkingDirectory); err != nil {
		return
	}
	flattenTree, fullDependencyTrees, techErr := GetTechDependencyTree(params.AuditBasicParams, scan.Technology)
	if techErr != nil {
		return fmt.Errorf("failed while building '%s' dependency tree:\n%s", scan.Technology, techErr.Error())
	}
	if flattenTree == nil || len(flattenTree.Nodes) == 0 {
		return errors.New("no dependencies were found. Please try to build your project and re-run the audit command")
	}
	// Scan the dependency tree.
	scanResults, xrayErr := runScaWithTech(scan.Technology, params, serverDetails, flattenTree, fullDependencyTrees)
	if xrayErr != nil {
		return fmt.Errorf("'%s' Xray dependency tree scan request failed:\n%s", scan.Technology, xrayErr.Error())
	}
	scan.IsMultipleRootProject = clientutils.Pointer(len(fullDependencyTrees) > 1)
	addThirdPartyDependenciesToParams(params, scan.Technology, flattenTree, fullDependencyTrees)
	scan.XrayResults = append(scan.XrayResults, scanResults...)
	return
}

func runScaWithTech(tech coreutils.Technology, params *AuditParams, serverDetails *config.ServerDetails, flatTree *xrayCmdUtils.GraphNode, fullDependencyTrees []*xrayCmdUtils.GraphNode) (techResults []services.ScanResponse, err error) {
	scanGraphParams := scangraph.NewScanGraphParams().
		SetServerDetails(serverDetails).
		SetXrayGraphScanParams(params.xrayGraphScanParams).
		SetXrayVersion(params.xrayVersion).
		SetFixableOnly(params.fixableOnly).
		SetSeverityLevel(params.minSeverityFilter)
	techResults, err = sca.RunXrayDependenciesTreeScanGraph(flatTree, params.Progress(), tech, scanGraphParams)
	if err != nil {
		return
	}
	techResults = sca.BuildImpactPathsForScanResponse(techResults, fullDependencyTrees)
	return
}

func addThirdPartyDependenciesToParams(params *AuditParams, tech coreutils.Technology, flatTree *xrayCmdUtils.GraphNode, fullDependencyTrees []*xrayCmdUtils.GraphNode) {
	var dependenciesForApplicabilityScan []string
	if shouldUseAllDependencies(params.thirdPartyApplicabilityScan, tech) {
		dependenciesForApplicabilityScan = getDirectDependenciesFromTree([]*xrayCmdUtils.GraphNode{flatTree})
	} else {
		dependenciesForApplicabilityScan = getDirectDependenciesFromTree(fullDependencyTrees)
	}
	params.AppendDependenciesForApplicabilityScan(dependenciesForApplicabilityScan)
}

// When building pip dependency tree using pipdeptree, some of the direct dependencies are recognized as transitive and missed by the CA scanner.
// Our solution for this case is to send all dependencies to the CA scanner.
// When thirdPartyApplicabilityScan is true, use flatten graph to include all the dependencies in applicability scanning.
// Only npm is supported for this flag.
func shouldUseAllDependencies(thirdPartyApplicabilityScan bool, tech coreutils.Technology) bool {
	return tech == coreutils.Pip || (thirdPartyApplicabilityScan && tech == coreutils.Npm)
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

func GetTechDependencyTree(params xrayutils.AuditParams, tech coreutils.Technology) (flatTree *xrayCmdUtils.GraphNode, fullDependencyTrees []*xrayCmdUtils.GraphNode, err error) {
	logMessage := fmt.Sprintf("Calculating %s dependencies", tech.ToFormal())
	log.Info(logMessage + "...")
	if params.Progress() != nil {
		params.Progress().SetHeadlineMsg(logMessage)
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
		fullDependencyTrees, uniqueDeps, err = npm.BuildDependencyTree(params)
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
	if err != nil || len(uniqueDeps) == 0 {
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
