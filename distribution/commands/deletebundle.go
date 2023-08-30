package commands

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/distribution"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/distribution/services"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type DeleteReleaseBundleCommand struct {
	serverDetails       *config.ServerDetails
	deleteBundlesParams services.DeleteDistributionParams
	distributionRules   *spec.DistributionRules
	dryRun              bool
	quiet               bool
}

func NewReleaseBundleDeleteParams() *DeleteReleaseBundleCommand {
	return &DeleteReleaseBundleCommand{}
}

func (db *DeleteReleaseBundleCommand) SetServerDetails(serverDetails *config.ServerDetails) *DeleteReleaseBundleCommand {
	db.serverDetails = serverDetails
	return db
}

func (db *DeleteReleaseBundleCommand) SetDistributeBundleParams(params services.DeleteDistributionParams) *DeleteReleaseBundleCommand {
	db.deleteBundlesParams = params
	return db
}

func (db *DeleteReleaseBundleCommand) SetDistributionRules(distributionRules *spec.DistributionRules) *DeleteReleaseBundleCommand {
	db.distributionRules = distributionRules
	return db
}

func (db *DeleteReleaseBundleCommand) SetDryRun(dryRun bool) *DeleteReleaseBundleCommand {
	db.dryRun = dryRun
	return db
}

func (db *DeleteReleaseBundleCommand) SetQuiet(quiet bool) *DeleteReleaseBundleCommand {
	db.quiet = quiet
	return db
}

func (db *DeleteReleaseBundleCommand) Run() error {
	servicesManager, err := utils.CreateDistributionServiceManager(db.serverDetails, db.dryRun)
	if err != nil {
		return err
	}

	for _, spec := range db.distributionRules.DistributionRules {
		db.deleteBundlesParams.DistributionRules = append(db.deleteBundlesParams.DistributionRules, spec.ToDistributionCommonParams())
	}

	distributionRulesEmpty := db.distributionRulesEmpty()
	if !db.quiet {
		confirm, err := db.confirmDelete(distributionRulesEmpty)
		if err != nil {
			return err
		}
		if !confirm {
			return nil
		}
	}

	if distributionRulesEmpty && db.deleteBundlesParams.DeleteFromDistribution {
		return servicesManager.DeleteLocalReleaseBundle(db.deleteBundlesParams)
	}
	return servicesManager.DeleteReleaseBundle(db.deleteBundlesParams)
}

func (db *DeleteReleaseBundleCommand) ServerDetails() (*config.ServerDetails, error) {
	return db.serverDetails, nil
}

func (db *DeleteReleaseBundleCommand) CommandName() string {
	return "rt_bundle_delete"
}

// Return true iff there are no distribution rules
func (db *DeleteReleaseBundleCommand) distributionRulesEmpty() bool {
	return db.distributionRules == nil ||
		len(db.distributionRules.DistributionRules) == 0 ||
		len(db.distributionRules.DistributionRules) == 1 && db.distributionRules.DistributionRules[0].IsEmpty()
}

func (db *DeleteReleaseBundleCommand) confirmDelete(distributionRulesEmpty bool) (bool, error) {
	message := fmt.Sprintf("Are you sure you want to delete the release bundle \"%s\"/\"%s\" ", db.deleteBundlesParams.Name, db.deleteBundlesParams.Version)
	if distributionRulesEmpty && db.deleteBundlesParams.DeleteFromDistribution {
		return coreutils.AskYesNo(message+"locally from distribution?\n"+
			"You can avoid this confirmation message by adding --quiet to the command.", false), nil
	}

	var distributionRulesBodies []distribution.DistributionRulesBody
	for _, rule := range db.deleteBundlesParams.DistributionRules {
		distributionRulesBodies = append(distributionRulesBodies, distribution.DistributionRulesBody{
			SiteName:     rule.GetSiteName(),
			CityName:     rule.GetCityName(),
			CountryCodes: rule.GetCountryCodes(),
		})
	}
	bytes, err := json.Marshal(distributionRulesBodies)
	if err != nil {
		return false, errorutils.CheckError(err)
	}

	log.Output(clientutils.IndentJson(bytes))
	if db.deleteBundlesParams.DeleteFromDistribution {
		log.Output("This command will also delete the release bundle locally from distribution.")
	}
	return coreutils.AskYesNo(message+"with the above distribution rules?\n"+
		"You can avoid this confirmation message by adding --quiet to the command.", false), nil
}
