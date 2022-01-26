package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/distribution/services"
	distributionServicesUtils "github.com/jfrog/jfrog-client-go/distribution/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
)

type CreateBundleCommand struct {
	serverDetails        *config.ServerDetails
	releaseBundlesParams distributionServicesUtils.ReleaseBundleParams
	spec                 *spec.SpecFiles
	dryRun               bool
	detailedSummary      bool
	summary              *clientutils.Sha256Summary
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

	params := services.CreateReleaseBundleParams{ReleaseBundleParams: cb.releaseBundlesParams}
	summary, err := servicesManager.CreateReleaseBundle(params)
	if cb.detailedSummary {
		cb.summary = summary
	}
	return err
}

func (cb *CreateBundleCommand) ServerDetails() (*config.ServerDetails, error) {
	return cb.serverDetails, nil
}

func (cb *CreateBundleCommand) CommandName() string {
	return "rt_bundle_create"
}

func (cb *CreateBundleCommand) SetSummary(summary *clientutils.Sha256Summary) *CreateBundleCommand {
	cb.summary = summary
	return cb
}

func (cb *CreateBundleCommand) GetSummary() *clientutils.Sha256Summary {
	return cb.summary
}

func (cb *CreateBundleCommand) SetDetailedSummary(detailedSummary bool) *CreateBundleCommand {
	cb.detailedSummary = detailedSummary
	return cb
}

func (cb *CreateBundleCommand) IsDetailedSummary() bool {
	return cb.detailedSummary
}

func (cb *CreateBundleCommand) IsSignImmediately() bool {
	return cb.releaseBundlesParams.SignImmediately
}
