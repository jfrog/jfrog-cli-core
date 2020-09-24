package buildinfo

import (
	"errors"
	"fmt"

	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type BuildAppendCommand struct {
	buildConfiguration  *utils.BuildConfiguration
	rtDetails           *config.ArtifactoryDetails
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
	if err := bac.checkBuildToAppend(); err != nil {
		return err
	}

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

func (bac *BuildAppendCommand) SetRtDetails(rtDetails *config.ArtifactoryDetails) *BuildAppendCommand {
	bac.rtDetails = rtDetails
	return bac
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

func (bac *BuildAppendCommand) checkBuildToAppend() error {
	// Create services manager to get build-info from Artifactory.
	sm, err := utils.CreateServiceManager(bac.rtDetails, false)
	if err != nil {
		return err
	}

	// Get published build-info from Artifactory.
	buildInfoParams := services.BuildInfoParams{BuildName: bac.buildNameToAppend, BuildNumber: bac.buildNumberToAppend}
	buildInfo, err := sm.GetBuildInfo(buildInfoParams)
	if err != nil {
		return err
	}
	if buildInfo == nil || buildInfo.Name == "" {
		return errorutils.CheckError(errors.New(fmt.Sprintf("Build %s/%s does not exist", bac.buildNameToAppend, bac.buildNumberToAppend)))
	}
	return nil
}
