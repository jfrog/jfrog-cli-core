package lifecycle

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/lifecycle"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strings"
)

type ReleaseBundleDeleteCommand struct {
	releaseBundleCmd
	environment string
	quiet       bool
}

func NewReleaseBundleDeleteCommand() *ReleaseBundleDeleteCommand {
	return &ReleaseBundleDeleteCommand{}
}

func (rbd *ReleaseBundleDeleteCommand) SetServerDetails(serverDetails *config.ServerDetails) *ReleaseBundleDeleteCommand {
	rbd.serverDetails = serverDetails
	return rbd
}

func (rbd *ReleaseBundleDeleteCommand) SetReleaseBundleName(releaseBundleName string) *ReleaseBundleDeleteCommand {
	rbd.releaseBundleName = releaseBundleName
	return rbd
}

func (rbd *ReleaseBundleDeleteCommand) SetReleaseBundleVersion(releaseBundleVersion string) *ReleaseBundleDeleteCommand {
	rbd.releaseBundleVersion = releaseBundleVersion
	return rbd
}

func (rbd *ReleaseBundleDeleteCommand) SetSync(sync bool) *ReleaseBundleDeleteCommand {
	rbd.sync = sync
	return rbd
}

func (rbd *ReleaseBundleDeleteCommand) SetReleaseBundleProject(rbProjectKey string) *ReleaseBundleDeleteCommand {
	rbd.rbProjectKey = rbProjectKey
	return rbd
}

func (rbd *ReleaseBundleDeleteCommand) SetEnvironment(environment string) *ReleaseBundleDeleteCommand {
	rbd.environment = environment
	return rbd
}

func (rbd *ReleaseBundleDeleteCommand) SetQuiet(quiet bool) *ReleaseBundleDeleteCommand {
	rbd.quiet = quiet
	return rbd
}

func (rbd *ReleaseBundleDeleteCommand) CommandName() string {
	return "rb_delete"
}

func (rbd *ReleaseBundleDeleteCommand) ServerDetails() (*config.ServerDetails, error) {
	return rbd.serverDetails, nil
}

func (rbd *ReleaseBundleDeleteCommand) Run() error {
	if err := validateArtifactoryVersionSupported(rbd.serverDetails); err != nil {
		return err
	}

	servicesManager, rbDetails, queryParams, err := rbd.getPrerequisites()
	if err != nil {
		return err
	}

	if rbd.environment != "" {
		return rbd.deletePromotionsOnly(servicesManager, rbDetails, queryParams)
	}
	return rbd.deleteLocalReleaseBundle(servicesManager, rbDetails, queryParams)
}

func (rbd *ReleaseBundleDeleteCommand) deletePromotionsOnly(servicesManager *lifecycle.LifecycleServicesManager,
	rbDetails services.ReleaseBundleDetails, commonQueryParams services.CommonOptionalQueryParams) error {

	deletionSubject := fmt.Sprintf("all promotions to environment '%s' of release bundle '%s/%s'", rbd.environment, rbd.releaseBundleName, rbd.releaseBundleVersion)
	if !rbd.confirmDelete(deletionSubject) {
		return nil
	}

	optionalQueryParams := services.GetPromotionsOptionalQueryParams{ProjectKey: commonQueryParams.ProjectKey}
	response, err := servicesManager.GetReleaseBundleVersionPromotions(rbDetails, optionalQueryParams)
	if err != nil {
		return err
	}
	success := 0
	fail := 0
	for _, promotion := range response.Promotions {
		if strings.EqualFold(promotion.Environment, rbd.environment) {
			if curErr := servicesManager.DeleteReleaseBundleVersionPromotion(rbDetails, commonQueryParams, promotion.CreatedMillis.String()); curErr != nil {
				err = errors.Join(err, curErr)
				fail++
			} else {
				success++
			}
		}
	}
	if success == 0 && fail == 0 {
		log.Info(fmt.Sprintf("No promotions were found for environment '%s'", rbd.environment))
	} else {
		log.Info(fmt.Sprintf("Promotions deleted successfully: %d, failed: %d", success, fail))
	}

	return err
}

func (rbd *ReleaseBundleDeleteCommand) deleteLocalReleaseBundle(servicesManager *lifecycle.LifecycleServicesManager,
	rbDetails services.ReleaseBundleDetails, queryParams services.CommonOptionalQueryParams) error {
	deletionSubject := fmt.Sprintf("release bundle '%s/%s' locally with all its promotions", rbd.releaseBundleName, rbd.releaseBundleVersion)
	if !rbd.confirmDelete(deletionSubject) {
		return nil
	}
	return servicesManager.DeleteReleaseBundleVersion(rbDetails, queryParams)
}

func (rbd *ReleaseBundleDeleteCommand) confirmDelete(deletionSubject string) bool {
	if rbd.quiet {
		return true
	}
	return coreutils.AskYesNo(
		fmt.Sprintf("Are you sure you want to delete %s?\n"+avoidConfirmationMsg, deletionSubject), false)
}
