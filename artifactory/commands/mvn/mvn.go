package mvn

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/generic"
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/spf13/viper"
)

const mavenExtractorDependencyVersion = "2.27.0"

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
	xrayScan        bool
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

func (mc *MvnCommand) SetXrayScan(xrayScan bool) *MvnCommand {
	mc.xrayScan = xrayScan
	return mc
}

func (mc *MvnCommand) IsXrayScan() bool {
	return mc.xrayScan
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
	err = gofrogcmd.RunCmd(mvnRunConfig)
	if err != nil {
		return err
	}
	if mc.IsDetailedSummary() {
		return mc.unmarshalDeployableArtifacts(mvnRunConfig.deployableArtifactsFilePath)
	}
	if mc.IsXrayScan() {
		return mc.conditionalUpload()
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

func (mc *MvnCommand) conditionalUpload() error {
	binariesSpecFile, pomSpecFile, err := commandsutils.ScanDeployableArtifacts(mc.result, mc.serverDetails)
	// First upload binaries
	uploadCmd := generic.NewUploadCommand()
	uploadCmd.SetBuildConfiguration(mc.configuration).SetSpec(binariesSpecFile).SetServerDetails(mc.serverDetails)
	err = uploadCmd.Run()
	if err != nil {
		return err
	}
	// Then Upload pom.xml's
	uploadCmd = generic.NewUploadCommand()
	uploadCmd.SetBuildConfiguration(mc.configuration).SetSpec(pomSpecFile).SetServerDetails(mc.serverDetails)
	return uploadCmd.Run()
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
	dependenciesPath = filepath.Join(dependenciesPath, "maven", mavenExtractorDependencyVersion)

	filename := fmt.Sprintf("build-info-extractor-maven3-%s-uber.jar", mavenExtractorDependencyVersion)
	filePath := fmt.Sprintf("org/jfrog/buildinfo/build-info-extractor-maven3/%s", mavenExtractorDependencyVersion)
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
		vConfig.Set(utils.BUILD_NAME, mc.configuration.BuildName)
		vConfig.Set(utils.BUILD_NUMBER, mc.configuration.BuildNumber)
		vConfig.Set(utils.BUILD_PROJECT, mc.configuration.Project)
		err = utils.SaveBuildGeneralDetails(mc.configuration.BuildName, mc.configuration.BuildNumber, mc.configuration.Project)
		if err != nil {
			return nil, err
		}
	}
	vConfig.Set(utils.INSECURE_TLS, mc.insecureTls)

	if mc.threads > 0 {
		vConfig.Set(utils.FORK_COUNT, mc.threads)
	}

	if !vConfig.IsSet("deployer") {
		setEmptyDeployer(vConfig)
	}

	buildInfoProperties, err := utils.CreateBuildInfoPropertiesFile(mc.configuration.BuildName, mc.configuration.BuildNumber, mc.configuration.Project, mc.IsDetailedSummary() || mc.IsXrayScan(), mc.IsXrayScan(), vConfig, utils.Maven)
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
		generatedBuildInfoPath:       vConfig.GetString(utils.GENERATED_BUILD_INFO),
		mavenOpts:                    mavenOpts,
		deployableArtifactsFilePath:  vConfig.GetString(utils.DEPLOYABLE_ARTIFACTS),
	}, nil
}

func (mc *MvnCommand) unmarshalDeployableArtifacts(filesPath string) error {
	result, err := commandsutils.UnmarshalDeployableArtifacts(filesPath)
	if err != nil {
		return err
	}
	mc.SetResult(result)
	return nil
}

func setEmptyDeployer(vConfig *viper.Viper) {
	vConfig.Set(utils.DEPLOYER_PREFIX+utils.DEPLOY_ARTIFACTS, "false")
	vConfig.Set(utils.DEPLOYER_PREFIX+utils.URL, "http://empty_url")
	vConfig.Set(utils.DEPLOYER_PREFIX+utils.RELEASE_REPO, "empty_repo")
	vConfig.Set(utils.DEPLOYER_PREFIX+utils.SNAPSHOT_REPO, "empty_repo")
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

func (config *mvnRunConfig) GetEnv() map[string]string {
	return map[string]string{}
}

func (config *mvnRunConfig) GetStdWriter() io.WriteCloser {
	return os.Stderr
}

func (config *mvnRunConfig) GetErrWriter() io.WriteCloser {
	return os.Stderr
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
