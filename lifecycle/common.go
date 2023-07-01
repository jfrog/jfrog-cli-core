package lifecycle

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/lifecycle"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
)

type releaseBundleCmd struct {
	serverDetails        *config.ServerDetails
	releaseBundleName    string
	releaseBundleVersion string
	signingKeyName       string
	sync                 bool
	rbProjectKey         string
}

func (rbc *releaseBundleCmd) getPrerequisites() (servicesManager *lifecycle.LifecycleServicesManager, rbDetails services.ReleaseBundleDetails, params services.CreateOrPromoteReleaseBundleParams, err error) {
	servicesManager, err = utils.CreateLifecycleServiceManager(rbc.serverDetails, false)
	if err != nil {
		return
	}
	rbDetails = services.ReleaseBundleDetails{
		ReleaseBundleName:    rbc.releaseBundleName,
		ReleaseBundleVersion: rbc.releaseBundleVersion,
	}
	params = services.CreateOrPromoteReleaseBundleParams{
		ReleaseBundleQueryParams: services.ReleaseBundleQueryParams{
			ProjectKey: rbc.rbProjectKey,
			Async:      !rbc.sync,
		},
		SigningKeyName: rbc.signingKeyName,
	}
	return
}
