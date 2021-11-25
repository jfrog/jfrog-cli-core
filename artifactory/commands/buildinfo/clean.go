package buildinfo

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type BuildCleanCommand struct {
	buildConfiguration *utils.BuildConfiguration
}

func NewBuildCleanCommand() *BuildCleanCommand {
	return &BuildCleanCommand{}
}

func (bcc *BuildCleanCommand) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *BuildCleanCommand {
	bcc.buildConfiguration = buildConfiguration
	return bcc
}

func (bcc *BuildCleanCommand) CommandName() string {
	return "rt_build_clean"
}

// Returns the default Artifactory server
func (bcc *BuildCleanCommand) ServerDetails() (*config.ServerDetails, error) {
	return config.GetDefaultServerConf()
}

func (bcc *BuildCleanCommand) Run() error {
	log.Info("Cleaning build info...")
	bn, err := bcc.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	err = utils.RemoveBuildDir(bn, bcc.buildConfiguration.GetBuildNumber(), bcc.buildConfiguration.GetProject())
	if err != nil {
		return err
	}
	log.Info("Cleaned build info", bn+"/"+bcc.buildConfiguration.GetBuildNumber()+".")
	return nil
}
