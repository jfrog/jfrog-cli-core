package lifecycle

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/lifecycle"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
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
	servicesManager, err = utils.CreateLifecycleServiceManager(rbc.serverDetails, false)
	if err != nil {
		return
	}
	rbDetails = services.ReleaseBundleDetails{
		ReleaseBundleName:    rbc.releaseBundleName,
		ReleaseBundleVersion: rbc.releaseBundleVersion,
	}
	queryParams = services.CommonOptionalQueryParams{
		ProjectKey: rbc.rbProjectKey,
		Async:      !rbc.sync,
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
