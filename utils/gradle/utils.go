package gradleutils

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/spf13/viper"
)

const (
	gradleExtractorDependencyVersion = "4.24.12"
	gradleInitScriptTemplate         = "gradle.init"
	usePlugin                        = "useplugin"
	useWrapper                       = "usewrapper"
	gradleBuildInfoProperties        = "BUILDINFO_PROPFILE"
)

func RunGradle(tasks, configPath, deployableArtifactsFile string, configuration *utils.BuildConfiguration, threads int, useWrapperIfMissingConfig, disableDeploy bool) error {
	gradleDependenciesDir, gradlePluginFilename, err := downloadGradleDependencies()
	if err != nil {
		return err
	}
	gradleRunConfig, err := createGradleRunConfig(tasks, configPath, deployableArtifactsFile, configuration, threads, gradleDependenciesDir, gradlePluginFilename, useWrapperIfMissingConfig, disableDeploy)
	if err != nil {
		return err
	}
	defer os.Remove(gradleRunConfig.env[gradleBuildInfoProperties])
	return gradleRunConfig.runCmd()
}

func downloadGradleDependencies() (gradleDependenciesDir, gradlePluginFilename string, err error) {
	dependenciesPath, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return
	}
	gradleDependenciesDir = filepath.Join(dependenciesPath, "gradle", gradleExtractorDependencyVersion)
	gradlePluginFilename = fmt.Sprintf("build-info-extractor-gradle-%s-uber.jar", gradleExtractorDependencyVersion)

	filePath := fmt.Sprintf("org/jfrog/buildinfo/build-info-extractor-gradle/%s", gradleExtractorDependencyVersion)
	downloadPath := path.Join(filePath, gradlePluginFilename)

	err = utils.DownloadExtractorIfNeeded(downloadPath, filepath.Join(gradleDependenciesDir, gradlePluginFilename))
	return
}

func createGradleRunConfig(tasks, configPath, deployableArtifactsFile string, configuration *utils.BuildConfiguration, threads int, gradleDependenciesDir, gradlePluginFilename string, useWrapperIfMissingConfig, disableDeploy bool) (*gradleRunConfig, error) {
	runConfig := &gradleRunConfig{
		env:   map[string]string{},
		tasks: tasks,
	}
	var vConfig *viper.Viper
	var err error
	if configPath == "" {
		vConfig = viper.New()
		vConfig.SetConfigType(string(utils.YAML))
		vConfig.Set("type", utils.Gradle.String())
		vConfig.Set(useWrapper, useWrapperIfMissingConfig)
	} else {
		vConfig, err = utils.ReadConfigFile(configPath, utils.YAML)
		if err != nil {
			return nil, err
		}
	}

	runConfig.gradle, err = getGradleExecPath(vConfig.GetBool(useWrapper))
	if err != nil {
		return nil, err
	}

	if threads > 0 {
		vConfig.Set(utils.FORK_COUNT, threads)
	}

	if disableDeploy {
		setEmptyDeployer(vConfig)
	}

	runConfig.env[gradleBuildInfoProperties], err = utils.CreateBuildInfoPropertiesFile(configuration.BuildName, configuration.BuildNumber, configuration.Project, deployableArtifactsFile, vConfig, utils.Gradle)
	if err != nil {
		return nil, err
	}
	if deployableArtifactsFile != "" {
		// Save the path to a temp file, where buildinfo project will write the deployable artifacts details.
		runConfig.env[utils.DEPLOYABLE_ARTIFACTS] = vConfig.Get(utils.DEPLOYABLE_ARTIFACTS).(string)
	}

	if !vConfig.GetBool(usePlugin) {
		runConfig.initScript, err = getInitScript(gradleDependenciesDir, gradlePluginFilename)
		if err != nil {
			return nil, err
		}
	}

	return runConfig, nil
}

func setEmptyDeployer(vConfig *viper.Viper) {
	vConfig.Set(utils.DEPLOYER_PREFIX+utils.DEPLOY_ARTIFACTS, "false")
	vConfig.Set(utils.DEPLOYER_PREFIX+utils.URL, "http://empty_url")
	vConfig.Set(utils.DEPLOYER_PREFIX+utils.REPO, "empty_repo")
}

func getInitScript(gradleDependenciesDir, gradlePluginFilename string) (string, error) {
	gradleDependenciesDir, err := filepath.Abs(gradleDependenciesDir)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	initScriptPath := filepath.Join(gradleDependenciesDir, gradleInitScriptTemplate)

	exists, err := fileutils.IsFileExists(initScriptPath, false)
	if exists || err != nil {
		return initScriptPath, err
	}

	gradlePluginPath := filepath.Join(gradleDependenciesDir, gradlePluginFilename)
	gradlePluginPath = strings.Replace(gradlePluginPath, "\\", "\\\\", -1)
	initScriptContent := strings.Replace(utils.GradleInitScript, "${pluginLibDir}", gradlePluginPath, -1)
	if !fileutils.IsPathExists(gradleDependenciesDir, false) {
		err = os.MkdirAll(gradleDependenciesDir, 0777)
		if errorutils.CheckError(err) != nil {
			return "", err
		}
	}

	return initScriptPath, errorutils.CheckError(ioutil.WriteFile(initScriptPath, []byte(initScriptContent), 0644))
}

type gradleRunConfig struct {
	gradle     string
	tasks      string
	initScript string
	env        map[string]string
}

func (config *gradleRunConfig) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, config.gradle)
	if config.initScript != "" {
		cmd = append(cmd, "--init-script", config.initScript)
	}
	cmd = append(cmd, strings.Split(config.tasks, " ")...)

	log.Info("Running gradle command:", strings.Join(cmd, " "))
	return exec.Command(cmd[0], cmd[1:]...)
}

func (config *gradleRunConfig) runCmd() error {
	command := config.GetCmd()
	command.Env = os.Environ()
	for k, v := range config.env {
		command.Env = append(command.Env, k+"="+v)
	}
	command.Stderr = os.Stderr
	command.Stdout = os.Stderr
	return command.Run()
}

func getGradleExecPath(useWrapper bool) (string, error) {
	if useWrapper {
		if coreutils.IsWindows() {
			return "gradlew.bat", nil
		}
		return "./gradlew", nil
	}
	gradleExec, err := exec.LookPath("gradle")
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return gradleExec, nil
}
