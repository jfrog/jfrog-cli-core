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

	//todo add flags to install command to change only yarn.lock. check defferences between yarn 1 and yarn 2
	//todo take care of yarn 1 with .yarnrc instead .yarnrc.yml
	yarnrcYmlExits, err := fileutils.IsFileExists(filepath.Join(currentDir, yarn.YarnLockFileName), false)
	if err != nil {
		err = fmt.Errorf("failed to check the existence of '%s' file: %s", filepath.Join(currentDir, yarn.YarnLockFileName), err.Error())
		return
	}
	if !yarnrcYmlExits || len(params.InstallCommandArgs()) != 0 {
		// In case yarn.lock doesn't exist or in case the user has provided us an install command to run
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
		var installCommandArgs []string
		if len(params.InstallCommandArgs()) == 0 {
			installCommandArgs = []string{"install"}
		} else {
			installCommandArgs = params.InstallCommandArgs()
		}
		return build.RunYarnCommand(yarnExecPath, curWd, installCommandArgs...)
	}

	// If an Artifactory resolution repository was provided we first configure to resolve from it and only then run the 'install' command
	executableYarnVersion, err := biUtils.GetVersion(yarnExecPath, curWd)
	if err != nil {
		err = fmt.Errorf("failed to get yarn version: %s", err.Error())
		return
	}
	// TODO check if we can remove barrier of Yarn1 resolve. if not - remove it from default install as well!!!
	// Checking if the current yarn version is Yarn V1, and if so - abort. Resolving dependencies is currently not supported for Yarn V1
	if version.NewVersion(executableYarnVersion).Compare("2.0.0") > 0 {
		err = errors.New("resolving yarn dependencies is currently not supported for Yarn V1")
		return
	}

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

	var installCommandArgs []string
	if len(params.InstallCommandArgs()) == 0 {
		installCommandArgs = []string{"install"}
	} else {
		// If we get installCommandArgs in params it should include 'install' as the first param
		installCommandArgs = params.InstallCommandArgs()
	}
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
