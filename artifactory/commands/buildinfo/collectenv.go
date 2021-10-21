package buildinfo

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type BuildCollectEnvCommand struct {
	buildConfiguration *utils.BuildConfiguration
}

func NewBuildCollectEnvCommand() *BuildCollectEnvCommand {
	return &BuildCollectEnvCommand{}
}

func (bcec *BuildCollectEnvCommand) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *BuildCollectEnvCommand {
	bcec.buildConfiguration = buildConfiguration
	return bcec
}

func (bcec *BuildCollectEnvCommand) Run() error {
	log.Info("Collecting environment variables...")
	buildInfoService := utils.CreateBuildInfoService()
	build, err := buildInfoService.GetOrCreateBuildWithProject(bcec.buildConfiguration.BuildName, bcec.buildConfiguration.BuildNumber, bcec.buildConfiguration.Project)
	if err != nil {
		return errorutils.CheckError(err)
	}
	err = build.CollectEnv()
	if err != nil {
		return errorutils.CheckError(err)
	}
	log.Info("Collected environment variables for", bcec.buildConfiguration.BuildName+"/"+bcec.buildConfiguration.BuildNumber+".")
	return nil
}

// Returns the default configured Artifactory server
func (bcec *BuildCollectEnvCommand) ServerDetails() (*config.ServerDetails, error) {
	return config.GetDefaultServerConf()
}

func (bcec *BuildCollectEnvCommand) CommandName() string {
	return "rt_build_collect_env"
}
