package mvnutils

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/spf13/viper"
)

const (
	mavenExtractorDependencyVersion = "2.28.6"
	classworldsConfFileName         = "classworlds.conf"
	MavenHome                       = "M2_HOME"
)

func RunMvn(configPath, deployableArtifactsFile string, buildConf *utils.BuildConfiguration, goals []string, threads int, insecureTls, disableDeploy bool) error {
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

	mvnRunConfig, err := createMvnRunConfig(dependenciesPath, configPath, deployableArtifactsFile, buildConf, goals, threads, insecureTls, disableDeploy)
	if err != nil {
		return err
	}

	defer os.Remove(mvnRunConfig.buildInfoProperties)
	return mvnRunConfig.runCmd()
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

func createMvnRunConfig(dependenciesPath, configPath, deployableArtifactsFile string, buildConf *utils.BuildConfiguration, goals []string, threads int, insecureTls, disableDeploy bool) (*mvnRunConfig, error) {
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
	if configPath == "" {
		vConfig = viper.New()
		vConfig.SetConfigType(string(utils.YAML))
		vConfig.Set("type", utils.Maven.String())
	} else {
		vConfig, err = utils.ReadConfigFile(configPath, utils.YAML)
		if err != nil {
			return nil, err
		}
	}

	if len(buildConf.BuildName) > 0 && len(buildConf.BuildNumber) > 0 {
		vConfig.Set(utils.BUILD_NAME, buildConf.BuildName)
		vConfig.Set(utils.BUILD_NUMBER, buildConf.BuildNumber)
		vConfig.Set(utils.BUILD_PROJECT, buildConf.Project)
		err = utils.SaveBuildGeneralDetails(buildConf.BuildName, buildConf.BuildNumber, buildConf.Project)
		if err != nil {
			return nil, err
		}
	}
	vConfig.Set(utils.INSECURE_TLS, insecureTls)

	if threads > 0 {
		vConfig.Set(utils.FORK_COUNT, threads)
	}

	if !vConfig.IsSet("deployer") || disableDeploy {
		setEmptyDeployer(vConfig)
	}
	handleIncludeExcludePatterns(vConfig)

	buildInfoProperties, err := utils.CreateBuildInfoPropertiesFile(buildConf.BuildName, buildConf.BuildNumber, buildConf.Project, deployableArtifactsFile, vConfig, utils.Maven)
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
		goals:                        goals,
		buildInfoProperties:          buildInfoProperties,
		artifactoryResolutionEnabled: vConfig.IsSet("resolver"),
		generatedBuildInfoPath:       vConfig.GetString(utils.GENERATED_BUILD_INFO),
		mavenOpts:                    mavenOpts,
		deployableArtifactsFilePath:  vConfig.GetString(utils.DEPLOYABLE_ARTIFACTS),
	}, nil
}

func setEmptyDeployer(vConfig *viper.Viper) {
	vConfig.Set(utils.DEPLOYER_PREFIX+utils.DEPLOY_ARTIFACTS, "false")
	vConfig.Set(utils.DEPLOYER_PREFIX+utils.URL, "http://empty_url")
	vConfig.Set(utils.DEPLOYER_PREFIX+utils.RELEASE_REPO, "empty_repo")
	vConfig.Set(utils.DEPLOYER_PREFIX+utils.SNAPSHOT_REPO, "empty_repo")
}

func handleIncludeExcludePatterns(vConfig *viper.Viper) {
	fixPatternsSeparator(vConfig, utils.INCLUDE_PATTERNS)
	fixPatternsSeparator(vConfig, utils.EXCLUDE_PATTERNS)
}

// The extractor expects the separator to be ", " while the cli uses ";".
func fixPatternsSeparator(vConfig *viper.Viper, patternType string) {
	key := utils.DEPLOYER_PREFIX + patternType
	if vConfig.IsSet(key) {
		curPattern := vConfig.GetString(key)
		vConfig.Set(key, strings.ReplaceAll(curPattern, ";", ", "))
	}
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

func (config *mvnRunConfig) runCmd() error {
	command := config.GetCmd()
	command.Stderr = os.Stderr
	command.Stdout = os.Stderr
	return coreutils.ConvertExitCodeError(errorutils.CheckError(command.Run()))
}
