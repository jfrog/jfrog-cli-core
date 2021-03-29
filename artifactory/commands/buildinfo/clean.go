package buildinfo

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	coreutils "github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type BuildCleanCommand struct {
	buildConfiguration *coreutils.BuildConfiguration
}

func NewBuildCleanCommand() *BuildCleanCommand {
	return &BuildCleanCommand{}
}

func (bcc *BuildCleanCommand) SetBuildConfiguration(buildConfiguration *coreutils.BuildConfiguration) *BuildCleanCommand {
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
	err := utils.RemoveBuildDir(bcc.buildConfiguration.BuildName, bcc.buildConfiguration.BuildNumber, bcc.buildConfiguration.Project)
	if err != nil {
		return err
	}
	log.Info("Cleaned build info", bcc.buildConfiguration.BuildName+"/"+bcc.buildConfiguration.BuildNumber+".")
	return nil
}
