package nuget

import (
	"errors"
	"fmt"
	bidotnet "github.com/jfrog/build-info-go/build/utils/dotnet"
	"github.com/jfrog/build-info-go/build/utils/dotnet/solution"
	"github.com/jfrog/build-info-go/entities"
	biutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/gofrog/datastructures"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/dotnet"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	nugetPackageTypeIdentifier = "nuget://"
	csprojFileName             = ".csproj"
	packageReferenceSyntax     = "PackageReference"
	packagesConfigFileName     = "packages.config"
	installCommandName         = "restore"
	dotnetToolType             = "dotnet"
	nugetToolType              = "nuget"
)

func BuildDependencyTree(params utils.AuditParams) (dependencyTree []*xrayUtils.GraphNode, uniqueDeps []string, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	sol, err := solution.Load(wd, "", log.Logger)
	if err != nil {
		return
	}

	if isInstallRequired(params, sol) {
		log.Info("Dependencies sources were not detected or 'install' command was provided. Running 'restore' command")
		sol, err = runDotnetRestoreAndLoadSolution(params, wd)
		if err != nil {
			return
		}
	}

	buildInfo, err := sol.BuildInfo("", log.Logger)
	if err != nil {
		return
	}
	dependencyTree, uniqueDeps = parseNugetDependencyTree(buildInfo)
	return
}

func isInstallRequired(params utils.AuditParams, sol solution.Solution) (installRequired bool) {
	// In case the user provided an 'install' command we execute 'restore' command even if the project is already installed
	// In case dependency sources were not detected when construction the Solution struct the project requires 'install' as well
	if len(params.InstallCommandArgs()) > 0 || !sol.DependenciesSourcesExist() {
		installRequired = true
	}
	return
}

func runDotnetRestoreAndLoadSolution(params utils.AuditParams, originalWd string) (sol solution.Solution, err error) {
	// Creating a temporary copy of the project in order to run 'install' command without effecting the original directory + creating the jfrog config for artifactory resolution
	tmpWd, err := fileutils.CreateTempDir()
	if err != nil {
		err = fmt.Errorf("failed creating temporary dir: %w", err)
		return
	}
	defer func() {
		err = errors.Join(err, fileutils.RemoveTempDir(tmpWd))
	}()

	err = biutils.CopyDir(originalWd, tmpWd, true, nil)
	if err != nil {
		err = fmt.Errorf("failed copying project to temp dir: %w", err)
		return
	}

	var installCommandArgs []string
	var toolType bidotnet.ToolchainType
	// Set up an Artifactory server as a resolution server if needed
	depsRepo := params.DepsRepo()
	if depsRepo != "" {
		var serverDetails *config.ServerDetails
		serverDetails, err = params.ServerDetails()
		if err != nil {
			err = fmt.Errorf("failed to get server details: %s", err.Error())
			return
		}

		log.Info("Resolving dependencies from", serverDetails.Url, "from repo", depsRepo)

		// Detect whether the project is a NuGet or .NET project
		var toolName string
		toolName, err = getProjectToolName(originalWd)
		if err != nil {
			err = fmt.Errorf("failed while checking for the porject's tool type: %s", err.Error())
			return
		}
		toolType = bidotnet.ConvertNameToToolType(toolName)

		var configFile *os.File
		configFile, err = dotnet.InitNewConfig(tmpWd, depsRepo, serverDetails, false)
		if err != nil {
			err = fmt.Errorf("failed to create a config file in order to set artifactory as a resolution server")
			return
		}
		installCommandArgs = append(installCommandArgs, toolType.GetTypeFlagPrefix()+"configfile", configFile.Name())
	}

	err = runDotnetRestore(tmpWd, params, toolType, installCommandArgs)
	if err != nil {
		return
	}
	sol, err = solution.Load(tmpWd, "", log.Logger)
	return
}

func getProjectToolName(wd string) (toolName string, err error) {
	// If <PackageReference> syntax is detected in the .csproj file - the tool type that is being used is .NET CLI
	csprojFilePath := filepath.Join(wd, csprojFileName)
	// TODO do we need to check if this file exists? of just use the error if it failed upon reading it?
	csprojExists, err := fileutils.IsFileExists(csprojFilePath, false)
	if err != nil {
		err = fmt.Errorf("failed while searching for '%s' file: %s", csprojFilePath, err.Error())
		return
	}
	if !csprojExists {
		err = errorutils.CheckErrorf(".csproj file wasn't fount at the project's root directory '%s'", wd)
		return
	}

	fileData, err := os.ReadFile(csprojFilePath)
	if err != nil {
		err = fmt.Errorf("failed to read file '%s': %s", csprojFilePath, err.Error())
		return
	}
	if strings.Contains(string(fileData), packageReferenceSyntax) {
		toolName = dotnetToolType
		return
	}

	// If packages.config file is found in the root dir - the tool type that is being used is NuGet CLI
	packagesConfigFilePath := filepath.Join(wd, packagesConfigFileName)
	packagesConfigExists, err := fileutils.IsFileExists(packagesConfigFilePath, false)
	if err != nil {
		err = fmt.Errorf("failed while searching for '%s' file: %s", packagesConfigFilePath, err.Error())
		return
	}
	if packagesConfigExists {
		toolName = nugetToolType
		return
	}

	err = errorutils.CheckErrorf("couldn't detect the project's tool type (.NET/NuGet CLI). Please execute 'restore' command.\nNotice: Some entry points enable provision of an 'install' command instead of running it yourself")
	return
}

func runDotnetRestore(wd string, params utils.AuditParams, toolType bidotnet.ToolchainType, commandExtraArgs []string) (err error) {
	// case 1: user command & artifactory
	// case 2: user command & no artifactory
	// case 3: default install & artifactory
	// case 4: default install & no artifactory

	// check if there is a user provided command. if so run it WITH the commandArgs
	// if no user command run the default command (check for nuget flags) WITH the commandArgs

	// TODO VERIFY INSTALL COMMAND FROM USER CONTAINS THE TOOL TYPE AS THE FIRST ARG
	var completeCommandArgs []string
	if len(params.InstallCommandArgs()) > 0 {
		// If the user has provided an 'install' command we run the command with the addition of artifactory server configuration (if exists)
		completeCommandArgs = params.InstallCommandArgs()
	} else {
		completeCommandArgs = append(completeCommandArgs, toolType.String(), installCommandName)
	}
	// Here we add the flag (if exists) that enables resolution from an Artifactory server
	completeCommandArgs = append(completeCommandArgs, commandExtraArgs...)
	command := exec.Command(completeCommandArgs[0], completeCommandArgs[1:]...)
	command.Dir = wd
	output, err := command.CombinedOutput()
	if err != nil {
		err = errorutils.CheckErrorf("'dotnet restore' command failed: %s - %s", err.Error(), output)
	}
	return
}

func parseNugetDependencyTree(buildInfo *entities.BuildInfo) (nodes []*xrayUtils.GraphNode, allUniqueDeps []string) {
	uniqueDepsSet := datastructures.MakeSet[string]()
	for _, module := range buildInfo.Modules {
		treeMap := make(map[string][]string)
		for _, dependency := range module.Dependencies {
			dependencyId := nugetPackageTypeIdentifier + dependency.Id
			parent := nugetPackageTypeIdentifier + dependency.RequestedBy[0][0]
			if children, ok := treeMap[parent]; ok {
				treeMap[parent] = append(children, dependencyId)
			} else {
				treeMap[parent] = []string{dependencyId}
			}
		}
		dependencyTree, uniqueDeps := sca.BuildXrayDependencyTree(treeMap, nugetPackageTypeIdentifier+module.Id)
		nodes = append(nodes, dependencyTree)
		for _, uniqueDep := range uniqueDeps {
			uniqueDepsSet.Add(uniqueDep)
		}
	}
	allUniqueDeps = uniqueDepsSet.ToSlice()
	return
}
