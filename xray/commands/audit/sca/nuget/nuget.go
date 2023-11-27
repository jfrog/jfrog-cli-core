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
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	nugetPackageTypeIdentifier = "nuget://"
	csprojFileSuffix           = ".csproj"
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
		// If we get an error that global package path couldn't be found we want to continue because will be fixed after restoring the project
		if !strings.Contains(err.Error(), "could not find global packages path at:") {
			return
		}
	}
	err = nil

	if isInstallRequired(params, sol) {
		log.Info("Dependencies sources were not detected nor 'install' command provided. Running 'restore' command")
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
	if len(params.InstallCommandArgs()) > 0 || !sol.DependenciesSourcesAndProjectsPathExist() {
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

	toolName := params.InstallCommandName()
	if toolName == "" {
		// Detect whether the project is a NuGet or .NET project
		toolName, err = getProjectToolName(originalWd)
		if err != nil {
			err = fmt.Errorf("failed while checking for the porject's tool type: %s", err.Error())
			return
		}
	}

	toolType := bidotnet.ConvertNameToToolType(toolName)

	var installCommandArgs []string
	// Set up an Artifactory server as a resolution server if needed
	depsRepo := params.DepsRepo()
	if depsRepo != "" {
		var serverDetails *config.ServerDetails
		serverDetails, err = params.ServerDetails()
		if err != nil {
			err = fmt.Errorf("failed to get server details: %s", err.Error())
			return
		}

		log.Info("Resolving dependencies from", serverDetails.Url, "from repo '", depsRepo, "'")

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

// Identifies if the project operating using .NET Cli to NuGet Cli, preferring .NET Cli
// Notice That for multi-module projects only one of these tools can be identified and will be applied to all modules
func getProjectToolName(wd string) (toolName string, err error) {
	projectConfigFilesPaths, err := getProjectConfigurationFilesPaths(wd)
	if err != nil {
		err = fmt.Errorf("failed which getting file's list in '%s': %s", wd, err.Error())
		return
	}

	var packagesConfigFiles []string
	for _, configFilePath := range projectConfigFilesPaths {
		if strings.HasSuffix(configFilePath, csprojFileSuffix) {
			var fileData []byte
			fileData, err = os.ReadFile(configFilePath)
			if err != nil {
				err = fmt.Errorf("failed to read file '%s': %s", configFilePath, err.Error())
				return
			}

			// If <PackageReference> syntax is detected in the .csproj file - the tool type that is being used is .NET CLI
			if strings.Contains(string(fileData), packageReferenceSyntax) {
				toolName = dotnetToolType
				return
			}
		} else {
			packagesConfigFiles = append(packagesConfigFiles, configFilePath)
		}
	}

	// If <PackageReference> syntax wasn't detected in any .csproj file and packages.config file was found - the tool type that is being used is NuGet CLI
	if len(packagesConfigFiles) > 0 {
		toolName = nugetToolType
		return
	}

	err = errorutils.CheckErrorf("couldn't detect the project's tool type (.NET/NuGet CLI). Please execute 'restore' command.\nNotice: Some entry points enable provision of an 'install' command instead of running it yourself")
	return
}

// Returns a list of all absolute paths of project's configuration files - .csproj files and packages.config files ONLY
func getProjectConfigurationFilesPaths(wd string) (projectConfigFilesPaths []string, err error) {
	err = filepath.WalkDir(wd, func(path string, d fs.DirEntry, innerErr error) error {
		if innerErr != nil {
			return fmt.Errorf("error has occured when trying to access or traverse the files system: %s", err.Error())
		}

		if strings.HasSuffix(path, csprojFileSuffix) || strings.HasSuffix(path, packagesConfigFileName) {
			var absFilePath string
			absFilePath, innerErr = filepath.Abs(path)
			if innerErr != nil {
				return fmt.Errorf("couldn't retrieve file's absolute path for './%s':%s", path, innerErr.Error())
			}
			projectConfigFilesPaths = append(projectConfigFilesPaths, absFilePath)
		}
		return nil
	})
	return
}

func runDotnetRestore(wd string, params utils.AuditParams, toolType bidotnet.ToolchainType, commandExtraArgs []string) (err error) {
	var completeCommandArgs []string
	if len(params.InstallCommandArgs()) > 0 {
		// If the user has provided an 'install' command we run the provided command
		completeCommandArgs = append(completeCommandArgs, params.InstallCommandName())
		completeCommandArgs = append(completeCommandArgs, params.InstallCommandArgs()...)
	} else {
		completeCommandArgs = append(completeCommandArgs, toolType.String(), installCommandName)
	}

	// We add the flag that enables resolution from an Artifactory server (if exists)
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
