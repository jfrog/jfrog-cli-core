package lifecycle

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
	"github.com/jfrog/jfrog-client-go/utils/distribution"
)

type ReleaseBundleDistributeCommand struct {
	serverDetails           *config.ServerDetails
	distributeBundlesParams distribution.DistributionParams
	distributionRules       *spec.DistributionRules
	dryRun                  bool
	autoCreateRepo          bool
	pathMappingPattern      string
	pathMappingTarget       string
}

func NewReleaseBundleDistributeCommand() *ReleaseBundleDistributeCommand {
	return &ReleaseBundleDistributeCommand{}
}

func (rbd *ReleaseBundleDistributeCommand) SetServerDetails(serverDetails *config.ServerDetails) *ReleaseBundleDistributeCommand {
	rbd.serverDetails = serverDetails
	return rbd
}

func (rbd *ReleaseBundleDistributeCommand) SetDistributeBundleParams(params distribution.DistributionParams) *ReleaseBundleDistributeCommand {
	rbd.distributeBundlesParams = params
	return rbd
}

func (rbd *ReleaseBundleDistributeCommand) SetDistributionRules(distributionRules *spec.DistributionRules) *ReleaseBundleDistributeCommand {
	rbd.distributionRules = distributionRules
	return rbd
}

func (rbd *ReleaseBundleDistributeCommand) SetDryRun(dryRun bool) *ReleaseBundleDistributeCommand {
	rbd.dryRun = dryRun
	return rbd
}

func (rbd *ReleaseBundleDistributeCommand) SetAutoCreateRepo(autoCreateRepo bool) *ReleaseBundleDistributeCommand {
	rbd.autoCreateRepo = autoCreateRepo
	return rbd
}

func (rbd *ReleaseBundleDistributeCommand) SetPathMappingPattern(pathMappingPattern string) *ReleaseBundleDistributeCommand {
	rbd.pathMappingPattern = pathMappingPattern
	return rbd
}

func (rbd *ReleaseBundleDistributeCommand) SetPathMappingTarget(pathMappingTarget string) *ReleaseBundleDistributeCommand {
	rbd.pathMappingTarget = pathMappingTarget
	return rbd
}

func (rbd *ReleaseBundleDistributeCommand) Run() error {
	if err := validateArtifactoryVersionSupported(rbd.serverDetails); err != nil {
		return err
	}

	servicesManager, err := utils.CreateLifecycleServiceManager(rbd.serverDetails, rbd.dryRun)
	if err != nil {
		return err
	}

	for _, rule := range rbd.distributionRules.DistributionRules {
		rbd.distributeBundlesParams.DistributionRules = append(rbd.distributeBundlesParams.DistributionRules, rule.ToDistributionCommonParams())
	}

	pathMapping := services.PathMapping{
		Pattern: rbd.pathMappingPattern,
		Target:  rbd.pathMappingTarget,
	}

	return servicesManager.DistributeReleaseBundle(rbd.distributeBundlesParams, rbd.autoCreateRepo, pathMapping)
}

func (rbd *ReleaseBundleDistributeCommand) ServerDetails() (*config.ServerDetails, error) {
	return rbd.serverDetails, nil
}

func (rbd *ReleaseBundleDistributeCommand) CommandName() string {
	return "rb_distribute"
}
