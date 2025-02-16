package lifecycle

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/lifecycle"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/distribution"
)

const minimalLifecycleArtifactoryVersion = "7.63.2"

type releaseBundleCmd struct {
	serverDetails        *config.ServerDetails
	releaseBundleName    string
	releaseBundleVersion string
	sync                 bool
	rbProjectKey         string
}

func (rbc *releaseBundleCmd) getPrerequisites() (servicesManager *lifecycle.LifecycleServicesManager,
	rbDetails services.ReleaseBundleDetails, queryParams services.CommonOptionalQueryParams, err error) {
	return rbc.initPrerequisites(false, "")
}

func (rbp *ReleaseBundlePromoteCommand) getPromotionPrerequisites() (servicesManager *lifecycle.LifecycleServicesManager,
	rbDetails services.ReleaseBundleDetails, queryParams services.CommonOptionalQueryParams, err error) {
	return rbp.initPrerequisites(true, rbp.promotionType)
}

func (rb *releaseBundleCmd) initPrerequisites(isPromotion bool, promotionType string) (servicesManager *lifecycle.LifecycleServicesManager,
	rbDetails services.ReleaseBundleDetails, queryParams services.CommonOptionalQueryParams, err error) {
	servicesManager, err = utils.CreateLifecycleServiceManager(rb.serverDetails, false)
	if err != nil {
		return
	}
	rbDetails = services.ReleaseBundleDetails{
		ReleaseBundleName:    rb.releaseBundleName,
		ReleaseBundleVersion: rb.releaseBundleVersion,
	}
	queryParams = services.CommonOptionalQueryParams{
		ProjectKey: rb.rbProjectKey,
		Async:      !rb.sync,
	}

	if isPromotion {
		queryParams.PromotionType = promotionType
	}
	return
}

func validateArtifactoryVersionSupported(serverDetails *config.ServerDetails) error {
	rtServiceManager, err := utils.CreateServiceManager(serverDetails, 3, 0, false)
	if err != nil {
		return err
	}

	versionStr, err := rtServiceManager.GetVersion()
	if err != nil {
		return err
	}

	return clientUtils.ValidateMinimumVersion(clientUtils.Artifactory, versionStr, minimalLifecycleArtifactoryVersion)
}

// If distribution rules are empty, distribute to all edges.
func getAggregatedDistRules(distributionRules *spec.DistributionRules) (aggregatedRules []*distribution.DistributionCommonParams) {
	if isDistributionRulesEmpty(distributionRules) {
		aggregatedRules = append(aggregatedRules, &distribution.DistributionCommonParams{SiteName: "*"})
	} else {
		for _, rules := range distributionRules.DistributionRules {
			aggregatedRules = append(aggregatedRules, rules.ToDistributionCommonParams())
		}
	}
	return
}

func isDistributionRulesEmpty(distributionRules *spec.DistributionRules) bool {
	return distributionRules == nil ||
		len(distributionRules.DistributionRules) == 0 ||
		len(distributionRules.DistributionRules) == 1 && distributionRules.DistributionRules[0].IsEmpty()
}
