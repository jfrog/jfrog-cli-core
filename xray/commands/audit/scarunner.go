package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/gofrog/datastructures"
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
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	xrayCmdUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"os"
	"time"
)

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
		technologies = coreutils.DetectedTechnologiesList()
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
		flattenTree, fullDependencyTrees, techErr := GetTechDependencyTree(params.AuditBasicParams, tech)
		if techErr != nil {
			err = errors.Join(err, fmt.Errorf("failed while building '%s' dependency tree:\n%s\n", tech, techErr.Error()))
			continue
		}
		if flattenTree == nil || len(flattenTree.Nodes) == 0 {
			err = errors.Join(err, errors.New("no dependencies were found. Please try to build your project and re-run the audit command"))
			continue
		}

		scanGraphParams := scangraph.NewScanGraphParams().
			SetServerDetails(serverDetails).
			SetXrayGraphScanParams(params.xrayGraphScanParams).
			SetXrayVersion(params.xrayVersion).
			SetFixableOnly(params.fixableOnly).
			SetSeverityLevel(params.minSeverityFilter)
		techResults, techErr := sca.RunXrayDependenciesTreeScanGraph(flattenTree, params.Progress(), tech, scanGraphParams)
		if techErr != nil {
			err = errors.Join(err, fmt.Errorf("'%s' Xray dependency tree scan request failed:\n%s\n", tech, techErr.Error()))
			continue
		}
		techResults = sca.BuildImpactPathsForScanResponse(techResults, fullDependencyTrees)
		var directDependencies []string
		if tech == coreutils.Pip || params.thirdPartyApplicabilityScan {
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

func GetTechDependencyTree(params *xrayutils.AuditBasicParams, tech coreutils.Technology) (flatTree *xrayCmdUtils.GraphNode, fullDependencyTrees []*xrayCmdUtils.GraphNode, err error) {
	logMessage := fmt.Sprintf("Calculating %s dependencies", tech.ToFormal())
	log.Info(logMessage)
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
