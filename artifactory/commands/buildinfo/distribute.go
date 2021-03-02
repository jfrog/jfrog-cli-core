package buildinfo

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
)

type BuildDistributeCommnad struct {
	serverDetails *config.ServerDetails
	services.BuildDistributionParams
	dryRun bool
}

func NewBuildDistributeCommnad() *BuildDistributeCommnad {
	return &BuildDistributeCommnad{}
}

func (bdc *BuildDistributeCommnad) SetServerDetails(serverDetails *config.ServerDetails) *BuildDistributeCommnad {
	bdc.serverDetails = serverDetails
	return bdc
}

func (bdc *BuildDistributeCommnad) SetDryRun(dryRun bool) *BuildDistributeCommnad {
	bdc.dryRun = dryRun
	return bdc
}

func (bdc *BuildDistributeCommnad) SetBuildDistributionParams(buildDistributeParams services.BuildDistributionParams) *BuildDistributeCommnad {
	bdc.BuildDistributionParams = buildDistributeParams
	return bdc
}

func (bdc *BuildDistributeCommnad) Run() error {
	servicesManager, err := utils.CreateServiceManager(bdc.serverDetails, bdc.dryRun)
	if err != nil {
		return err
	}
	return servicesManager.DistributeBuild(bdc.BuildDistributionParams)
}

func (bdc *BuildDistributeCommnad) ServerDetails() (*config.ServerDetails, error) {
	return bdc.serverDetails, nil
}

func (bdc *BuildDistributeCommnad) CommandName() string {
	return "rt_build_distribute"
}
