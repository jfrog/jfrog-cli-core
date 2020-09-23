package buildinfo

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type BuildAppendCommand struct {
	buildConfiguration  *utils.BuildConfiguration
	buildNameToAppend   string
	buildNumberToAppend string
}

func NewBuildAppendCommand() *BuildAppendCommand {
	return &BuildAppendCommand{}
}

func (bac *BuildAppendCommand) CommandName() string {
	return "rt_build_append"
}

func (bac *BuildAppendCommand) RtDetails() (*config.ArtifactoryDetails, error) {
	return config.GetDefaultArtifactoryConf()
}

func (bac *BuildAppendCommand) Run() error {
	log.Info("Running Build Append command...")
	if err := utils.SaveBuildGeneralDetails(bac.buildConfiguration.BuildName, bac.buildConfiguration.BuildNumber); err != nil {
		return err
	}

	log.Debug("Appending build", bac.buildNameToAppend+"/"+bac.buildNumberToAppend, "to build info")
	populateFunc := func(partial *buildinfo.Partial) {
		partial.ModuleType = buildinfo.Build
		partial.ModuleId = bac.buildNameToAppend + "/" + bac.buildNumberToAppend
	}
	return utils.SavePartialBuildInfo(bac.buildConfiguration.BuildName, bac.buildConfiguration.BuildNumber, populateFunc)
}

func (bac *BuildAppendCommand) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *BuildAppendCommand {
	bac.buildConfiguration = buildConfiguration
	return bac
}

func (bac *BuildAppendCommand) SetBuildNameToAppend(buildName string) *BuildAppendCommand {
	bac.buildNameToAppend = buildName
	return bac
}

func (bac *BuildAppendCommand) SetBuildNumberToAppend(buildNumber string) *BuildAppendCommand {
	bac.buildNumberToAppend = buildNumber
	return bac
}
