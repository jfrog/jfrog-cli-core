package distribution

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/distribution/services"
)

type DistributeReleaseBundleCommand struct {
	serverDetails           *config.ServerDetails
	distributeBundlesParams services.DistributionParams
	distributionRules       *spec.DistributionRules
	sync                    bool
	maxWaitMinutes          int
	dryRun                  bool
}

func NewReleaseBundleDistributeCommand() *DistributeReleaseBundleCommand {
	return &DistributeReleaseBundleCommand{}
}

func (db *DistributeReleaseBundleCommand) SetServerDetails(serverDetails *config.ServerDetails) *DistributeReleaseBundleCommand {
	db.serverDetails = serverDetails
	return db
}

func (db *DistributeReleaseBundleCommand) SetDistributeBundleParams(params services.DistributionParams) *DistributeReleaseBundleCommand {
	db.distributeBundlesParams = params
	return db
}

func (db *DistributeReleaseBundleCommand) SetDistributionRules(distributionRules *spec.DistributionRules) *DistributeReleaseBundleCommand {
	db.distributionRules = distributionRules
	return db
}

func (db *DistributeReleaseBundleCommand) SetSync(sync bool) *DistributeReleaseBundleCommand {
	db.sync = sync
	return db
}

func (db *DistributeReleaseBundleCommand) SetMaxWaitMinutes(maxWaitMinutes int) *DistributeReleaseBundleCommand {
	db.maxWaitMinutes = maxWaitMinutes
	return db
}

func (db *DistributeReleaseBundleCommand) SetDryRun(dryRun bool) *DistributeReleaseBundleCommand {
	db.dryRun = dryRun
	return db
}

func (db *DistributeReleaseBundleCommand) Run() error {
	servicesManager, err := utils.CreateDistributionServiceManager(db.serverDetails, db.dryRun)
	if err != nil {
		return err
	}

	for _, rule := range db.distributionRules.DistributionRules {
		db.distributeBundlesParams.DistributionRules = append(db.distributeBundlesParams.DistributionRules, rule.ToDistributionCommonParams())
	}

	if db.sync {
		return servicesManager.DistributeReleaseBundleSync(db.distributeBundlesParams, db.maxWaitMinutes)
	}
	return servicesManager.DistributeReleaseBundle(db.distributeBundlesParams)
}

func (db *DistributeReleaseBundleCommand) ServerDetails() (*config.ServerDetails, error) {
	return db.serverDetails, nil
}

func (db *DistributeReleaseBundleCommand) CommandName() string {
	return "rt_distribute_bundle"
}
