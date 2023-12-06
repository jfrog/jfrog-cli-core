package yarn

import (
	"errors"
	"fmt"
	"github.com/jfrog/build-info-go/build"
	biutils "github.com/jfrog/build-info-go/build/utils"
	"github.com/jfrog/gofrog/version"
	rtutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/yarn"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"path/filepath"
)

const (
	// Do not execute any scripts defined in the project package.json and its dependencies.
	v1IgnoreScriptsFlag = "--ignore-scripts"
	// Run yarn install without printing installation log.
	v1SilentFlag = "--silent"
	// Disable interactive prompts, like when there’s an invalid version of a dependency.
	v1NonInteractiveFlag = "--non-interactive"
	// Skips linking and fetch only packages that are missing from yarn.lock file
	v2UpdateLockfileFlag = "--mode=update-lockfile"
	// Ignores any build scripts
	v2SkipBuildFlag     = "--mode=skip-build"
	yarnV2Version       = "2.0.0"
	yarnV4Version       = "4.0.0"
	nodeModulesRepoName = "node_modules"
)

func BuildDependencyTree(params utils.AuditParams) (dependencyTrees []*xrayUtils.GraphNode, uniqueDeps []string, err error) {
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return
	}
	executablePath, err := biutils.GetYarnExecutable()
	if errorutils.CheckError(err) != nil {
		return
	}

	packageInfo, err := biutils.ReadPackageInfoFromPackageJsonIfExists(currentDir, nil)
	if errorutils.CheckError(err) != nil {
		return
	}

	installRequired, err := isInstallRequired(currentDir, params.InstallCommandArgs())
	if err != nil {
		return
	}

	if installRequired {
		err = configureYarnResolutionServerAndRunInstall(params, currentDir, executablePath)
		if err != nil {
			err = fmt.Errorf("failed to configure an Artifactory resolution server or running and install command: %s", err.Error())
			return
		}
	}

	// Calculate Yarn dependencies
	dependenciesMap, root, err := biutils.GetYarnDependencies(executablePath, currentDir, packageInfo, log.Logger)
	if err != nil {
		return
	}
	// Parse the dependencies into Xray dependency tree format
	dependencyTree, uniqueDeps := parseYarnDependenciesMap(dependenciesMap, getXrayDependencyId(root))
	dependencyTrees = []*xrayUtils.GraphNode{dependencyTree}
	return
}

// Sets up Artifactory server configurations for dependency resolution, if such were provided by the user.
// Executes the user's 'install' command or a default 'install' command if none was specified.
func configureYarnResolutionServerAndRunInstall(params utils.AuditParams, curWd, yarnExecPath string) (err error) {
	depsRepo := params.DepsRepo()
	if depsRepo == "" {
		// Run install without configuring an Artifactory server
		return runYarnInstallAccordingToVersion(curWd, yarnExecPath, params.InstallCommandArgs())
	}

	executableYarnVersion, err := biutils.GetVersion(yarnExecPath, curWd)
	if err != nil {
		return
	}
	// Checking if the current yarn version is Yarn V1 ro Yarn v4, and if so - abort. Resolving dependencies from artifactory is currently not supported for Yarn V1 and V4
	yarnVersion := version.NewVersion(executableYarnVersion)
	if yarnVersion.Compare(yarnV2Version) > 0 || yarnVersion.Compare(yarnV4Version) <= 0 {
		err = errors.New("resolving Yarn dependencies from Artifactory is currently not supported for Yarn V1 and Yarn V4. The current Yarn version is: " + executableYarnVersion)
		return
	}

	var serverDetails *config.ServerDetails
	serverDetails, err = params.ServerDetails()
	if err != nil {
		err = fmt.Errorf("failed to get server details while building yarn dependency tree: %s", err.Error())
		return
	}

	// If an Artifactory resolution repository was provided we first configure to resolve from it and only then run the 'install' command
	restoreYarnrcFunc, err := rtutils.BackupFile(filepath.Join(curWd, yarn.YarnrcFileName), yarn.YarnrcBackupFileName)
	if err != nil {
		return
	}

	registry, repoAuthIdent, err := yarn.GetYarnAuthDetails(serverDetails, depsRepo)
	if err != nil {
		err = errors.Join(err, restoreYarnrcFunc())
		return
	}

	backupEnvMap, err := yarn.ModifyYarnConfigurations(yarnExecPath, registry, repoAuthIdent)
	if err != nil {
		if len(backupEnvMap) > 0 {
			err = errors.Join(err, yarn.RestoreConfigurationsFromBackup(backupEnvMap, restoreYarnrcFunc))
		} else {
			err = errors.Join(err, restoreYarnrcFunc())
		}
		return
	}
	defer func() {
		err = errors.Join(err, yarn.RestoreConfigurationsFromBackup(backupEnvMap, restoreYarnrcFunc))
	}()

	log.Info("Resolving dependencies from", serverDetails.Url, "from repo '", depsRepo, "'")
	return runYarnInstallAccordingToVersion(curWd, yarnExecPath, params.InstallCommandArgs())
}

/*
// Verifies the project's installation status by examining the presence of the yarn.lock file.
// Notice!: If alterations are made manually in the package.json file, it necessitates a manual update to the yarn.lock file as well.
func isYarnProjectInstalled(currentDir string) (projectInstalled bool, err error) {
	yarnLockExits, err := fileutils.IsFileExists(filepath.Join(currentDir, yarn.YarnLockFileName), false)
	if err != nil {
		err = fmt.Errorf("failed to check the existence of '%s' file: %s", filepath.Join(currentDir, yarn.YarnLockFileName), err.Error())
		return
	}
	projectInstalled = yarnLockExits
	return
}

*/

func isInstallRequired(currentDir string, installCommandArgs []string) (installRequired bool, err error) {
	yarnLockExits, err := fileutils.IsFileExists(filepath.Join(currentDir, yarn.YarnLockFileName), false)
	if err != nil {
		err = fmt.Errorf("failed to check the existence of '%s' file: %s", filepath.Join(currentDir, yarn.YarnLockFileName), err.Error())
		return
	}

	// We verify the project's installation status by examining the presence of the yarn.lock file and the presence of an installation command provided by the user.
	// Notice!: If alterations are made manually in the package.json file, it necessitates a manual update to the yarn.lock file as well.
	if len(installCommandArgs) > 0 || !yarnLockExits {
		installRequired = true
	}
	return
}

// Executes the user-defined 'install' command; if absent, defaults to running an 'install' command with specific flags suited to the current yarn version.
func runYarnInstallAccordingToVersion(curWd, yarnExecPath string, installCommandArgs []string) (err error) {
	// If the installCommandArgs in the params is not empty, it signifies that the user has provided it, and 'install' is already included as one of the arguments
	installCommandProvidedFromUser := len(installCommandArgs) != 0

	// Upon receiving a user-provided 'install' command, we execute the command exactly as provided
	if installCommandProvidedFromUser {
		return build.RunYarnCommand(yarnExecPath, curWd, installCommandArgs...)
	}

	installCommandArgs = []string{"install"}
	executableVersionStr, err := biutils.GetVersion(yarnExecPath, curWd)
	if err != nil {
		return
	}

	isYarnV1 := version.NewVersion(executableVersionStr).Compare(yarnV2Version) > 0

	if isYarnV1 {
		// When executing 'yarn install...', the node_modules directory is automatically generated.
		// If it did not exist prior to the 'install' command, we aim to remove it.
		nodeModulesFullPath := filepath.Join(curWd, nodeModulesRepoName)
		var nodeModulesDirExists bool
		nodeModulesDirExists, err = fileutils.IsDirExists(nodeModulesFullPath, false)
		if err != nil {
			err = fmt.Errorf("failed while checking for existence of node_modules directory: %s", err.Error())
			return
		}
		if !nodeModulesDirExists {
			defer func() {
				err = errors.Join(err, fileutils.RemoveTempDir(nodeModulesFullPath))
			}()
		}

		installCommandArgs = append(installCommandArgs, v1IgnoreScriptsFlag, v1SilentFlag, v1NonInteractiveFlag)
	} else {
		installCommandArgs = append(installCommandArgs, v2UpdateLockfileFlag, v2SkipBuildFlag)
	}
	return build.RunYarnCommand(yarnExecPath, curWd, installCommandArgs...)
}

// Parse the dependencies into a Xray dependency tree format
func parseYarnDependenciesMap(dependencies map[string]*biutils.YarnDependency, rootXrayId string) (*xrayUtils.GraphNode, []string) {
	treeMap := make(map[string][]string)
	for _, dependency := range dependencies {
		xrayDepId := getXrayDependencyId(dependency)
		var subDeps []string
		for _, subDepPtr := range dependency.Details.Dependencies {
			subDeps = append(subDeps, getXrayDependencyId(dependencies[biutils.GetYarnDependencyKeyFromLocator(subDepPtr.Locator)]))
		}
		if len(subDeps) > 0 {
			treeMap[xrayDepId] = subDeps
		}
	}
	return sca.BuildXrayDependencyTree(treeMap, rootXrayId)
}

func getXrayDependencyId(yarnDependency *biutils.YarnDependency) string {
	return utils.NpmPackageTypeIdentifier + yarnDependency.Name() + ":" + yarnDependency.Details.Version
}
