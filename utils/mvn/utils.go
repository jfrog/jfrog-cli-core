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

func RunMvn(vConfig *viper.Viper, buildArtifactsDetailsFile string, buildConf *utils.BuildConfiguration,
	goals []string, threads int, insecureTls, disableDeploy bool) error {
	buildInfoService := utils.CreateBuildInfoService()
	buildName, err := buildConf.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := buildConf.GetBuildNumber()
	if err != nil {
		return err
	}
	mvnBuild, err := buildInfoService.GetOrCreateBuildWithProject(buildName, buildNumber, buildConf.GetProject())
	if err != nil {
		return errorutils.CheckError(err)
	}
	mavenModule, err := mvnBuild.AddMavenModule("")
	if err != nil {
		return errorutils.CheckError(err)
	}
	props, err := createMvnRunProps(vConfig, buildArtifactsDetailsFile, buildConf, goals, threads, insecureTls, disableDeploy)
	if err != nil {
		return err
	}
	var mvnOpts []string
	if v := os.Getenv("MAVEN_OPTS"); v != "" {
		mvnOpts = strings.Fields(v)
	}
	if v, ok := props["buildInfoConfig.artifactoryResolutionEnabled"]; ok {
		mvnOpts = append(mvnOpts, "-DbuildInfoConfig.artifactoryResolutionEnabled="+v)
	}
	dependencyLocalPath, err := getMavenDependencyLocalPath()
	if err != nil {
		return err
	}
	mavenModule.SetExtractorDetails(dependencyLocalPath, filepath.Join(coreutils.GetCliPersistentTempDirPath(), utils.PropertiesTempPath), goals, utils.DownloadExtractorIfNeeded, props).SetMavenOpts(mvnOpts...)
	return coreutils.ConvertExitCodeError(mavenModule.CalcDependencies())
}

func getMavenDependencyLocalPath() (string, error) {
	dependenciesPath, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dependenciesPath, "maven", build.MavenExtractorDependencyVersion), nil
}

func createMvnRunProps(vConfig *viper.Viper, buildArtifactsDetailsFile string, buildConf *utils.BuildConfiguration, goals []string, threads int, insecureTls, disableDeploy bool) (map[string]string, error) {
	vConfig.Set(utils.InsecureTls, insecureTls)
	if threads > 0 {
		vConfig.Set(utils.ForkCount, threads)
	}

	if disableDeploy {
		setEmptyDeployer(vConfig)
	}

	if vConfig.IsSet("resolver") {
		vConfig.Set("buildInfoConfig.artifactoryResolutionEnabled", "true")
	}
	return utils.CreateBuildInfoProps(buildArtifactsDetailsFile, vConfig, utils.Maven)
}

func setEmptyDeployer(vConfig *viper.Viper) {
	vConfig.Set(utils.DeployerPrefix+utils.DeployArtifacts, "false")
	vConfig.Set(utils.DeployerPrefix+utils.Url, "http://empty_url")
	vConfig.Set(utils.DeployerPrefix+utils.ReleaseRepo, "empty_repo")
	vConfig.Set(utils.DeployerPrefix+utils.SnapshotRepo, "empty_repo")
}
