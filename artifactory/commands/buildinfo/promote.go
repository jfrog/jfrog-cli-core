package buildinfo

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
)

type BuildPromotionCommand struct {
	services.PromotionParams
	serverDetails *config.ServerDetails
	dryRun        bool
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

func (bpc *BuildPromotionCommand) Run() error {
	servicesManager, err := utils.CreateServiceManager(bpc.serverDetails, -1, bpc.dryRun)
	if err != nil {
		return err
	}
	return servicesManager.PromoteBuild(bpc.PromotionParams)
}

func (bpc *BuildPromotionCommand) ServerDetails() (*config.ServerDetails, error) {
	return bpc.serverDetails, nil
}

func (bpc *BuildPromotionCommand) CommandName() string {
	return "rt_build_promote"
}
