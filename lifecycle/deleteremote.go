package lifecycle

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/lifecycle"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/distribution"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const avoidConfirmationMsg = "You can avoid this confirmation message by adding --quiet to the command."

type ReleaseBundleRemoteDeleteCommand struct {
	releaseBundleCmd
	distributionRules *spec.DistributionRules
	dryRun            bool
	quiet             bool
	maxWaitMinutes    int
}

func NewReleaseBundleRemoteDeleteCommand() *ReleaseBundleRemoteDeleteCommand {
	return &ReleaseBundleRemoteDeleteCommand{}
}

func (rbd *ReleaseBundleRemoteDeleteCommand) SetServerDetails(serverDetails *config.ServerDetails) *ReleaseBundleRemoteDeleteCommand {
	rbd.serverDetails = serverDetails
	return rbd
}

func (rbd *ReleaseBundleRemoteDeleteCommand) SetReleaseBundleName(releaseBundleName string) *ReleaseBundleRemoteDeleteCommand {
	rbd.releaseBundleName = releaseBundleName
	return rbd
}

func (rbd *ReleaseBundleRemoteDeleteCommand) SetReleaseBundleVersion(releaseBundleVersion string) *ReleaseBundleRemoteDeleteCommand {
	rbd.releaseBundleVersion = releaseBundleVersion
	return rbd
}

func (rbd *ReleaseBundleRemoteDeleteCommand) SetSync(sync bool) *ReleaseBundleRemoteDeleteCommand {
	rbd.sync = sync
	return rbd
}

func (rbd *ReleaseBundleRemoteDeleteCommand) SetReleaseBundleProject(rbProjectKey string) *ReleaseBundleRemoteDeleteCommand {
	rbd.rbProjectKey = rbProjectKey
	return rbd
}

func (rbd *ReleaseBundleRemoteDeleteCommand) SetDistributionRules(distributionRules *spec.DistributionRules) *ReleaseBundleRemoteDeleteCommand {
	rbd.distributionRules = distributionRules
	return rbd
}

func (rbd *ReleaseBundleRemoteDeleteCommand) SetDryRun(dryRun bool) *ReleaseBundleRemoteDeleteCommand {
	rbd.dryRun = dryRun
	return rbd
}

func (rbd *ReleaseBundleRemoteDeleteCommand) SetQuiet(quiet bool) *ReleaseBundleRemoteDeleteCommand {
	rbd.quiet = quiet
	return rbd
}

func (rbd *ReleaseBundleRemoteDeleteCommand) SetMaxWaitMinutes(maxWaitMinutes int) *ReleaseBundleRemoteDeleteCommand {
	rbd.maxWaitMinutes = maxWaitMinutes
	return rbd
}

func (rbd *ReleaseBundleRemoteDeleteCommand) CommandName() string {
	return "rb_remote_delete"
}

func (rbd *ReleaseBundleRemoteDeleteCommand) ServerDetails() (*config.ServerDetails, error) {
	return rbd.serverDetails, nil
}

func (rbd *ReleaseBundleRemoteDeleteCommand) Run() error {
	if err := validateArtifactoryVersionSupported(rbd.serverDetails); err != nil {
		return err
	}

	servicesManager, rbDetails, queryParams, err := rbd.getPrerequisites()
	if err != nil {
		return err
	}

	return rbd.deleteRemote(servicesManager, rbDetails, queryParams)
}

func (rbd *ReleaseBundleRemoteDeleteCommand) deleteRemote(servicesManager *lifecycle.LifecycleServicesManager,
	rbDetails services.ReleaseBundleDetails, queryParams services.CommonOptionalQueryParams) error {

	confirm, err := rbd.confirmDelete()
	if err != nil || !confirm {
		return err
	}

	aggregatedRules := rbd.getAggregatedDistRules()

	return servicesManager.RemoteDeleteReleaseBundle(rbDetails, services.ReleaseBundleRemoteDeleteParams{
		DistributionRules:         aggregatedRules,
		DryRun:                    rbd.dryRun,
		MaxWaitMinutes:            rbd.maxWaitMinutes,
		CommonOptionalQueryParams: queryParams,
	})
}

func (rbd *ReleaseBundleRemoteDeleteCommand) distributionRulesEmpty() bool {
	return rbd.distributionRules == nil ||
		len(rbd.distributionRules.DistributionRules) == 0 ||
		len(rbd.distributionRules.DistributionRules) == 1 && rbd.distributionRules.DistributionRules[0].IsEmpty()
}

func (rbd *ReleaseBundleRemoteDeleteCommand) confirmDelete() (bool, error) {
	if rbd.quiet {
		return true, nil
	}

	message := fmt.Sprintf("Are you sure you want to delete the release bundle '%s/%s' remotely ", rbd.releaseBundleName, rbd.releaseBundleVersion)
	if rbd.distributionRulesEmpty() {
		message += "from all edges?"
	} else {
		var distributionRulesBodies []distribution.DistributionRulesBody
		for _, rule := range rbd.distributionRules.DistributionRules {
			distributionRulesBodies = append(distributionRulesBodies, distribution.DistributionRulesBody{
				SiteName:     rule.SiteName,
				CityName:     rule.CityName,
				CountryCodes: rule.CountryCodes,
			})
		}
		bytes, err := json.Marshal(distributionRulesBodies)
		if err != nil {
			return false, errorutils.CheckError(err)
		}

		log.Output(clientutils.IndentJson(bytes))
		message += "from all edges with the above distribution rules?"
	}

	return coreutils.AskYesNo(message+"\n"+avoidConfirmationMsg, false), nil
}

func (rbd *ReleaseBundleRemoteDeleteCommand) getAggregatedDistRules() (aggregatedRules []*distribution.DistributionCommonParams) {
	if rbd.distributionRulesEmpty() {
		aggregatedRules = append(aggregatedRules, &distribution.DistributionCommonParams{SiteName: "*"})
	} else {
		for _, rules := range rbd.distributionRules.DistributionRules {
			aggregatedRules = append(aggregatedRules, rules.ToDistributionCommonParams())
		}
	}
	return
}
