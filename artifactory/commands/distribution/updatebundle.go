package distribution

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/spec"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/distribution/services"
	distributionServicesUtils "github.com/jfrog/jfrog-client-go/distribution/services/utils"
)

type UpdateBundleCommand struct {
	serverDetails        *config.ServerDetails
	releaseBundlesParams distributionServicesUtils.ReleaseBundleParams
	spec                 *spec.SpecFiles
	dryRun               bool
}

func NewReleaseBundleUpdateCommand() *UpdateBundleCommand {
	return &UpdateBundleCommand{}
}

func (cb *UpdateBundleCommand) SetServerDetails(serverDetails *config.ServerDetails) *UpdateBundleCommand {
	cb.serverDetails = serverDetails
	return cb
}

func (cb *UpdateBundleCommand) SetReleaseBundleUpdateParams(params distributionServicesUtils.ReleaseBundleParams) *UpdateBundleCommand {
	cb.releaseBundlesParams = params
	return cb
}

func (cb *UpdateBundleCommand) SetSpec(spec *spec.SpecFiles) *UpdateBundleCommand {
	cb.spec = spec
	return cb
}

func (cb *UpdateBundleCommand) SetDryRun(dryRun bool) *UpdateBundleCommand {
	cb.dryRun = dryRun
	return cb
}

func (cb *UpdateBundleCommand) Run() error {
	servicesManager, err := utils.CreateDistributionServiceManager(cb.serverDetails, cb.dryRun)
	if err != nil {
		return err
	}

	for _, spec := range cb.spec.Files {
		cb.releaseBundlesParams.SpecFiles = append(cb.releaseBundlesParams.SpecFiles, spec.ToArtifactoryCommonParams())
	}

	params := services.UpdateReleaseBundleParams{ReleaseBundleParams: cb.releaseBundlesParams}
	return servicesManager.UpdateReleaseBundle(params)
}

func (cb *UpdateBundleCommand) ServerDetails() (*config.ServerDetails, error) {
	return cb.serverDetails, nil
}

func (cb *UpdateBundleCommand) CommandName() string {
	return "rt_bundle_update"
}
