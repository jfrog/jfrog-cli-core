package buildinfo

import (
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type BuildCollectEnvCommand struct {
	buildConfiguration *build.BuildConfiguration
}

func NewBuildCollectEnvCommand() *BuildCollectEnvCommand {
	return &BuildCollectEnvCommand{}
}

func (bcec *BuildCollectEnvCommand) SetBuildConfiguration(buildConfiguration *build.BuildConfiguration) *BuildCollectEnvCommand {
	bcec.buildConfiguration = buildConfiguration
	return bcec
}

func (bcec *BuildCollectEnvCommand) Run() error {
	log.Info("Collecting environment variables...")
	buildInfoService := build.CreateBuildInfoService()
	buildName, err := bcec.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := bcec.buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}
	build, err := buildInfoService.GetOrCreateBuildWithProject(buildName, buildNumber, bcec.buildConfiguration.GetProject())
	if err != nil {
		return errorutils.CheckError(err)
	}
	err = build.CollectEnv()
	if err != nil {
		return errorutils.CheckError(err)
	}
	log.Info("Collected environment variables for", buildName+"/"+buildNumber+".")
	return nil
}

// Returns the default configured Artifactory server
func (bcec *BuildCollectEnvCommand) ServerDetails() (*config.ServerDetails, error) {
	return config.GetDefaultServerConf()
}

func (bcec *BuildCollectEnvCommand) CommandName() string {
	return "rt_build_collect_env"
}
