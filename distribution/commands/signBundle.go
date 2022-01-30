package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/distribution/services"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
)

type SignBundleCommand struct {
	serverDetails     *config.ServerDetails
	signBundlesParams services.SignBundleParams
	detailedSummary   bool
	summary           *clientutils.Sha256Summary
}

func NewReleaseBundleSignCommand() *SignBundleCommand {
	return &SignBundleCommand{}
}

func (sb *SignBundleCommand) SetServerDetails(serverDetails *config.ServerDetails) *SignBundleCommand {
	sb.serverDetails = serverDetails
	return sb
}

func (sb *SignBundleCommand) SetReleaseBundleSignParams(params services.SignBundleParams) *SignBundleCommand {
	sb.signBundlesParams = params
	return sb
}

func (sb *SignBundleCommand) Run() error {
	servicesManager, err := utils.CreateDistributionServiceManager(sb.serverDetails, false)
	if err != nil {
		return err
	}

	summary, err := servicesManager.SignReleaseBundle(sb.signBundlesParams)
	if sb.detailedSummary {
		sb.summary = summary
	}
	return err
}

func (sb *SignBundleCommand) ServerDetails() (*config.ServerDetails, error) {
	return sb.serverDetails, nil
}

func (sb *SignBundleCommand) CommandName() string {
	return "rt_sign_bundle"
}

func (sb *SignBundleCommand) SetSummary(summary *clientutils.Sha256Summary) *SignBundleCommand {
	sb.summary = summary
	return sb
}

func (sb *SignBundleCommand) GetSummary() *clientutils.Sha256Summary {
	return sb.summary
}

func (sb *SignBundleCommand) SetDetailedSummary(detailedSummary bool) *SignBundleCommand {
	sb.detailedSummary = detailedSummary
	return sb
}

func (sb *SignBundleCommand) IsDetailedSummary() bool {
	return sb.detailedSummary
}
