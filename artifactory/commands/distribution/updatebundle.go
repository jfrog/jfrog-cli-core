package distribution

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/distribution/services"
	distributionServicesUtils "github.com/jfrog/jfrog-client-go/distribution/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
)

type UpdateBundleCommand struct {
	serverDetails        *config.ServerDetails
	releaseBundlesParams distributionServicesUtils.ReleaseBundleParams
	spec                 *spec.SpecFiles
	dryRun               bool
	detailedSummary      bool
	summary              *clientutils.Sha256Summary
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
		params, err := spec.ToCommonParams()
		if err != nil {
			return err
		}
		recursive, err := spec.IsRecursive(true)
		if err != nil {
			return err
		}
		params.Recursive = recursive
		cb.releaseBundlesParams.SpecFiles = append(cb.releaseBundlesParams.SpecFiles, params)
	}

	params := services.UpdateReleaseBundleParams{ReleaseBundleParams: cb.releaseBundlesParams}
	summary, err := servicesManager.UpdateReleaseBundle(params)
	if cb.detailedSummary {
		cb.summary = summary
	}
	return err
}

func (cb *UpdateBundleCommand) ServerDetails() (*config.ServerDetails, error) {
	return cb.serverDetails, nil
}

func (cb *UpdateBundleCommand) CommandName() string {
	return "rt_bundle_update"
}

func (cb *UpdateBundleCommand) SetSummary(summary *clientutils.Sha256Summary) *UpdateBundleCommand {
	cb.summary = summary
	return cb
}

func (cb *UpdateBundleCommand) GetSummary() *clientutils.Sha256Summary {
	return cb.summary
}

func (cb *UpdateBundleCommand) SetDetailedSummary(detailedSummary bool) *UpdateBundleCommand {
	cb.detailedSummary = detailedSummary
	return cb
}

func (cb *UpdateBundleCommand) IsDetailedSummary() bool {
	return cb.detailedSummary
}

func (cb *UpdateBundleCommand) IsSignImmediately() bool {
	return cb.releaseBundlesParams.SignImmediately
}
