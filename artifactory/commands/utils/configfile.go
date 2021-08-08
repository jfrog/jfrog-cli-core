package utils

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"gopkg.in/yaml.v2"
)

const BUILD_CONF_VERSION = 1

const (
	// Common flags
	Global             = "global"
	ResolutionServerId = "server-id-resolve"
	DeploymentServerId = "server-id-deploy"
	ResolutionRepo     = "repo-resolve"
	DeploymentRepo     = "repo-deploy"

	// Maven flags
	ResolutionReleasesRepo  = "repo-resolve-releases"
	ResolutionSnapshotsRepo = "repo-resolve-snapshots"
	DeploymentReleasesRepo  = "repo-deploy-releases"
	DeploymentSnapshotsRepo = "repo-deploy-snapshots"

	// Gradle flags
	UsesPlugin          = "uses-plugin"
	UseWrapper          = "use-wrapper"
	DeployMavenDesc     = "deploy-maven-desc"
	DeployIvyDesc       = "deploy-ivy-desc"
	IvyDescPattern      = "ivy-desc-pattern"
	IvyArtifactsPattern = "ivy-artifacts-pattern"

	// Nuget flags
	NugetV2 = "nuget-v2"
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

func NewConfigFile(confType utils.ProjectType, c *cli.Context) (*ConfigFile, error) {
	configFile := &ConfigFile{
		Version:    BUILD_CONF_VERSION,
		ConfigType: confType.String(),
	}
	err := configFile.populateConfigFromFlags(c)
	if err != nil {
		return nil, err
	}
	switch confType {
	case utils.Maven:
		configFile.populateMavenConfigFromFlags(c)
	case utils.Gradle:
		configFile.populateGradleConfigFromFlags(c)
	case utils.Nuget, utils.Dotnet:
		configFile.populateNugetConfigFromFlags(c)
	}
	return configFile, nil
}

func CreateBuildConfig(c *cli.Context, confType utils.ProjectType) (err error) {
	global := c.Bool(Global)
	projectDir, err := utils.GetProjectDir(global)
	if err != nil {
		return err
	}
	if err = fileutils.CreateDirIfNotExist(projectDir); err != nil {
		return err
	}
	configFilePath := filepath.Join(projectDir, confType.String()+".yaml")
	configFile, err := NewConfigFile(confType, c)
	if err != nil {
		return err
	}
	if err := configFile.VerifyConfigFile(configFilePath); err != nil {
		return err
	}
	if configFile.Interactive {
		switch confType {
		case utils.Go:
			err = configFile.configGo()
		case utils.Pip:
			err = configFile.configPip()
		case utils.Yarn:
			err = configFile.configYarn()
		case utils.Npm:
			err = configFile.configNpm()
		case utils.Dotnet:
			fallthrough
		case utils.Nuget:
			err = configFile.configDotnet()
		case utils.Maven:
			err = configFile.configMaven()
		case utils.Gradle:
			err = configFile.configGradle()
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
	if err = ioutil.WriteFile(configFilePath, resBytes, 0644); err != nil {
		return errorutils.CheckError(err)
	}
	log.Info(confType.String() + " build config successfully created.")
	return nil
}

func isInteractive(c *cli.Context) bool {
	if strings.ToLower(os.Getenv(coreutils.CI)) == "true" {
		return false
	}
	return !isAnyFlagSet(c, ResolutionServerId, ResolutionRepo, DeploymentServerId, DeploymentRepo)
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
func (configFile *ConfigFile) populateConfigFromFlags(c *cli.Context) (err error) {
	resolverServerId, deployerServerId, err := getServerId(c)
	if err != nil {
		return
	}
	resolverRepo := c.String(ResolutionRepo)
	deployerRepo := c.String(DeploymentRepo)
	// Resolver/deployer server-id flags are optional. if not provided the configured default server will be chosen.
	// need to check if only deployer esolver is ok?
	if resolverRepo != "" {
		configFile.Resolver.ServerId = resolverServerId
		configFile.Resolver.Repo = resolverRepo
	}
	if deployerRepo != "" {
		configFile.Deployer.ServerId = deployerServerId
		configFile.Deployer.Repo = deployerRepo
	}
	configFile.Interactive = isInteractive(c)
	return
}

// For package managers' config commands flags resolver/deployer server-id are optional.
// In case no server-id was provided the default configured server id will be used.
func getServerId(c *cli.Context) (resolverServerId, deployerServerId string, err error) {
	resolverServerId = c.String(ResolutionServerId)
	deployerServerId = c.String(DeploymentServerId)
	if resolverServerId != "" && deployerServerId != "" {
		return
	}
	serverDetails, err := config.GetDefaultServerConf()
	if err != nil || serverDetails == nil {
		return resolverServerId, deployerServerId, err
	}
	if resolverServerId == "" {
		resolverServerId = serverDetails.ServerId
	}
	if deployerServerId == "" {
		deployerServerId = serverDetails.ServerId
	}
	return
}

// Populate Maven related configuration from cli flags
func (configFile *ConfigFile) populateMavenConfigFromFlags(c *cli.Context) {
	configFile.Resolver.SnapshotRepo = c.String(ResolutionSnapshotsRepo)
	configFile.Resolver.ReleaseRepo = c.String(ResolutionReleasesRepo)
	configFile.Deployer.SnapshotRepo = c.String(DeploymentSnapshotsRepo)
	configFile.Deployer.ReleaseRepo = c.String(DeploymentReleasesRepo)
	configFile.Interactive = configFile.Interactive && !isAnyFlagSet(c, ResolutionSnapshotsRepo, ResolutionReleasesRepo, DeploymentSnapshotsRepo, DeploymentReleasesRepo)
}

// Populate Gradle related configuration from cli flags
func (configFile *ConfigFile) populateGradleConfigFromFlags(c *cli.Context) {
	configFile.Deployer.DeployMavenDesc = c.BoolT(DeployMavenDesc)
	configFile.Deployer.DeployIvyDesc = c.BoolT(DeployIvyDesc)
	configFile.Deployer.IvyPattern = defaultIfNotSet(c, IvyDescPattern, "[organization]/[module]/ivy-[revision].xml")
	configFile.Deployer.ArtifactsPattern = defaultIfNotSet(c, IvyArtifactsPattern, "[organization]/[module]/[revision]/[artifact]-[revision](-[classifier]).[ext]")
	configFile.UsePlugin = c.Bool(UsesPlugin)
	configFile.UseWrapper = c.Bool(UseWrapper)
	configFile.Interactive = configFile.Interactive && !isAnyFlagSet(c, DeployMavenDesc, DeployIvyDesc, IvyDescPattern, IvyArtifactsPattern, UsesPlugin, UseWrapper)
}

// Populate NuGet related configuration from cli flags
func (configFile *ConfigFile) populateNugetConfigFromFlags(c *cli.Context) {
	configFile.Resolver.NugetV2 = c.Bool(NugetV2)
	configFile.Interactive = configFile.Interactive && !isAnyFlagSet(c, NugetV2)
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
			return errorutils.CheckError(errors.New("operation canceled"))
		}
		return nil
	}

	// Create config file to make sure the path is valid
	f, err := os.OpenFile(configFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if errorutils.CheckError(err) != nil {
		return err
	}
	f.Close()
	// The file will be written at the end of successful configuration command.
	return errorutils.CheckError(os.Remove(configFilePath))
}

func (configFile *ConfigFile) configGo() error {
	return configFile.setDeployerResolver()
}

func (configFile *ConfigFile) configPip() error {
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
		if err := configFile.setRepo(&configFile.Resolver.ReleaseRepo, "Set resolution repository for release dependencies", configFile.Resolver.ServerId, utils.REMOTE); err != nil {
			return err
		}
		if err := configFile.setRepo(&configFile.Resolver.SnapshotRepo, "Set resolution repository for snapshot dependencies", configFile.Resolver.ServerId, utils.REMOTE); err != nil {
			return err
		}
	}
	// Set deployment repositories
	if err := configFile.setDeployerId(); err != nil {
		return err
	}
	if configFile.Deployer.ServerId != "" {
		if err := configFile.setRepo(&configFile.Deployer.ReleaseRepo, "Set repository for release artifacts deployment", configFile.Deployer.ServerId, utils.LOCAL); err != nil {
			return err
		}
		return configFile.setRepo(&configFile.Deployer.SnapshotRepo, "Set repository for snapshot artifacts deployment", configFile.Deployer.ServerId, utils.LOCAL)
	}
	return nil
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

func (configFile *ConfigFile) setDeployer() error {
	// Set deployer id
	if err := configFile.setDeployerId(); err != nil {
		return err
	}

	// Set deployment repository
	if configFile.Deployer.ServerId != "" {
		return configFile.setRepo(&configFile.Deployer.Repo, "Set repository for artifacts deployment", configFile.Deployer.ServerId, utils.LOCAL)
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
		return configFile.setRepo(&configFile.Resolver.Repo, "Set repository for dependencies resolution", configFile.Resolver.ServerId, utils.REMOTE)
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

func (configFile *ConfigFile) setRepo(repo *string, message string, serverId string, repoType utils.RepoType) error {
	var err error
	if *repo == "" {
		*repo, err = readRepo(message+PressTabMsg, serverId, repoType, utils.VIRTUAL)
	}
	return err
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

// Check correctness of spec file configuration
func (configFile *ConfigFile) validateConfig() error {
	resolver := configFile.Resolver
	releaseRepo := resolver.ReleaseRepo
	snapshotRepo := resolver.SnapshotRepo
	if resolver.ServerId != "" {
		if resolver.Repo == "" && releaseRepo == "" && snapshotRepo == "" {
			return errorutils.CheckError(errors.New("Resolution repository/ies must be set."))
		}
		if (releaseRepo == "" && snapshotRepo != "") || (releaseRepo != "" && snapshotRepo == "") {
			return errorutils.CheckError(errors.New("Resolution snapshot and release repositories must be set."))
		}
	} else {
		if resolver.Repo != "" || releaseRepo != "" || snapshotRepo != "" {
			return errorutils.CheckError(errors.New("Resolver server ID must be set. use --server-id-resolve flag or configure a default server using 'jfrog c add' and 'jfrog c use' commands. "))
		}
	}
	deployer := configFile.Deployer
	releaseRepo = deployer.ReleaseRepo
	snapshotRepo = deployer.SnapshotRepo
	if deployer.ServerId != "" {
		if deployer.Repo == "" && releaseRepo == "" && snapshotRepo == "" {
			return errorutils.CheckError(errors.New("Deployment repository/ies must be set."))
		}
		if (releaseRepo == "" && snapshotRepo != "") || (releaseRepo != "" && snapshotRepo == "") {
			return errorutils.CheckError(errors.New("Deployment snapshot and release repositories must be set."))
		}
	} else {
		if deployer.Repo != "" || releaseRepo != "" || snapshotRepo != "" {
			return errorutils.CheckError(errors.New("Deployer server ID must be set. use --server-id-deploy flag or configure a default server using 'jfrog c add' and 'jfrog c use' commands. "))
		}
	}
	return nil
}

// Get Artifactory serverId from the user. If useArtifactoryQuestion is not empty, ask first whether to use artifactory.
func readArtifactoryServer(useArtifactoryQuestion string) (string, error) {
	// Get all Artifactory servers
	serversIds, defaultServer, err := getServersIdAndDefault()
	if err != nil {
		return "", err
	}
	if len(serversIds) == 0 {
		return "", errorutils.CheckError(errors.New("No Artifactory servers configured. Use the 'jfrog c add' command to set the Artifactory server details."))
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

func readRepo(promptPrefix string, serverId string, repoTypes ...utils.RepoType) (string, error) {
	availableRepos, err := getRepositories(serverId, repoTypes...)
	if err != nil {
		log.Error("failed getting repositories list: " + err.Error())
		// Continue without auto complete.
		availableRepos = []string{}
	}
	if len(availableRepos) > 0 {
		return AskFromListWithMismatchConfirmation(promptPrefix, "Repository not found.", ConvertToSuggests(availableRepos)), nil
	}
	return AskString("", promptPrefix, false, false), nil
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

	artAuth, err := artDetails.CreateArtAuthConfig()
	if err != nil {
		return nil, err
	}

	return utils.GetRepositories(artAuth, repoTypes...)
}

func defaultIfNotSet(c *cli.Context, flagName string, defaultValue string) string {
	if c.IsSet(flagName) {
		return c.String(flagName)
	}
	return defaultValue
}
