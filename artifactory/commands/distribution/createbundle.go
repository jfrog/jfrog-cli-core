package distribution

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/spec"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/distribution/services"
	distributionServicesUtils "github.com/jfrog/jfrog-client-go/distribution/services/utils"
)

type CreateBundleCommand struct {
	serverDetails        *config.ServerDetails
	releaseBundlesParams distributionServicesUtils.ReleaseBundleParams
	spec                 *spec.SpecFiles
	dryRun               bool
}

func NewReleaseBundleCreateCommand() *CreateBundleCommand {
	return &CreateBundleCommand{}
}

func (cb *CreateBundleCommand) SetServerDetails(serverDetails *config.ServerDetails) *CreateBundleCommand {
	cb.serverDetails = serverDetails
	return cb
}

func (cb *CreateBundleCommand) SetReleaseBundleCreateParams(params distributionServicesUtils.ReleaseBundleParams) *CreateBundleCommand {
	cb.releaseBundlesParams = params
	return cb
}

func (cb *CreateBundleCommand) SetSpec(spec *spec.SpecFiles) *CreateBundleCommand {
	cb.spec = spec
	return cb
}

func (cb *CreateBundleCommand) SetDryRun(dryRun bool) *CreateBundleCommand {
	cb.dryRun = dryRun
	return cb
}

func (cb *CreateBundleCommand) Run() error {
	servicesManager, err := utils.CreateDistributionServiceManager(cb.serverDetails, cb.dryRun)
	if err != nil {
		return err
	}

	for _, spec := range cb.spec.Files {
		params, err := spec.ToArtifactoryCommonParams()
		if err != nil {
			return err
		}
		cb.releaseBundlesParams.SpecFiles = append(cb.releaseBundlesParams.SpecFiles, params)
	}

	params := services.CreateReleaseBundleParams{ReleaseBundleParams: cb.releaseBundlesParams}
	return servicesManager.CreateReleaseBundle(params)
}

func (cb *CreateBundleCommand) ServerDetails() (*config.ServerDetails, error) {
	return cb.serverDetails, nil
}

func (cb *CreateBundleCommand) CommandName() string {
	return "rt_bundle_create"
}
