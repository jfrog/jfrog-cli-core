package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/distribution"
)

type DistributeReleaseBundleV1Command struct {
	serverDetails           *config.ServerDetails
	distributeBundlesParams distribution.DistributionParams
	distributionRules       *spec.DistributionRules
	sync                    bool
	maxWaitMinutes          int
	dryRun                  bool
	autoCreateRepo          bool
}

func NewReleaseBundleDistributeV1Command() *DistributeReleaseBundleV1Command {
	return &DistributeReleaseBundleV1Command{}
}

func (db *DistributeReleaseBundleV1Command) SetServerDetails(serverDetails *config.ServerDetails) *DistributeReleaseBundleV1Command {
	db.serverDetails = serverDetails
	return db
}

func (db *DistributeReleaseBundleV1Command) SetDistributeBundleParams(params distribution.DistributionParams) *DistributeReleaseBundleV1Command {
	db.distributeBundlesParams = params
	return db
}

func (db *DistributeReleaseBundleV1Command) SetDistributionRules(distributionRules *spec.DistributionRules) *DistributeReleaseBundleV1Command {
	db.distributionRules = distributionRules
	return db
}

func (db *DistributeReleaseBundleV1Command) SetSync(sync bool) *DistributeReleaseBundleV1Command {
	db.sync = sync
	return db
}

func (db *DistributeReleaseBundleV1Command) SetMaxWaitMinutes(maxWaitMinutes int) *DistributeReleaseBundleV1Command {
	db.maxWaitMinutes = maxWaitMinutes
	return db
}

func (db *DistributeReleaseBundleV1Command) SetDryRun(dryRun bool) *DistributeReleaseBundleV1Command {
	db.dryRun = dryRun
	return db
}

func (db *DistributeReleaseBundleV1Command) SetAutoCreateRepo(autoCreateRepo bool) *DistributeReleaseBundleV1Command {
	db.autoCreateRepo = autoCreateRepo
	return db
}

func (db *DistributeReleaseBundleV1Command) Run() error {
	servicesManager, err := utils.CreateDistributionServiceManager(db.serverDetails, db.dryRun)
	if err != nil {
		return err
	}

	for _, rule := range db.distributionRules.DistributionRules {
		db.distributeBundlesParams.DistributionRules = append(db.distributeBundlesParams.DistributionRules, rule.ToDistributionCommonParams())
	}

	if db.sync {
		return servicesManager.DistributeReleaseBundleSync(db.distributeBundlesParams, db.maxWaitMinutes, db.autoCreateRepo)
	}
	return servicesManager.DistributeReleaseBundle(db.distributeBundlesParams, db.autoCreateRepo)
}

func (db *DistributeReleaseBundleV1Command) ServerDetails() (*config.ServerDetails, error) {
	return db.serverDetails, nil
}

func (db *DistributeReleaseBundleV1Command) CommandName() string {
	return "rt_distribute_bundle"
}
