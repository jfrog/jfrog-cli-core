package distribution

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/distribution/services"
)

type SignBundleCommand struct {
	serverDetails     *config.ServerDetails
	signBundlesParams services.SignBundleParams
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

	return servicesManager.SignReleaseBundle(sb.signBundlesParams)
}

func (sb *SignBundleCommand) ServerDetails() (*config.ServerDetails, error) {
	return sb.serverDetails, nil
}

func (sb *SignBundleCommand) CommandName() string {
	return "rt_sign_bundle"
}
