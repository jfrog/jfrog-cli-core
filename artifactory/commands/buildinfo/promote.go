package buildinfo

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
)

type BuildPromotionCommand struct {
	services.PromotionParams
	buildConfiguration *build.BuildConfiguration
	serverDetails      *config.ServerDetails
	dryRun             bool
}

func NewBuildPromotionCommand() *BuildPromotionCommand {
	return &BuildPromotionCommand{}
}

func (bpc *BuildPromotionCommand) SetDryRun(dryRun bool) *BuildPromotionCommand {
	bpc.dryRun = dryRun
	return bpc
}

func (bpc *BuildPromotionCommand) SetServerDetails(serverDetails *config.ServerDetails) *BuildPromotionCommand {
	bpc.serverDetails = serverDetails
	return bpc
}

func (bpc *BuildPromotionCommand) SetPromotionParams(params services.PromotionParams) *BuildPromotionCommand {
	bpc.PromotionParams = params
	return bpc
}

func (bpc *BuildPromotionCommand) SetBuildConfiguration(buildConfiguration *build.BuildConfiguration) *BuildPromotionCommand {
	bpc.buildConfiguration = buildConfiguration
	return bpc
}

func (bpc *BuildPromotionCommand) Run() error {
	servicesManager, err := utils.CreateServiceManager(bpc.serverDetails, -1, 0, bpc.dryRun)
	if err != nil {
		return err
	}
	if err := bpc.buildConfiguration.ValidateBuildParams(); err != nil {
		return err
	}
	buildName, err := bpc.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := bpc.buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}
	bpc.BuildName, bpc.BuildNumber, bpc.ProjectKey = buildName, buildNumber, bpc.buildConfiguration.GetProject()
	return servicesManager.PromoteBuild(bpc.PromotionParams)
}

func (bpc *BuildPromotionCommand) ServerDetails() (*config.ServerDetails, error) {
	return bpc.serverDetails, nil
}

func (bpc *BuildPromotionCommand) CommandName() string {
	return "rt_build_promote"
}
