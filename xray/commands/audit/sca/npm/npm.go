package npm

import (
	"errors"
	"fmt"
	biutils "github.com/jfrog/build-info-go/build/utils"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/npm"
	utils2 "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	xrayUtils "github.com/jfrog/jfrog-client-go/xray/services/utils"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	"path/filepath"
)

const (
	ignoreScriptsFlag = "--ignore-scripts"
)

func BuildDependencyTree(params utils.AuditParams) (dependencyTrees []*xrayUtils.GraphNode, uniqueDeps []string, err error) {
	currentDir, err := coreutils.GetWorkingDirectory()
	if err != nil {
		return
	}
	npmVersion, npmExecutablePath, err := biutils.GetNpmVersionAndExecPath(log.Logger)
	if err != nil {
		return
	}
	packageInfo, err := biutils.ReadPackageInfoFromPackageJsonIfExists(currentDir, npmVersion)
	if err != nil {
		return
	}

	treeDepsParam := createTreeDepsParam(params)

	var restoreNpmrcFunc func() error
	restoreNpmrcFunc, err = configureResolutionServerIfNeeded(params)
	defer func() {
		if restoreNpmrcFunc != nil {
			err = errors.Join(err, restoreNpmrcFunc())
		}
	}()

	// Calculate npm dependencies
	dependenciesMap, err := biutils.CalculateDependenciesMap(npmExecutablePath, currentDir, packageInfo.BuildInfoModuleId(), treeDepsParam, log.Logger)
	if err != nil {
		log.Info("Used npm version:", npmVersion.GetVersion())
		return
	}
	var dependenciesList []buildinfo.Dependency
	for _, dependency := range dependenciesMap {
		dependenciesList = append(dependenciesList, dependency.Dependency)
	}
	// Parse the dependencies into Xray dependency tree format
	dependencyTree, uniqueDeps := parseNpmDependenciesList(dependenciesList, packageInfo)
	dependencyTrees = []*xrayUtils.GraphNode{dependencyTree}
	return
}

// We check for the existence of DepsRepo and ServerDetails in the audit params or in a Jfrog config file.
// If found we configure an artifactory server for resolving dependencies in the current project
func configureResolutionServerIfNeeded(params utils.AuditParams) (restoreNpmrcFunc func() error, err error) {
	depsRepo := params.DepsRepo()
	serverDetails, err := params.ServerDetails()
	if err != nil {
		err = fmt.Errorf("couldn't get server details for resolve: %s", err.Error())
		return
	}
	if depsRepo == "" || serverDetails == nil {
		// In case we don't have DepsRepo or ServerDetails, we search for a Jfrog config file, if exists, in order to get them
		var isNpmConfigFileExists bool
		npmYamlFilePath := filepath.Join(".jfrog", "projects", "npm.yaml") // todo can this file be in other path?
		isNpmConfigFileExists, err = fileutils.IsFileExists(npmYamlFilePath, false)
		if !isNpmConfigFileExists {
			return
		}

		var npmConfigYamlData *viper.Viper
		npmConfigYamlData, err = utils2.ReadConfigFile(npmYamlFilePath, utils2.YAML)
		if err != nil {
			return
		}

		if serverDetails == nil {
			serverId := npmConfigYamlData.GetString("resolver.serverId")
			serverDetails, err = config.GetSpecificConfig(serverId, true, false)
		}
		if depsRepo == "" {
			depsRepo = npmConfigYamlData.GetString("resolver.repo")
		}
	}

	restoreNpmrcFunc, err = configNpmResolutionServer(depsRepo, serverDetails)
	if err != nil {
		err = fmt.Errorf("configuring an artifactory server for resolution failed: %s", err.Error())
	}
	return
}

// Creating npmrc in order to set an artifactory server as the resolver server
func configNpmResolutionServer(depsRepo string, serverDetails *config.ServerDetails) (restoreNpmrcFunc func() error, err error) {
	npmCmd := npm.NewNpmCommand("install", false).SetServerDetails(serverDetails)
	if err = npmCmd.PreparePrerequisites(depsRepo); err != nil {
		return
	}
	if err = npmCmd.CreateTempNpmrc(); err != nil {
		return
	}
	restoreNpmrcFunc = npmCmd.RestoreNpmrcFunc()
	return
}

func createTreeDepsParam(params utils.AuditParams) biutils.NpmTreeDepListParam {
	if params == nil {
		return biutils.NpmTreeDepListParam{
			Args:               addIgnoreScriptsFlag([]string{}),
			InstallCommandArgs: params.InstallCommandArgs(),
		}
	}
	npmTreeDepParam := biutils.NpmTreeDepListParam{
		Args:               addIgnoreScriptsFlag(params.Args()),
		InstallCommandArgs: params.InstallCommandArgs(),
	}
	if npmParams, ok := params.(utils.AuditNpmParams); ok {
		npmTreeDepParam.IgnoreNodeModules = npmParams.NpmIgnoreNodeModules()
		npmTreeDepParam.OverwritePackageLock = npmParams.NpmOverwritePackageLock()
	}
	return npmTreeDepParam
}

// Add the --ignore-scripts to prevent execution of npm scripts during npm install.
func addIgnoreScriptsFlag(npmArgs []string) []string {
	if !slices.Contains(npmArgs, ignoreScriptsFlag) {
		return append(npmArgs, ignoreScriptsFlag)
	}
	return npmArgs
}

// Parse the dependencies into an Xray dependency tree format
func parseNpmDependenciesList(dependencies []buildinfo.Dependency, packageInfo *biutils.PackageInfo) (*xrayUtils.GraphNode, []string) {
	treeMap := make(map[string][]string)
	for _, dependency := range dependencies {
		dependencyId := utils.NpmPackageTypeIdentifier + dependency.Id
		for _, requestedByNode := range dependency.RequestedBy {
			parent := utils.NpmPackageTypeIdentifier + requestedByNode[0]
			if children, ok := treeMap[parent]; ok {
				treeMap[parent] = appendUniqueChild(children, dependencyId)
			} else {
				treeMap[parent] = []string{dependencyId}
			}
		}
	}
	return sca.BuildXrayDependencyTree(treeMap, utils.NpmPackageTypeIdentifier+packageInfo.BuildInfoModuleId())
}

func appendUniqueChild(children []string, candidateDependency string) []string {
	for _, existingChild := range children {
		if existingChild == candidateDependency {
			return children
		}
	}
	return append(children, candidateDependency)
}
