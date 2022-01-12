package gradleutils

import (
	"path/filepath"
	"strings"

	"github.com/jfrog/build-info-go/build"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/spf13/viper"
)

const (
	usePlugin  = "useplugin"
	useWrapper = "usewrapper"
)

func RunGradle(tasks, configPath, deployableArtifactsFile string, configuration *utils.BuildConfiguration, threads int, useWrapperIfMissingConfig, disableDeploy bool) error {
	buildInfoService := utils.CreateBuildInfoService()
	buildName, err := configuration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := configuration.GetBuildNumber()
	if err != nil {
		return err
	}
	gradleBuild, err := buildInfoService.GetOrCreateBuildWithProject(buildName, buildNumber, configuration.GetProject())
	if err != nil {
		return errorutils.CheckError(err)
	}
	gradleModule, err := gradleBuild.AddGradleModule("")
	if err != nil {
		return errorutils.CheckError(err)
	}
	props, wrapper, plugin, err := createGradleRunConfig(configPath, deployableArtifactsFile, configuration, threads, useWrapperIfMissingConfig, disableDeploy)
	if err != nil {
		return err
	}
	dependenciesPath, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return err
	}
	dependencyLocalPath := filepath.Join(dependenciesPath, "gradle", build.GradleExtractorDependencyVersion)
	gradleModule.SetExtractorDetails(dependencyLocalPath, filepath.Join(coreutils.GetCliPersistentTempDirPath(), utils.PropertiesTempPath), strings.Split(tasks, " "), wrapper, plugin, utils.DownloadExtractorIfNeeded, props)
	return coreutils.ConvertExitCodeError(gradleModule.CalcDependencies())
}

func createGradleRunConfig(configPath, deployableArtifactsFile string, buildConf *utils.BuildConfiguration, threads int, useWrapperIfMissingConfig, disableDeploy bool) (props map[string]string, wrapper, plugin bool, err error) {
	var vConfig *viper.Viper
	if configPath == "" {
		vConfig = viper.New()
		vConfig.SetConfigType(string(utils.YAML))
		vConfig.Set("type", utils.Gradle.String())
		vConfig.Set(useWrapper, useWrapperIfMissingConfig)
	} else {
		vConfig, err = utils.ReadConfigFile(configPath, utils.YAML)
		if err != nil {
			return
		}
	}
	wrapper = vConfig.GetBool(useWrapper)
	if threads > 0 {
		vConfig.Set(utils.ForkCount, threads)
	}

	if disableDeploy {
		setDeployFalse(vConfig)
	}
	props, err = utils.CreateBuildInfoProps(deployableArtifactsFile, vConfig, utils.Gradle)
	if err != nil {
		return
	}
	if deployableArtifactsFile != "" {
		// Save the path to a temp file, where buildinfo project will write the deployable artifacts details.
		props[utils.DeployableArtifacts] = vConfig.Get(utils.DeployableArtifacts).(string)
	}
	plugin = vConfig.GetBool(usePlugin)
	return
}

func setDeployFalse(vConfig *viper.Viper) {
	vConfig.Set(utils.DeployerPrefix+utils.DeployArtifacts, "false")
	if vConfig.GetString(utils.DeployerPrefix+utils.Url) == "" {
		vConfig.Set(utils.DeployerPrefix+utils.Url, "http://empty_url")
	}
	if vConfig.GetString(utils.DeployerPrefix+utils.Repo) == "" {
		vConfig.Set(utils.DeployerPrefix+utils.Repo, "empty_repo")
	}
}
