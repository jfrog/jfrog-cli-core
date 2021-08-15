package mvn

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	commandsutils "github.com/jfrog/jfrog-cli-core/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/spf13/viper"
)

const mavenExtractorDependencyVersion = "2.28.6"

// Deprecated. This version is the latest published in JCenter.
const mavenExtractorDependencyJCenterVersion = "2.23.0"
const classworldsConfFileName = "classworlds.conf"
const MavenHome = "M2_HOME"

type MvnCommand struct {
	goals           []string
	configPath      string
	insecureTls     bool
	configuration   *utils.BuildConfiguration
	serverDetails   *config.ServerDetails
	threads         int
	detailedSummary bool
	result          *commandsutils.Result
}

func NewMvnCommand() *MvnCommand {
	return &MvnCommand{}
}

func (mc *MvnCommand) SetServerDetails(serverDetails *config.ServerDetails) *MvnCommand {
	mc.serverDetails = serverDetails
	return mc
}

func (mc *MvnCommand) SetConfiguration(configuration *utils.BuildConfiguration) *MvnCommand {
	mc.configuration = configuration
	return mc
}

func (mc *MvnCommand) SetConfigPath(configPath string) *MvnCommand {
	mc.configPath = configPath
	return mc
}

func (mc *MvnCommand) SetGoals(goals []string) *MvnCommand {
	mc.goals = goals
	return mc
}

func (mc *MvnCommand) SetThreads(threads int) *MvnCommand {
	mc.threads = threads
	return mc
}

func (mc *MvnCommand) SetInsecureTls(insecureTls bool) *MvnCommand {
	mc.insecureTls = insecureTls
	return mc
}

func (mc *MvnCommand) SetDetailedSummary(detailedSummary bool) *MvnCommand {
	mc.detailedSummary = detailedSummary
	return mc
}

func (mc *MvnCommand) IsDetailedSummary() bool {
	return mc.detailedSummary
}

func (mc *MvnCommand) Result() *commandsutils.Result {
	return mc.result
}

func (mc *MvnCommand) SetResult(result *commandsutils.Result) *MvnCommand {
	mc.result = result
	return mc
}

func (mc *MvnCommand) Run() error {
	log.Info("Running Mvn...")
	err := validateMavenInstallation()
	if err != nil {
		return err
	}

	var dependenciesPath string
	dependenciesPath, err = downloadDependencies()
	if err != nil {
		return err
	}

	mvnRunConfig, err := mc.createMvnRunConfig(dependenciesPath)
	if err != nil {
		return err
	}

	defer os.Remove(mvnRunConfig.buildInfoProperties)
	err = mvnRunConfig.runCmd()
	if err != nil {
		return err
	}
	if mc.IsDetailedSummary() {
		return mc.unmarshalDeployableArtifacts(mvnRunConfig.deployableArtifactsFilePath)
	}
	return nil
}

// Returns the ServerDetails. The information returns from the config file provided.
func (mc *MvnCommand) ServerDetails() (*config.ServerDetails, error) {
	// Get the serverDetails from the config file.
	var err error
	if mc.serverDetails == nil {
		vConfig, err := utils.ReadConfigFile(mc.configPath, utils.YAML)
		if err != nil {
			return nil, err
		}
		mc.serverDetails, err = utils.GetServerDetails(vConfig)
	}
	return mc.serverDetails, err
}

func (mc *MvnCommand) CommandName() string {
	return "rt_maven"
}

func validateMavenInstallation() error {
	log.Debug("Checking prerequisites.")
	mavenHome := os.Getenv(MavenHome)
	if mavenHome == "" {
		return errorutils.CheckError(errors.New(MavenHome + " environment variable is not set"))
	}
	return nil
}

func downloadDependencies() (string, error) {
	dependenciesPath, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return "", err
	}
	extractorVersion := utils.GetExtractorVersion(mavenExtractorDependencyVersion, mavenExtractorDependencyJCenterVersion)
	dependenciesPath = filepath.Join(dependenciesPath, "maven", extractorVersion)

	filename := fmt.Sprintf("build-info-extractor-maven3-%s-uber.jar", extractorVersion)
	filePath := fmt.Sprintf("org/jfrog/buildinfo/build-info-extractor-maven3/%s", extractorVersion)
	downloadPath := path.Join(filePath, filename)

	err = utils.DownloadExtractorIfNeeded(downloadPath, filepath.Join(dependenciesPath, filename))
	if err != nil {
		return "", err
	}

	err = createClassworldsConfig(dependenciesPath)
	return dependenciesPath, err
}

func createClassworldsConfig(dependenciesPath string) error {
	classworldsPath := filepath.Join(dependenciesPath, classworldsConfFileName)

	if fileutils.IsPathExists(classworldsPath, false) {
		return nil
	}
	return errorutils.CheckError(ioutil.WriteFile(classworldsPath, []byte(utils.ClassworldsConf), 0644))
}

func (mc *MvnCommand) createMvnRunConfig(dependenciesPath string) (*mvnRunConfig, error) {
	var err error
	var javaExecPath string

	javaHome := os.Getenv("JAVA_HOME")
	if javaHome != "" {
		javaExecPath = filepath.Join(javaHome, "bin", "java")
	} else {
		javaExecPath, err = exec.LookPath("java")
		if err != nil {
			return nil, errorutils.CheckError(err)
		}
	}

	mavenHome := os.Getenv("M2_HOME")
	plexusClassworlds, err := filepath.Glob(filepath.Join(mavenHome, "boot", "plexus-classworlds*.jar"))
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	mavenOpts := os.Getenv("MAVEN_OPTS")

	if len(plexusClassworlds) != 1 {
		return nil, errorutils.CheckError(errors.New("couldn't find plexus-classworlds-x.x.x.jar in Maven installation path, please check M2_HOME environment variable"))
	}

	var currentWorkdir string
	currentWorkdir, err = os.Getwd()
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	var vConfig *viper.Viper
	vConfig, err = utils.ReadConfigFile(mc.configPath, utils.YAML)
	if err != nil {
		return nil, err
	}

	if len(mc.configuration.BuildName) > 0 && len(mc.configuration.BuildNumber) > 0 {
		vConfig.Set(utils.BuildName, mc.configuration.BuildName)
		vConfig.Set(utils.BuildNumber, mc.configuration.BuildNumber)
		vConfig.Set(utils.BuildProject, mc.configuration.Project)
		err = utils.SaveBuildGeneralDetails(mc.configuration.BuildName, mc.configuration.BuildNumber, mc.configuration.Project)
		if err != nil {
			return nil, err
		}
	}
	vConfig.Set(utils.InsecureTls, mc.insecureTls)

	if mc.threads > 0 {
		vConfig.Set(utils.ForkCount, mc.threads)
	}

	if !vConfig.IsSet("deployer") {
		setEmptyDeployer(vConfig)
	}

	buildInfoProperties, err := utils.CreateBuildInfoPropertiesFile(mc.configuration.BuildName, mc.configuration.BuildNumber, mc.configuration.Project, mc.IsDetailedSummary(), vConfig, utils.Maven)
	if err != nil {
		return nil, err
	}

	return &mvnRunConfig{
		java:                         javaExecPath,
		pluginDependencies:           dependenciesPath,
		plexusClassworlds:            plexusClassworlds[0],
		cleassworldsConfig:           filepath.Join(dependenciesPath, classworldsConfFileName),
		mavenHome:                    mavenHome,
		workspace:                    currentWorkdir,
		goals:                        mc.goals,
		buildInfoProperties:          buildInfoProperties,
		artifactoryResolutionEnabled: vConfig.IsSet("resolver"),
		generatedBuildInfoPath:       vConfig.GetString(utils.GeneratedBuildInfo),
		mavenOpts:                    mavenOpts,
		deployableArtifactsFilePath:  vConfig.GetString(utils.DeployableArtifacts),
	}, nil
}

func (mc *MvnCommand) unmarshalDeployableArtifacts(filesPath string) error {
	result, err := commandsutils.UnmarshalDeployableArtifacts(filesPath, mc.configPath)
	if err != nil {
		return err
	}
	mc.SetResult(result)
	return nil
}

func setEmptyDeployer(vConfig *viper.Viper) {
	vConfig.Set(utils.DeployerPrefix+utils.DeployArtifacts, "false")
	vConfig.Set(utils.DeployerPrefix+utils.Url, "http://empty_url")
	vConfig.Set(utils.DeployerPrefix+utils.ReleaseRepo, "empty_repo")
	vConfig.Set(utils.DeployerPrefix+utils.SnapshotRepo, "empty_repo")
}

func (config *mvnRunConfig) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, config.java)
	cmd = append(cmd, "-classpath", config.plexusClassworlds)
	cmd = append(cmd, "-Dmaven.home="+config.mavenHome)
	cmd = append(cmd, "-DbuildInfoConfig.propertiesFile="+config.buildInfoProperties)
	if config.artifactoryResolutionEnabled {
		cmd = append(cmd, "-DbuildInfoConfig.artifactoryResolutionEnabled=true")
	}
	cmd = append(cmd, "-Dm3plugin.lib="+config.pluginDependencies)
	cmd = append(cmd, "-Dclassworlds.conf="+config.cleassworldsConfig)
	cmd = append(cmd, "-Dmaven.multiModuleProjectDirectory="+config.workspace)
	if config.mavenOpts != "" {
		cmd = append(cmd, strings.Split(config.mavenOpts, " ")...)
	}
	cmd = append(cmd, "org.codehaus.plexus.classworlds.launcher.Launcher")
	cmd = append(cmd, config.goals...)
	return exec.Command(cmd[0], cmd[1:]...)
}

func (config *mvnRunConfig) runCmd() error {
	command := config.GetCmd()
	command.Stderr = os.Stderr
	command.Stdout = os.Stderr
	return coreutils.ConvertExitCodeError(errorutils.CheckError(command.Run()))
}

type mvnRunConfig struct {
	java                         string
	plexusClassworlds            string
	cleassworldsConfig           string
	mavenHome                    string
	pluginDependencies           string
	workspace                    string
	pom                          string
	goals                        []string
	buildInfoProperties          string
	artifactoryResolutionEnabled bool
	generatedBuildInfoPath       string
	mavenOpts                    string
	deployableArtifactsFilePath  string
}
