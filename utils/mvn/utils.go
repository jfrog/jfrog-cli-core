package mvnutils

import (
	"io"
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

type MvnUtils struct {
	vConfig                   *viper.Viper
	buildConf                 *utils.BuildConfiguration
	buildArtifactsDetailsFile string
	goals                     []string
	threads                   int
	insecureTls               bool
	disableDeploy             bool
	outputWriter              io.Writer
}

func NewMvnUtils() *MvnUtils {
	return &MvnUtils{buildConf: &utils.BuildConfiguration{}}
}

func (mu *MvnUtils) SetBuildConf(buildConf *utils.BuildConfiguration) *MvnUtils {
	mu.buildConf = buildConf
	return mu
}

func (mu *MvnUtils) SetBuildArtifactsDetailsFile(buildArtifactsDetailsFile string) *MvnUtils {
	mu.buildArtifactsDetailsFile = buildArtifactsDetailsFile
	return mu
}

func (mu *MvnUtils) SetGoals(goals []string) *MvnUtils {
	mu.goals = goals
	return mu
}

func (mu *MvnUtils) SetThreads(threads int) *MvnUtils {
	mu.threads = threads
	return mu
}

func (mu *MvnUtils) SetInsecureTls(insecureTls bool) *MvnUtils {
	mu.insecureTls = insecureTls
	return mu
}

func (mu *MvnUtils) SetDisableDeploy(disableDeploy bool) *MvnUtils {
	mu.disableDeploy = disableDeploy
	return mu
}

func (mu *MvnUtils) SetConfig(vConfig *viper.Viper) *MvnUtils {
	mu.vConfig = vConfig
	return mu
}

func (mu *MvnUtils) SetOutputWriter(writer io.Writer) *MvnUtils {
	mu.outputWriter = writer
	return mu
}

func RunMvn(mu *MvnUtils) error {
	buildInfoService := utils.CreateBuildInfoService()
	buildName, err := mu.buildConf.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := mu.buildConf.GetBuildNumber()
	if err != nil {
		return err
	}
	mvnBuild, err := buildInfoService.GetOrCreateBuildWithProject(buildName, buildNumber, mu.buildConf.GetProject())
	if err != nil {
		return errorutils.CheckError(err)
	}
	mavenModule, err := mvnBuild.AddMavenModule("")
	if err != nil {
		return errorutils.CheckError(err)
	}
	props, useWrapper, err := createMvnRunProps(mu.vConfig, mu.buildArtifactsDetailsFile, mu.threads, mu.insecureTls, mu.disableDeploy)
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
	mavenModule.SetExtractorDetails(dependencyLocalPath,
		filepath.Join(coreutils.GetCliPersistentTempDirPath(), utils.PropertiesTempPath),
		mu.goals,
		utils.DownloadExtractorIfNeeded,
		props,
		useWrapper).
		SetOutputWriter(mu.outputWriter)
	mavenModule.SetMavenOpts(mvnOpts...)
	return coreutils.ConvertExitCodeError(mavenModule.CalcDependencies())
}

func getMavenDependencyLocalPath() (string, error) {
	dependenciesPath, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dependenciesPath, "maven", build.MavenExtractorDependencyVersion), nil
}

func createMvnRunProps(vConfig *viper.Viper, buildArtifactsDetailsFile string, threads int, insecureTls, disableDeploy bool) (props map[string]string, useWrapper bool, err error) {
	useWrapper = vConfig.GetBool("useWrapper")
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
	buildInfoProps, err := utils.CreateBuildInfoProps(buildArtifactsDetailsFile, vConfig, utils.Maven)

	return buildInfoProps, useWrapper, err
}

func setEmptyDeployer(vConfig *viper.Viper) {
	vConfig.Set(utils.DeployerPrefix+utils.DeployArtifacts, "false")
	vConfig.Set(utils.DeployerPrefix+utils.Url, "http://empty_url")
	vConfig.Set(utils.DeployerPrefix+utils.ReleaseRepo, "empty_repo")
	vConfig.Set(utils.DeployerPrefix+utils.SnapshotRepo, "empty_repo")
}
