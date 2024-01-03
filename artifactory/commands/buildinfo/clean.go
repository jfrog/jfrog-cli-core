package buildinfo

import (
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type BuildCleanCommand struct {
	buildConfiguration *build.BuildConfiguration
}

func NewBuildCleanCommand() *BuildCleanCommand {
	return &BuildCleanCommand{}
}

func (bcc *BuildCleanCommand) SetBuildConfiguration(buildConfiguration *build.BuildConfiguration) *BuildCleanCommand {
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
	buildName, err := bcc.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := bcc.buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}
	err = build.RemoveBuildDir(buildName, buildNumber, bcc.buildConfiguration.GetProject())
	if err != nil {
		return err
	}
	log.Info("Cleaned build info", buildName+"/"+buildNumber+".")
	return nil
}
