package utils

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
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

	// Errors
	resolutionErrorPrefix      = "[Resolution]: "
	deploymentErrorPrefix      = "[Deployment]: "
	setServerIdError           = "server ID must be set. Use the --server-id-resolve/deploy flag or configure a default server using 'jfrog c add' and 'jfrog c use' commands. "
	setRepositoryError         = "repository/ies must be set. "
	setSnapshotAndReleaseError = "snapshot and release repositories must be set. "
)

type ConfigFile struct {
	Interactive bool             `yaml:"-"`
	Version     int              `yaml:"version,omitempty"`
	ConfigType  string           `yaml:"type,omitempty"`
	Resolver    utils.Repository `yaml:"resolver,omitempty"`
	Deployer    utils.Repository `yaml:"deployer,omitempty"`
	UsePlugin   bool             `yaml:"usePlugin,omitempty"`
	UseWrapper  bool             `yaml:"useWrapper,omitempty"`
}

func NewConfigFile(confType utils.ProjectType, c *cli.Context) *ConfigFile {
	configFile := &ConfigFile{
		Version:    BuildConfVersion,
		ConfigType: confType.String(),
	}
	configFile.populateConfigFromFlags(c)
	switch confType {
	case utils.Maven:
		configFile.populateMavenConfigFromFlags(c)
	case utils.Gradle:
		configFile.populateGradleConfigFromFlags(c)
	case utils.Nuget, utils.Dotnet:
		configFile.populateNugetConfigFromFlags(c)
	}
	return configFile
}

func CreateBuildConfig(c *cli.Context, confType utils.ProjectType) (err error) {
	global := c.Bool(global)
	projectDir, err := utils.GetProjectDir(global)
	if err != nil {
		return err
	}
	if err = fileutils.CreateDirIfNotExist(projectDir); err != nil {
		return err
	}
	configFilePath := filepath.Join(projectDir, confType.String()+".yaml")
	configFile := NewConfigFile(confType, c)
	if err := configFile.VerifyConfigFile(configFilePath); err != nil {
		return err
	}
	if configFile.Interactive {
		switch confType {
		case utils.Go:
			err = configFile.configGo()
		case utils.Pip:
			err = configFile.configPip()
		case utils.Pipenv:
			err = configFile.configPipenv()
		case utils.Poetry:
			err = configFile.configPoetry()
		case utils.Yarn:
			err = configFile.configYarn()
		case utils.Npm:
			err = configFile.configNpm()
		case utils.Nuget, utils.Dotnet:
			err = configFile.configDotnet()
		case utils.Maven:
			err = configFile.configMaven()
		case utils.Gradle:
			err = configFile.configGradle()
		case utils.Terraform:
			err = configFile.configTerraform()
		}
		if err != nil {
			return errorutils.CheckError(err)
		}
	}
	if err = configFile.validateConfig(); err != nil {
		return err
	}
	resBytes, err := yaml.Marshal(&configFile)
	if err != nil {
		return errorutils.CheckError(err)
	}
	if err = os.WriteFile(configFilePath, resBytes, 0644); err != nil {
		return errorutils.CheckError(err)
	}
	log.Info(confType.String() + " build config successfully created.")
	return nil
}

func isInteractive(c *cli.Context) bool {
	if strings.ToLower(os.Getenv(coreutils.CI)) == "true" {
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

// Populate Maven related configuration from cli flags
func (configFile *ConfigFile) populateMavenConfigFromFlags(c *cli.Context) {
	configFile.Resolver.SnapshotRepo = c.String(resolutionSnapshotsRepo)
	configFile.Resolver.ReleaseRepo = c.String(resolutionReleasesRepo)
	configFile.Deployer.SnapshotRepo = c.String(deploymentSnapshotsRepo)
	configFile.Deployer.ReleaseRepo = c.String(deploymentReleasesRepo)
	configFile.Deployer.IncludePatterns = c.String(includePatterns)
	configFile.Deployer.ExcludePatterns = c.String(excludePatterns)
	configFile.Interactive = configFile.Interactive && !isAnyFlagSet(c, resolutionSnapshotsRepo, resolutionReleasesRepo,
		deploymentSnapshotsRepo, deploymentReleasesRepo, includePatterns, excludePatterns)
}

// Populate Gradle related configuration from cli flags
func (configFile *ConfigFile) populateGradleConfigFromFlags(c *cli.Context) {
	configFile.Deployer.DeployMavenDesc = c.BoolT(deployMavenDesc)
	configFile.Deployer.DeployIvyDesc = c.BoolT(deployIvyDesc)
	configFile.Deployer.IvyPattern = defaultIfNotSet(c, ivyDescPattern, "[organization]/[module]/ivy-[revision].xml")
	configFile.Deployer.ArtifactsPattern = defaultIfNotSet(c, ivyArtifactsPattern, "[organization]/[module]/[revision]/[artifact]-[revision](-[classifier]).[ext]")
	configFile.UsePlugin = c.Bool(usesPlugin)
	configFile.UseWrapper = c.Bool(useWrapper)
	configFile.Interactive = configFile.Interactive && !isAnyFlagSet(c, deployMavenDesc, deployIvyDesc, ivyDescPattern, ivyArtifactsPattern, usesPlugin, useWrapper)
}

// Populate NuGet related configuration from cli flags
func (configFile *ConfigFile) populateNugetConfigFromFlags(c *cli.Context) {
	configFile.Resolver.NugetV2 = c.Bool(nugetV2)
	configFile.Interactive = configFile.Interactive && !isAnyFlagSet(c, nugetV2)
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

func (configFile *ConfigFile) configGo() error {
	return configFile.setDeployerResolver()
}

func (configFile *ConfigFile) configPip() error {
	return configFile.setResolver()
}

func (configFile *ConfigFile) configPipenv() error {
	return configFile.setResolver()
}

func (configFile *ConfigFile) configPoetry() error {
	return configFile.setResolver()
}

func (configFile *ConfigFile) configYarn() error {
	return configFile.setResolver()
}

func (configFile *ConfigFile) configNpm() error {
	return configFile.setDeployerResolver()
}

func (configFile *ConfigFile) configDotnet() error {
	if err := configFile.setResolver(); err != nil {
		return err
	}
	if configFile.Resolver.ServerId != "" {
		configFile.setUseNugetV2()
	}
	return nil
}

func (configFile *ConfigFile) configMaven() error {
	// Set resolution repositories
	if err := configFile.setResolverId(); err != nil {
		return err
	}
	if configFile.Resolver.ServerId != "" {
		configFile.setRepo(&configFile.Resolver.ReleaseRepo, "Set resolution repository for release dependencies", configFile.Resolver.ServerId, utils.Remote)
		configFile.setRepo(&configFile.Resolver.SnapshotRepo, "Set resolution repository for snapshot dependencies", configFile.Resolver.ServerId, utils.Remote)
	}
	// Set deployment repositories
	if err := configFile.setDeployerId(); err != nil {
		return err
	}
	if configFile.Deployer.ServerId != "" {
		configFile.setRepo(&configFile.Deployer.ReleaseRepo, "Set repository for release artifacts deployment", configFile.Deployer.ServerId, utils.Local)
		configFile.setRepo(&configFile.Deployer.SnapshotRepo, "Set repository for snapshot artifacts deployment", configFile.Deployer.ServerId, utils.Local)
		configFile.setIncludeExcludePatterns()
	}
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
		newPattern := AskString("", cases.Title(language.Und, cases.NoLower).String(patternType)+" pattern "+strconv.Itoa(patternNum)+" (leave empty to continue):", true, false)
		if newPattern == "" {
			return strings.Join(patterns, ", ")
		}
		patterns = append(patterns, newPattern)
		patternNum++
	}
}

func (configFile *ConfigFile) configGradle() error {
	if err := configFile.setDeployerResolver(); err != nil {
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
	configFile.UseWrapper = coreutils.AskYesNo("Use Gradle wrapper?", false)
}

func (configFile *ConfigFile) configTerraform() error {
	return configFile.setDeployer()
}

func (configFile *ConfigFile) setDeployer() error {
	// Set deployer id
	if err := configFile.setDeployerId(); err != nil {
		return err
	}

	// Set deployment repository
	if configFile.Deployer.ServerId != "" {
		configFile.setRepo(&configFile.Deployer.Repo, "Set repository for artifacts deployment", configFile.Deployer.ServerId, utils.Local)
	}
	return nil
}

func (configFile *ConfigFile) setResolver() error {
	// Set resolver id
	if err := configFile.setResolverId(); err != nil {
		return err
	}
	// Set resolution repository
	if configFile.Resolver.ServerId != "" {
		configFile.setRepo(&configFile.Resolver.Repo, "Set repository for dependencies resolution", configFile.Resolver.ServerId, utils.Remote)
	}
	return nil
}

func (configFile *ConfigFile) setDeployerResolver() error {
	if err := configFile.setResolver(); err != nil {
		return err
	}
	return configFile.setDeployer()
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

func (configFile *ConfigFile) setRepo(repo *string, message string, serverId string, repoType utils.RepoType) {
	if *repo == "" {
		*repo = readRepo(message+PressTabMsg, serverId, repoType, utils.Virtual)
	}
}

func (configFile *ConfigFile) setMavenIvyDescriptors() {
	configFile.Deployer.DeployMavenDesc = coreutils.AskYesNo("Deploy Maven descriptors?", false)
	configFile.Deployer.DeployIvyDesc = coreutils.AskYesNo("Deploy Ivy descriptors?", false)

	if configFile.Deployer.DeployIvyDesc {
		configFile.Deployer.IvyPattern = AskStringWithDefault("", "Set Ivy descriptor pattern", "[organization]/[module]/ivy-[revision].xml")
		configFile.Deployer.ArtifactsPattern = AskStringWithDefault("", "Set Ivy artifact pattern", "[organization]/[module]/[revision]/[artifact]-[revision](-[classifier]).[ext]")
	}
}

func (configFile *ConfigFile) setUseNugetV2() {
	configFile.Resolver.NugetV2 = coreutils.AskYesNo("Use NuGet V2 Protocol?", false)
}

func validateRepositoryConfig(repository *utils.Repository, errorPrefix string) error {
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

	return AskFromList("", "Set Artifactory server ID", false, ConvertToSuggests(serversIds), defaultServer), nil
}

func readRepo(promptPrefix string, serverId string, repoTypes ...utils.RepoType) string {
	availableRepos, err := getRepositories(serverId, repoTypes...)
	if err != nil {
		log.Error("failed getting repositories list: " + err.Error())
		// Continue without auto complete.
		availableRepos = []string{}
	}
	if len(availableRepos) > 0 {
		return AskFromListWithMismatchConfirmation(promptPrefix, "Repository not found.", ConvertToSuggests(availableRepos))
	}
	return AskString("", promptPrefix, false, false)
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

func getRepositories(serverId string, repoTypes ...utils.RepoType) ([]string, error) {
	artDetails, err := config.GetSpecificConfig(serverId, false, true)
	if err != nil {
		return nil, err
	}

	return utils.GetRepositories(artDetails, repoTypes...)
}

func defaultIfNotSet(c *cli.Context, flagName string, defaultValue string) string {
	if c.IsSet(flagName) {
		return c.String(flagName)
	}
	return defaultValue
}
