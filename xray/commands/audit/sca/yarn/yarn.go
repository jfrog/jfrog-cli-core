package yarn

import (
	"errors"
	"fmt"
	"github.com/jfrog/build-info-go/build"
	biUtils "github.com/jfrog/build-info-go/build/utils"
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
	v1ModulesFolderFlag  = "--modules-folder="
	v1IgnoreScriptsFlag  = "--ignore-scripts"
	v1SilentFlag         = "--silent"
	v1NonInteractiveFlag = "--non-interactive"
	yarnV2Version        = "2.0.0"
	nodeModulesRepoName  = "node_modules"
)

func BuildDependencyTree(params utils.AuditParams) (dependencyTrees []*xrayUtils.GraphNode, uniqueDeps []string, err error) {
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return
	}
	executablePath, err := biUtils.GetYarnExecutable()
	if errorutils.CheckError(err) != nil {
		return
	}

	packageInfo, err := biUtils.ReadPackageInfoFromPackageJsonIfExists(currentDir, nil)
	if errorutils.CheckError(err) != nil {
		return
	}

	projectInstalled, err := isYarnProjectInstalled(currentDir)
	if err != nil {
		return
	}

	if !projectInstalled || len(params.InstallCommandArgs()) != 0 {
		// In case project is not "installed" or in case the user has provided an 'install' command to run
		var serverDetails *config.ServerDetails
		serverDetails, err = params.ServerDetails()
		if err != nil {
			err = fmt.Errorf("failed to get server details while building yarn dependency tree: %s", err.Error())
			return
		}
		depsRepo := params.DepsRepo()
		err = configureYarnResolutionServerAndRunInstall(params, currentDir, executablePath, serverDetails, depsRepo)
		if err != nil {
			err = fmt.Errorf("failed to configure an Artifactory resolution server or running and install command: %s", err.Error())
			return
		}
	}

	// Calculate Yarn dependencies
	dependenciesMap, root, err := biUtils.GetYarnDependencies(executablePath, currentDir, packageInfo, log.Logger)
	if err != nil {
		return
	}
	// Parse the dependencies into Xray dependency tree format
	dependencyTree, uniqueDeps := parseYarnDependenciesMap(dependenciesMap, getXrayDependencyId(root))
	dependencyTrees = []*xrayUtils.GraphNode{dependencyTree}
	return
}

func configureYarnResolutionServerAndRunInstall(params utils.AuditParams, curWd, yarnExecPath string, serverDetails *config.ServerDetails, depsRepo string) (err error) {
	if depsRepo == "" {
		// Run install without configuring an Artifactory server
		return runYarnInstallAccordingToVersion(curWd, yarnExecPath, params.InstallCommandArgs())
	}
	/*
		executableYarnVersion, err := biUtils.GetVersion(yarnExecPath, curWd)
		if err != nil {
			err = fmt.Errorf("failed to get yarn version: %s", err.Error())
			return
		}

		// TODO check if we can remove barrier of Yarn1 resolve. if not - remove it from the default install above as well!!!
		// Checking if the current yarn version is Yarn V1, and if so - abort. Resolving dependencies is currently not supported for Yarn V1
		if version.NewVersion(executableYarnVersion).Compare("2.0.0") > 0 {
			err = errors.New("resolving yarn dependencies is currently not supported for Yarn V1")
			return
		}
	*/

	// If an Artifactory resolution repository was provided we first configure to resolve from it and only then run the 'install' command
	restoreYarnrcFunc, err := rtutils.YarnBackupFile(filepath.Join(curWd, yarn.YarnrcFileName), yarn.YarnrcBackupFileName)
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

	return runYarnInstallAccordingToVersion(curWd, yarnExecPath, params.InstallCommandArgs())
}

// Checks if the project is 'installed' by checking the existence of yarn.lock file.
// In case a manual change was made in package.json file - yarn.lock file must be updated as well
func isYarnProjectInstalled(currentDir string) (projectInstalled bool, err error) {
	yarnrcYmlExits, err := fileutils.IsFileExists(filepath.Join(currentDir, yarn.YarnLockFileName), false)
	if err != nil {
		err = fmt.Errorf("failed to check the existence of '%s' file: %s", filepath.Join(currentDir, yarn.YarnLockFileName), err.Error())
		return
	}
	projectInstalled = yarnrcYmlExits
	return
}

func runYarnInstallAccordingToVersion(curWd, yarnExecPath string, installCommandArgs []string) (err error) {
	// If installCommandArgs in params is not empty it has been provided from the user and  already contains 'install' as one of the args
	installCommandProvidedFromUser := len(installCommandArgs) != 0
	if !installCommandProvidedFromUser {
		installCommandArgs = []string{"install"}
	}

	executableVersionStr, err := biUtils.GetVersion(yarnExecPath, curWd)
	if err != nil {
		return
	}

	isYarnV1 := version.NewVersion(executableVersionStr).Compare(yarnV2Version) > 0
	if !installCommandProvidedFromUser && isYarnV1 {
		// In Yarn 1 node_modules repo will be auto generated, and we don't want to leave it inside the directory if wasn't already there when automatically running our default install command
		var nodeModulesExist bool
		nodeModulesExist, err = fileutils.IsDirExists(filepath.Join(curWd, nodeModulesRepoName), false)
		if err != nil {
			err = fmt.Errorf("failed while checking for existence of node_modules directory: %s", err.Error())
			return
		}

		if !nodeModulesExist {
			var tmpNodeModulesDir string
			tmpNodeModulesDir, err = fileutils.CreateTempDir()
			defer func() {
				err = errors.Join(err, fileutils.RemoveTempDir(tmpNodeModulesDir))
			}()
			if err != nil {
				err = fmt.Errorf("failed to create tmporary directory for node_modules: %s", err.Error())
				return
			}
			installCommandArgs = append(installCommandArgs, v1ModulesFolderFlag+tmpNodeModulesDir)
		}
		installCommandArgs = append(installCommandArgs, v1IgnoreScriptsFlag, v1SilentFlag, v1NonInteractiveFlag)
	}
	// TODO check if we can also avoid creating cache in yarn 2 if it wasn't exist
	// TODO add --mode=update-lockfile flag to v2 ??

	/*
		TODO
		if we add flags to yarn v2 change the flow here: first 'if' is to check if provided from user:
		if NO: run install
		if YES: check if its yarn 1 or yarn 2:
			if YARN 1: put the existing flow of yarn 1
			if YARN 2: put the new addition of flags there
	*/

	return build.RunYarnCommand(yarnExecPath, curWd, installCommandArgs...)
}

// Parse the dependencies into a Xray dependency tree format
func parseYarnDependenciesMap(dependencies map[string]*biUtils.YarnDependency, rootXrayId string) (*xrayUtils.GraphNode, []string) {
	treeMap := make(map[string][]string)
	for _, dependency := range dependencies {
		xrayDepId := getXrayDependencyId(dependency)
		var subDeps []string
		for _, subDepPtr := range dependency.Details.Dependencies {
			subDeps = append(subDeps, getXrayDependencyId(dependencies[biUtils.GetYarnDependencyKeyFromLocator(subDepPtr.Locator)]))
		}
		if len(subDeps) > 0 {
			treeMap[xrayDepId] = subDeps
		}
	}
	return sca.BuildXrayDependencyTree(treeMap, rootXrayId)
}

func getXrayDependencyId(yarnDependency *biUtils.YarnDependency) string {
	return utils.NpmPackageTypeIdentifier + yarnDependency.Name() + ":" + yarnDependency.Details.Version
}
