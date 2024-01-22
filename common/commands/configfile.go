package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/repository"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/urfave/cli"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

const BuildConfVersion = 1

const (
	// Common flags
	global             = "global"
	resolutionServerId = "server-id-resolve"
	deploymentServerId = "server-id-deploy"
	resolutionRepo     = "repo-resolve"
	deploymentRepo     = "repo-deploy"

	// Maven flags
	resolutionReleasesRepo  = "repo-resolve-releases"
	resolutionSnapshotsRepo = "repo-resolve-snapshots"
	deploymentReleasesRepo  = "repo-deploy-releases"
	deploymentSnapshotsRepo = "repo-deploy-snapshots"
	includePatterns         = "include-patterns"
	excludePatterns         = "exclude-patterns"

	// Gradle flags
	usesPlugin          = "uses-plugin"
	useWrapper          = "use-wrapper"
	deployMavenDesc     = "deploy-maven-desc"
	deployIvyDesc       = "deploy-ivy-desc"
	ivyDescPattern      = "ivy-desc-pattern"
	ivyArtifactsPattern = "ivy-artifacts-pattern"

	// Nuget flags
	nugetV2 = "nuget-v2"

	// Default values
	defaultIvyDescPattern      = "[organization]/[module]/ivy-[revision].xml"
	defaultIvyArtifactsPattern = "[organization]/[module]/[revision]/[artifact]-[revision](-[classifier]).[ext]"

	// Errors
	resolutionErrorPrefix      = "[Resolution]: "
	deploymentErrorPrefix      = "[Deployment]: "
	setServerIdError           = "server ID must be set. Use the --server-id-resolve/deploy flag or configure a default server using 'jfrog c add' and 'jfrog c use' commands. "
	setRepositoryError         = "repository/ies must be set. "
	setSnapshotAndReleaseError = "snapshot and release repositories must be set. "
)

type ConfigFile struct {
	Interactive bool               `yaml:"-"`
	Version     int                `yaml:"version,omitempty"`
	ConfigType  string             `yaml:"type,omitempty"`
	Resolver    project.Repository `yaml:"resolver,omitempty"`
	Deployer    project.Repository `yaml:"deployer,omitempty"`
	UsePlugin   bool               `yaml:"usePlugin,omitempty"`
	UseWrapper  bool               `yaml:"useWrapper,omitempty"`
}

type ConfigOption func(c *ConfigFile)

func newConfigFile(confType project.ProjectType) *ConfigFile {
	return &ConfigFile{
		Version:    BuildConfVersion,
		ConfigType: confType.String(),
	}
}

func NewConfigFileWithOptions(confType project.ProjectType, options ...ConfigOption) *ConfigFile {
	configFile := newConfigFile(confType)
	configFile.setDefaultValues(confType)

	for _, option := range options {
		option(configFile)
	}
	return configFile
}

func (configFile *ConfigFile) setDefaultValues(confType project.ProjectType) {
	configFile.Interactive = !isCI()

	if confType == project.Gradle {
		configFile.Deployer.DeployMavenDesc = true
		configFile.Deployer.DeployIvyDesc = true
		configFile.Deployer.IvyPattern = defaultIvyDescPattern
		configFile.Deployer.ArtifactsPattern = defaultIvyArtifactsPattern
	}
}

func NewConfigFile(confType project.ProjectType, c *cli.Context) *ConfigFile {
	configFile := newConfigFile(confType)
	configFile.populateConfigFromFlags(c)
	switch confType {
	case project.Maven:
		configFile.populateMavenConfigFromFlags(c)
	case project.Gradle:
		configFile.populateGradleConfigFromFlags(c)
	case project.Nuget, project.Dotnet:
		configFile.populateNugetConfigFromFlags(c)
	}
	return configFile
}

func CreateBuildConfig(c *cli.Context, confType project.ProjectType) (err error) {
	return createBuildConfig(c.Bool(global), confType, NewConfigFile(confType, c))
}

func CreateBuildConfigWithOptions(global bool, confType project.ProjectType, options ...ConfigOption) (err error) {
	return createBuildConfig(global, confType, NewConfigFileWithOptions(confType, options...))
}

func createBuildConfig(global bool, confType project.ProjectType, configFile *ConfigFile) (err error) {
	// Create/verify project directory
	projectDir, err := utils.GetProjectDir(global)
	if err != nil {
		return err
	}
	if err = fileutils.CreateDirIfNotExist(projectDir); err != nil {
		return err
	}
	// Populate, validate and write the config file
	configFilePath := filepath.Join(projectDir, confType.String()+".yaml")
	if err := configFile.VerifyConfigFile(configFilePath); err != nil {
		return err
	}
	if err := handleInteractiveConfigCreation(configFile, confType); err != nil {
		return err
	}
	if err = configFile.validateConfig(); err != nil {
		return err
	}
	return writeConfigFile(configFile, configFilePath)
}

func handleInteractiveConfigCreation(configFile *ConfigFile, confType project.ProjectType) (err error) {
	if !configFile.Interactive {
		return
	}
	// Please Notice that confType is the actual project type, and the passed value is the package type, and they not always the same.
	// For example, the package type for pip, pipenv and poetry is 'pypi'.
	switch confType {
	case project.Go:
		return configFile.setDeployerResolver(repository.Go)
	case project.Pip, project.Pipenv, project.Poetry:
		return configFile.setResolver(repository.Pypi)
	case project.Yarn:
		return configFile.setResolver(repository.Npm)
	case project.Npm:
		return configFile.setDeployerResolver(repository.Npm)
	case project.Nuget, project.Dotnet:
		return configFile.configDotnet()
	case project.Maven:
		return configFile.configMaven()
	case project.Gradle:
		return configFile.configGradle()
	case project.Terraform:
		return configFile.setResolver(repository.Terraform)
	}
	return
}

func writeConfigFile(configFile *ConfigFile, destination string) (err error) {
	resBytes, err := yaml.Marshal(&configFile)
	if err != nil {
		return errorutils.CheckError(err)
	}
	if err = os.WriteFile(destination, resBytes, 0644); err != nil {
		return errorutils.CheckError(err)
	}
	log.Info(configFile.ConfigType + " build config successfully created.")
	return
}

func isCI() bool {
	return strings.ToLower(os.Getenv(coreutils.CI)) == "true"
}

func isInteractive(c *cli.Context) bool {
	if isCI() {
		return false
	}
	return !isAnyFlagSet(c, resolutionServerId, resolutionRepo, deploymentServerId, deploymentRepo)
}

func isAnyFlagSet(c *cli.Context, flagNames ...string) bool {
	for _, flagName := range flagNames {
		if c.IsSet(flagName) {
			return true
		}
	}
	return false
}

// Populate configuration from cli flags
func (configFile *ConfigFile) populateConfigFromFlags(c *cli.Context) {
	configFile.Resolver.ServerId = c.String(resolutionServerId)
	configFile.Resolver.Repo = c.String(resolutionRepo)
	configFile.Deployer.ServerId = c.String(deploymentServerId)
	configFile.Deployer.Repo = c.String(deploymentRepo)
	configFile.Interactive = isInteractive(c)
}

func WithResolverServerId(serverId string) ConfigOption {
	return func(c *ConfigFile) {
		c.Resolver.ServerId = serverId
		c.Interactive = false
	}
}

func WithDeployerServerId(serverId string) ConfigOption {
	return func(c *ConfigFile) {
		c.Deployer.ServerId = serverId
		c.Interactive = false
	}
}

func WithResolverRepo(repoId string) ConfigOption {
	return func(c *ConfigFile) {
		c.Resolver.Repo = repoId
		c.Interactive = false
	}
}

func WithDeployerRepo(repoId string) ConfigOption {
	return func(c *ConfigFile) {
		c.Deployer.Repo = repoId
		c.Interactive = false
	}
}

// Populate Maven related configuration from cli flags
func (configFile *ConfigFile) populateMavenConfigFromFlags(c *cli.Context) {
	configFile.Resolver.SnapshotRepo = c.String(resolutionSnapshotsRepo)
	configFile.Resolver.ReleaseRepo = c.String(resolutionReleasesRepo)
	configFile.Deployer.SnapshotRepo = c.String(deploymentSnapshotsRepo)
	configFile.Deployer.ReleaseRepo = c.String(deploymentReleasesRepo)
	configFile.Deployer.IncludePatterns = c.String(includePatterns)
	configFile.Deployer.ExcludePatterns = c.String(excludePatterns)
	configFile.UseWrapper = c.Bool(useWrapper)
	configFile.Interactive = configFile.Interactive && !isAnyFlagSet(c, resolutionSnapshotsRepo, resolutionReleasesRepo,
		deploymentSnapshotsRepo, deploymentReleasesRepo, includePatterns, excludePatterns)
}

func WithResolverSnapshotRepo(repoId string) ConfigOption {
	return func(c *ConfigFile) {
		c.Resolver.SnapshotRepo = repoId
		c.Interactive = false
	}
}

func WithResolverReleaseRepo(repoId string) ConfigOption {
	return func(c *ConfigFile) {
		c.Resolver.ReleaseRepo = repoId
		c.Interactive = false
	}
}

func WithDeployerSnapshotRepo(repoId string) ConfigOption {
	return func(c *ConfigFile) {
		c.Deployer.SnapshotRepo = repoId
		c.Interactive = false
	}
}

func WithDeployerReleaseRepo(repoId string) ConfigOption {
	return func(c *ConfigFile) {
		c.Deployer.ReleaseRepo = repoId
		c.Interactive = false
	}
}

func WithDeployerIncludePatterns(includePatterns string) ConfigOption {
	return func(c *ConfigFile) {
		c.Deployer.IncludePatterns = includePatterns
		c.Interactive = false
	}
}

func WithDeployerExcludePatterns(excludePatterns string) ConfigOption {
	return func(c *ConfigFile) {
		c.Deployer.ExcludePatterns = excludePatterns
		c.Interactive = false
	}
}

func UseWrapper(useWrapper bool) ConfigOption {
	return func(c *ConfigFile) {
		c.UseWrapper = useWrapper
	}
}

// Populate Gradle related configuration from cli flags
func (configFile *ConfigFile) populateGradleConfigFromFlags(c *cli.Context) {
	configFile.Deployer.DeployMavenDesc = c.BoolT(deployMavenDesc)
	configFile.Deployer.DeployIvyDesc = c.BoolT(deployIvyDesc)
	configFile.Deployer.IvyPattern = defaultIfNotSet(c, ivyDescPattern, defaultIvyDescPattern)
	configFile.Deployer.ArtifactsPattern = defaultIfNotSet(c, ivyArtifactsPattern, defaultIvyArtifactsPattern)
	configFile.UsePlugin = c.Bool(usesPlugin)
	configFile.UseWrapper = c.Bool(useWrapper)
	configFile.Interactive = configFile.Interactive && !isAnyFlagSet(c, deployMavenDesc, deployIvyDesc, ivyDescPattern, ivyArtifactsPattern, usesPlugin, useWrapper)
}

func WithMavenDescDeployment(mavenDesc bool) ConfigOption {
	return func(c *ConfigFile) {
		c.Deployer.DeployMavenDesc = mavenDesc
		c.Interactive = false
	}
}

func WithIvyDescDeployment(ivyDesc bool) ConfigOption {
	return func(c *ConfigFile) {
		c.Deployer.DeployIvyDesc = ivyDesc
		c.Interactive = false
	}
}

func WithIvyDeploymentPattern(ivyPattern string) ConfigOption {
	return func(c *ConfigFile) {
		c.Deployer.IvyPattern = ivyPattern
		c.Interactive = false
	}
}

func WithArtifactsDeploymentPattern(artifactsPattern string) ConfigOption {
	return func(c *ConfigFile) {
		c.Deployer.IvyPattern = artifactsPattern
		c.Interactive = false
	}
}

func UsePlugin(usePlugin bool) ConfigOption {
	return func(c *ConfigFile) {
		c.UsePlugin = usePlugin
		c.Interactive = false
	}
}

// Populate NuGet related configuration from cli flags
func (configFile *ConfigFile) populateNugetConfigFromFlags(c *cli.Context) {
	configFile.Resolver.NugetV2 = c.Bool(nugetV2)
	configFile.Interactive = configFile.Interactive && !isAnyFlagSet(c, nugetV2)
}

func WithResolverNugetV2(nugetV2 bool) ConfigOption {
	return func(c *ConfigFile) {
		c.Resolver.NugetV2 = nugetV2
		c.Interactive = false
	}
}

// Verify config file doesn't exist or prompt to override it
func (configFile *ConfigFile) VerifyConfigFile(configFilePath string) error {
	exists, err := fileutils.IsFileExists(configFilePath, false)
	if err != nil {
		return err
	}
	if exists {
		if !configFile.Interactive {
			return nil
		}
		override := coreutils.AskYesNo("Configuration file already exists at "+configFilePath+". Override it?", false)
		if !override {
			return errorutils.CheckErrorf("operation canceled")
		}
		return nil
	}

	// Create config file to make sure the path is valid
	f, err := os.OpenFile(configFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if errorutils.CheckError(err) != nil {
		return err
	}
	err = f.Close()
	if errorutils.CheckError(err) != nil {
		return err
	}
	// The file will be written at the end of successful configuration command.
	return errorutils.CheckError(os.Remove(configFilePath))
}

func (configFile *ConfigFile) configDotnet() error {
	if err := configFile.setResolver(repository.Nuget); err != nil {
		return err
	}
	if configFile.Resolver.ServerId != "" {
		configFile.setUseNugetV2()
	}
	return nil
}

func (configFile *ConfigFile) configMaven() error {
	if err := configFile.setDeployerResolver(repository.Maven); err != nil {
		return err
	}
	if configFile.Deployer.ServerId != "" {
		configFile.setIncludeExcludePatterns()
	}
	configFile.UseWrapper = coreutils.AskYesNo("Use Maven wrapper?", true)
	return nil
}

func (configFile *ConfigFile) setIncludeExcludePatterns() {
	if !coreutils.AskYesNo("Would you like to filter out some of the deployed artifacts?", false) {
		return
	}
	log.Output("You may set multiple wildcard patterns, to match the artifacts' names you'd like to include and/or exclude from being deployed.")
	includePatterns := getIncludeExcludePatterns("include")
	if includePatterns != "" {
		configFile.Deployer.IncludePatterns = includePatterns
	}
	excludePatterns := getIncludeExcludePatterns("exclude")
	if excludePatterns != "" {
		configFile.Deployer.ExcludePatterns = excludePatterns
	}
}

func getIncludeExcludePatterns(patternType string) string {
	var patterns []string
	patternNum := 1
	for {
		newPattern := ioutils.AskString("", cases.Title(language.Und, cases.NoLower).String(patternType)+" pattern "+strconv.Itoa(patternNum)+" (leave empty to continue):", true, false)
		if newPattern == "" {
			return strings.Join(patterns, ", ")
		}
		patterns = append(patterns, newPattern)
		patternNum++
	}
}

func (configFile *ConfigFile) configGradle() error {
	if err := configFile.setDeployerResolver(repository.Gradle); err != nil {
		return err
	}
	if configFile.Deployer.ServerId != "" {
		configFile.setMavenIvyDescriptors()
	}
	configFile.readGradleGlobalConfig()
	return nil
}

func (configFile *ConfigFile) readGradleGlobalConfig() {
	configFile.UsePlugin = coreutils.AskYesNo("Is the Gradle Artifactory Plugin already applied in the build script?", false)
	configFile.UseWrapper = coreutils.AskYesNo("Use Gradle wrapper?", true)
}

func (configFile *ConfigFile) setDeployer(packageType string) error {
	// Set deployer id
	if err := configFile.setDeployerId(); err != nil {
		return err
	}

	// Set deployment repository
	if configFile.Deployer.ServerId != "" {
		deployerRepos, err := getRepositories(configFile.Resolver.ServerId, packageType, utils.Virtual, utils.Local)
		if err != nil {
			log.Error("failed getting repositories list: " + err.Error())
			// Continue without auto complete.
			deployerRepos = []string{}
		}
		if packageType == repository.Maven {
			configFile.setRepo(&configFile.Resolver.SnapshotRepo, "Set repository for release artifacts deployment", deployerRepos)
			configFile.setRepo(&configFile.Resolver.SnapshotRepo, "Set repository for snapshot artifacts deployment", deployerRepos)
		} else {
			configFile.setRepo(&configFile.Deployer.Repo, "Set repository for artifacts deployment", deployerRepos)
		}
	}
	return nil
}

func (configFile *ConfigFile) setResolver(packageType string) error {
	// Set resolver id
	if err := configFile.setResolverId(); err != nil {
		return err
	}
	// Set resolution repository
	if configFile.Resolver.ServerId != "" {
		resolverRepos, err := getRepositories(configFile.Resolver.ServerId, packageType, utils.Virtual, utils.Remote)
		if err != nil {
			log.Error("failed getting repositories list: " + err.Error())
			// Continue without auto complete.
			resolverRepos = []string{}
		}
		if packageType == repository.Maven {
			configFile.setRepo(&configFile.Resolver.SnapshotRepo, "Set repository for release dependencies", resolverRepos)
			configFile.setRepo(&configFile.Resolver.SnapshotRepo, "Set repository for snapshot dependencies", resolverRepos)
		} else {
			configFile.setRepo(&configFile.Resolver.Repo, "Set repository for dependencies resolution", resolverRepos)
		}
	}
	return nil
}

func (configFile *ConfigFile) setDeployerResolver(packageType string) error {
	if err := configFile.setResolver(packageType); err != nil {
		return err
	}
	return configFile.setDeployer(packageType)
}

func (configFile *ConfigFile) setResolverId() error {
	return configFile.setServerId(&configFile.Resolver.ServerId, "Resolve dependencies from Artifactory?")
}

func (configFile *ConfigFile) setDeployerId() error {
	return configFile.setServerId(&configFile.Deployer.ServerId, "Deploy project artifacts to Artifactory?")
}

func (configFile *ConfigFile) setServerId(serverId *string, useArtifactoryQuestion string) error {
	var err error
	*serverId, err = readArtifactoryServer(useArtifactoryQuestion)
	return err
}

func (configFile *ConfigFile) setRepo(repo *string, promptPrefix string, availableRepos []string) {
	if *repo == "" {
		if len(availableRepos) > 0 {
			*repo = ioutils.AskFromListWithMismatchConfirmation(promptPrefix, "Repository not found.", ioutils.ConvertToSuggests(availableRepos))
		} else {
			*repo = ioutils.AskString("", promptPrefix, false, false)
		}
	}
}

func (configFile *ConfigFile) setMavenIvyDescriptors() {
	configFile.Deployer.DeployMavenDesc = coreutils.AskYesNo("Deploy Maven descriptors?", false)
	configFile.Deployer.DeployIvyDesc = coreutils.AskYesNo("Deploy Ivy descriptors?", false)

	if configFile.Deployer.DeployIvyDesc {
		configFile.Deployer.IvyPattern = ioutils.AskStringWithDefault("", "Set Ivy descriptor pattern", defaultIvyDescPattern)
		configFile.Deployer.ArtifactsPattern = ioutils.AskStringWithDefault("", "Set Ivy artifact pattern", defaultIvyArtifactsPattern)
	}
}

func (configFile *ConfigFile) setUseNugetV2() {
	configFile.Resolver.NugetV2 = coreutils.AskYesNo("Use NuGet V2 Protocol?", false)
}

func validateRepositoryConfig(repository *project.Repository, errorPrefix string) error {
	releaseRepo := repository.ReleaseRepo
	snapshotRepo := repository.SnapshotRepo

	if repository.ServerId != "" && repository.Repo == "" && releaseRepo == "" && snapshotRepo == "" {
		return errorutils.CheckErrorf(errorPrefix + setRepositoryError)
	}
	// Server-id flag was not provided.
	if repository.ServerId == "" {
		// If no Server ID provided, check if provided via environment variable
		serverId := os.Getenv(coreutils.ServerID)
		if serverId == "" {
			// For config commands - resolver/deployer server-id flags are optional.
			// In case no server-id flag was provided we use the default configured server id.
			defaultServerDetails, err := config.GetDefaultServerConf()
			if err != nil {
				return err
			}
			if defaultServerDetails != nil {
				serverId = defaultServerDetails.ServerId
			}
		}
		// No default server was configured and also no environment variable
		if serverId == "" {
			// Repositories flags were provided.
			if repository.Repo != "" || releaseRepo != "" || snapshotRepo != "" {
				return errorutils.CheckErrorf(errorPrefix + setServerIdError)
			}
		} else if repository.Repo != "" || releaseRepo != "" || snapshotRepo != "" {
			// Server-id flag wasn't provided and repositories flags were provided - the default configured global server will be chosen.
			repository.ServerId = serverId
		}
	}
	// Release/snapshot repositories should be entangled to each other.
	if (releaseRepo == "" && snapshotRepo != "") || (releaseRepo != "" && snapshotRepo == "") {
		return errorutils.CheckErrorf(errorPrefix + setSnapshotAndReleaseError)
	}
	return nil
}

// Validate spec file configuration
func (configFile *ConfigFile) validateConfig() error {
	err := validateRepositoryConfig(&configFile.Resolver, resolutionErrorPrefix)
	if err != nil {
		return err
	}
	return validateRepositoryConfig(&configFile.Deployer, deploymentErrorPrefix)
}

// Get Artifactory serverId from the user. If useArtifactoryQuestion is not empty, ask first whether to use artifactory.
func readArtifactoryServer(useArtifactoryQuestion string) (string, error) {
	// Get all Artifactory servers
	serversIds, defaultServer, err := getServersIdAndDefault()
	if err != nil {
		return "", err
	}
	if len(serversIds) == 0 {
		return "", errorutils.CheckErrorf("No Artifactory servers configured. Use the 'jfrog c add' command to set the Artifactory server details.")
	}

	// Ask whether to use artifactory
	if useArtifactoryQuestion != "" {
		useArtifactory := coreutils.AskYesNo(useArtifactoryQuestion, true)
		if !useArtifactory {
			return "", nil
		}
	}

	return ioutils.AskFromList("", "Set Artifactory server ID", false, ioutils.ConvertToSuggests(serversIds), defaultServer), nil
}

func getServersIdAndDefault() ([]string, string, error) {
	allConfigs, err := config.GetAllServersConfigs()
	if err != nil {
		return nil, "", err
	}
	var defaultVal string
	var serversId []string
	for _, v := range allConfigs {
		if v.IsDefault {
			defaultVal = v.ServerId
		}
		serversId = append(serversId, v.ServerId)
	}
	return serversId, defaultVal, nil
}

func getRepositories(serverId string, packageType string, repoTypes ...utils.RepoType) ([]string, error) {
	artDetails, err := config.GetSpecificConfig(serverId, false, true)
	if err != nil {
		return nil, err
	}

	return utils.GetRepositories(artDetails, packageType, repoTypes...)
}

func defaultIfNotSet(c *cli.Context, flagName string, defaultValue string) string {
	if c.IsSet(flagName) {
		return c.String(flagName)
	}
	return defaultValue
}
