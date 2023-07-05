package lifecycle

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type ReleaseBundleCreate struct {
	releaseBundleCmd
	buildsSpecPath         string
	releaseBundlesSpecPath string
}

func NewReleaseBundleCreate() *ReleaseBundleCreate {
	return &ReleaseBundleCreate{}
}

func (rbc *ReleaseBundleCreate) SetServerDetails(serverDetails *config.ServerDetails) *ReleaseBundleCreate {
	rbc.serverDetails = serverDetails
	return rbc
}

func (rbc *ReleaseBundleCreate) SetReleaseBundleName(releaseBundleName string) *ReleaseBundleCreate {
	rbc.releaseBundleName = releaseBundleName
	return rbc
}

func (rbc *ReleaseBundleCreate) SetReleaseBundleVersion(releaseBundleVersion string) *ReleaseBundleCreate {
	rbc.releaseBundleVersion = releaseBundleVersion
	return rbc
}

func (rbc *ReleaseBundleCreate) SetSigningKeyName(signingKeyName string) *ReleaseBundleCreate {
	rbc.signingKeyName = signingKeyName
	return rbc
}

func (rbc *ReleaseBundleCreate) SetSync(sync bool) *ReleaseBundleCreate {
	rbc.sync = sync
	return rbc
}

func (rbc *ReleaseBundleCreate) SetReleaseBundleProject(rbProjectKey string) *ReleaseBundleCreate {
	rbc.rbProjectKey = rbProjectKey
	return rbc
}

func (rbc *ReleaseBundleCreate) SetBuildsSpecPath(buildsSpecPath string) *ReleaseBundleCreate {
	rbc.buildsSpecPath = buildsSpecPath
	return rbc
}

func (rbc *ReleaseBundleCreate) SetReleaseBundlesSpecPath(releaseBundlesSpecPath string) *ReleaseBundleCreate {
	rbc.releaseBundlesSpecPath = releaseBundlesSpecPath
	return rbc
}

func (rbc *ReleaseBundleCreate) CommandName() string {
	return "rb_create"
}

func (rbc *ReleaseBundleCreate) ServerDetails() (*config.ServerDetails, error) {
	return rbc.serverDetails, nil
}

func (rbc *ReleaseBundleCreate) Run() error {
	servicesManager, rbDetails, params, err := rbc.getPrerequisites()
	if err != nil {
		return err
	}

	if rbc.buildsSpecPath != "" {
		return rbc.createFromBuilds(servicesManager, rbDetails, params)
	}
	return rbc.createFromReleaseBundles(servicesManager, rbDetails, params)
}
