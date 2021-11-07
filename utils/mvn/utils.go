package mvnutils

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/jfrog/build-info-go/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/spf13/viper"
)

func RunMvn(configPath, deployableArtifactsFile string, buildConf *utils.BuildConfiguration, goals []string, threads int, insecureTls, disableDeploy bool) error {
	buildInfoService := utils.CreateBuildInfoService()
	mvnBuild, err := buildInfoService.GetOrCreateBuildWithProject(buildConf.BuildName, buildConf.BuildNumber, buildConf.Project)
	if err != nil {
		return errorutils.CheckError(err)
	}
	mavenModule, err := mvnBuild.AddMavenModule("")
	if err != nil {
		return errorutils.CheckError(err)
	}
	dependenciesPath, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return err
	}
	props, err := createMvnRunProps(configPath, deployableArtifactsFile, buildConf, goals, threads, insecureTls, disableDeploy)
	if err != nil {
		return err
	}
	mvnOpts := strings.Split(os.Getenv("MAVEN_OPTS"), " ")
	if v, ok := props["buildInfoConfig.artifactoryResolutionEnabled"]; ok {
		mvnOpts = append(mvnOpts, "-DbuildInfoConfig.artifactoryResolutionEnabled="+v)
	}
	dependencyLocalPath := filepath.Join(dependenciesPath, "maven", build.MavenExtractorDependencyVersion)
	mavenModule.SetExtractorDetails(dependencyLocalPath, filepath.Join(coreutils.GetCliPersistentTempDirPath(), utils.PropertiesTempPath), goals, utils.DownloadExtractorIfNeeded, props).SetMavenOpts(mvnOpts...)
	return coreutils.ConvertExitCodeError(mavenModule.CalcDependencies())
}

func createMvnRunProps(configPath, deployableArtifactsFile string, buildConf *utils.BuildConfiguration, goals []string, threads int, insecureTls, disableDeploy bool) (map[string]string, error) {
	var err error
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
	vConfig.Set(utils.InsecureTls, insecureTls)

	if threads > 0 {
		vConfig.Set(utils.ForkCount, threads)
	}

	if !vConfig.IsSet("deployer") {
		setEmptyDeployer(vConfig)
	}

	if disableDeploy {
		setDeployFalse(vConfig)
	}

	if vConfig.IsSet("resolver") {
		vConfig.Set("buildInfoConfig.artifactoryResolutionEnabled", "true")
	}
	return utils.CreateBuildInfoProps(deployableArtifactsFile, vConfig, utils.Maven)
}

func setEmptyDeployer(vConfig *viper.Viper) {
	vConfig.Set(utils.DeployerPrefix+utils.DeployArtifacts, "false")
	vConfig.Set(utils.DeployerPrefix+utils.Url, "http://empty_url")
	vConfig.Set(utils.DeployerPrefix+utils.ReleaseRepo, "empty_repo")
	vConfig.Set(utils.DeployerPrefix+utils.SnapshotRepo, "empty_repo")
}

func setDeployFalse(vConfig *viper.Viper) {
	vConfig.Set(utils.DeployerPrefix+utils.DeployArtifacts, "false")
	if vConfig.GetString(utils.DeployerPrefix+utils.Url) == "" {
		vConfig.Set(utils.DeployerPrefix+utils.Url, "http://empty_url")
	}
	if vConfig.GetString(utils.DeployerPrefix+utils.ReleaseRepo) == "" {
		vConfig.Set(utils.DeployerPrefix+utils.ReleaseRepo, "empty_repo")
	}
	if vConfig.GetString(utils.DeployerPrefix+utils.SnapshotRepo) == "" {
		vConfig.Set(utils.DeployerPrefix+utils.SnapshotRepo, "empty_repo")
	}
}
