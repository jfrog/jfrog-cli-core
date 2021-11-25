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
	bn, err := bcec.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	build, err := buildInfoService.GetOrCreateBuildWithProject(bn, bcec.buildConfiguration.GetBuildNumber(), bcec.buildConfiguration.GetProject())
	if err != nil {
		return errorutils.CheckError(err)
	}
	err = build.CollectEnv()
	if err != nil {
		return errorutils.CheckError(err)
	}
	log.Info("Collected environment variables for", bn+"/"+bcec.buildConfiguration.GetBuildNumber()+".")
	return nil
}

// Returns the default configured Artifactory server
func (bcec *BuildCollectEnvCommand) ServerDetails() (*config.ServerDetails, error) {
	return config.GetDefaultServerConf()
}

func (bcec *BuildCollectEnvCommand) CommandName() string {
	return "rt_build_collect_env"
}
