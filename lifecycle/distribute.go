package lifecycle

import (
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
)

type ReleaseBundleDistributeCommand struct {
	releaseBundleCmd
	distributionRules  *spec.DistributionRules
	dryRun             bool
	autoCreateRepo     bool
	pathMappingPattern string
	pathMappingTarget  string
	maxWaitMinutes     int
}

func NewReleaseBundleDistributeCommand() *ReleaseBundleDistributeCommand {
	return &ReleaseBundleDistributeCommand{}
}

func (rbd *ReleaseBundleDistributeCommand) SetServerDetails(serverDetails *config.ServerDetails) *ReleaseBundleDistributeCommand {
	rbd.serverDetails = serverDetails
	return rbd
}

func (rbd *ReleaseBundleDistributeCommand) SetReleaseBundleName(releaseBundleName string) *ReleaseBundleDistributeCommand {
	rbd.releaseBundleName = releaseBundleName
	return rbd
}

func (rbd *ReleaseBundleDistributeCommand) SetReleaseBundleVersion(releaseBundleVersion string) *ReleaseBundleDistributeCommand {
	rbd.releaseBundleVersion = releaseBundleVersion
	return rbd
}

func (rbd *ReleaseBundleDistributeCommand) SetReleaseBundleProject(rbProjectKey string) *ReleaseBundleDistributeCommand {
	rbd.rbProjectKey = rbProjectKey
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

func (rbd *ReleaseBundleDistributeCommand) SetSync(sync bool) *ReleaseBundleDistributeCommand {
	rbd.sync = sync
	return rbd
}

func (rbd *ReleaseBundleDistributeCommand) SetMaxWaitMinutes(maxWaitMinutes int) *ReleaseBundleDistributeCommand {
	rbd.maxWaitMinutes = maxWaitMinutes
	return rbd
}

func (rbd *ReleaseBundleDistributeCommand) Run() error {
	if err := validateArtifactoryVersionSupported(rbd.serverDetails); err != nil {
		return err
	}

	servicesManager, rbDetails, _, err := rbd.getPrerequisites()
	if err != nil {
		return err
	}

	pathMapping := services.PathMapping{
		Pattern: rbd.pathMappingPattern,
		Target:  rbd.pathMappingTarget,
	}

	distributeParams := services.DistributeReleaseBundleParams{
		Sync:              rbd.sync,
		AutoCreateRepo:    rbd.autoCreateRepo,
		MaxWaitMinutes:    rbd.maxWaitMinutes,
		DistributionRules: getAggregatedDistRules(rbd.distributionRules),
		PathMappings:      []services.PathMapping{pathMapping},
		ProjectKey:        rbd.rbProjectKey,
	}

	return servicesManager.DistributeReleaseBundle(rbDetails, distributeParams)
}

func (rbd *ReleaseBundleDistributeCommand) ServerDetails() (*config.ServerDetails, error) {
	return rbd.serverDetails, nil
}

func (rbd *ReleaseBundleDistributeCommand) CommandName() string {
	return "rb_distribute"
}
