package gradleutils

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/dependencies"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/spf13/viper"
	"path/filepath"
)

const (
	usePlugin  = "useplugin"
	useWrapper = "usewrapper"
)

func RunGradle(vConfig *viper.Viper, tasks []string, deployableArtifactsFile string, configuration *build.BuildConfiguration, threads int, disableDeploy bool) error {
	buildInfoService := build.CreateBuildInfoService()
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
	props, wrapper, plugin, err := createGradleRunConfig(vConfig, deployableArtifactsFile, threads, disableDeploy)
	if err != nil {
		return err
	}
	dependencyLocalPath, err := getGradleDependencyLocalPath()
	if err != nil {
		return err
	}
	gradleModule.SetExtractorDetails(dependencyLocalPath, filepath.Join(coreutils.GetCliPersistentTempDirPath(), build.PropertiesTempPath), tasks, wrapper, plugin, dependencies.DownloadExtractor, props)
	return coreutils.ConvertExitCodeError(gradleModule.CalcDependencies())
}

func getGradleDependencyLocalPath() (string, error) {
	dependenciesPath, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dependenciesPath, "gradle"), nil
}

func createGradleRunConfig(vConfig *viper.Viper, deployableArtifactsFile string, threads int, disableDeploy bool) (props map[string]string, wrapper, plugin bool, err error) {
	wrapper = vConfig.GetBool(useWrapper)
	if threads > 0 {
		vConfig.Set(build.ForkCount, threads)
	}

	if disableDeploy {
		setDeployFalse(vConfig)
	}
	props, err = build.CreateBuildInfoProps(deployableArtifactsFile, vConfig, project.Gradle)
	if err != nil {
		return
	}
	if deployableArtifactsFile != "" {
		// Save the path to a temp file, where buildinfo project will write the deployable artifacts details.
		props[build.DeployableArtifacts] = fmt.Sprint(vConfig.Get(build.DeployableArtifacts))
	}
	plugin = vConfig.GetBool(usePlugin)
	return
}

func setDeployFalse(vConfig *viper.Viper) {
	vConfig.Set(build.DeployerPrefix+build.DeployArtifacts, "false")
	if vConfig.GetString(build.DeployerPrefix+build.Url) == "" {
		vConfig.Set(build.DeployerPrefix+build.Url, "http://empty_url")
	}
	if vConfig.GetString(build.DeployerPrefix+build.Repo) == "" {
		vConfig.Set(build.DeployerPrefix+build.Repo, "empty_repo")
	}
}
