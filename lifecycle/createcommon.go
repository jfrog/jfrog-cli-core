package lifecycle

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type ReleaseBundleCreateCommand struct {
	releaseBundleCmd
	buildsSpecPath         string
	releaseBundlesSpecPath string
}

func NewReleaseBundleCreateCommand() *ReleaseBundleCreateCommand {
	return &ReleaseBundleCreateCommand{}
}

func (rbc *ReleaseBundleCreateCommand) SetServerDetails(serverDetails *config.ServerDetails) *ReleaseBundleCreateCommand {
	rbc.serverDetails = serverDetails
	return rbc
}

func (rbc *ReleaseBundleCreateCommand) SetReleaseBundleName(releaseBundleName string) *ReleaseBundleCreateCommand {
	rbc.releaseBundleName = releaseBundleName
	return rbc
}

func (rbc *ReleaseBundleCreateCommand) SetReleaseBundleVersion(releaseBundleVersion string) *ReleaseBundleCreateCommand {
	rbc.releaseBundleVersion = releaseBundleVersion
	return rbc
}

func (rbc *ReleaseBundleCreateCommand) SetSigningKeyName(signingKeyName string) *ReleaseBundleCreateCommand {
	rbc.signingKeyName = signingKeyName
	return rbc
}

func (rbc *ReleaseBundleCreateCommand) SetSync(sync bool) *ReleaseBundleCreateCommand {
	rbc.sync = sync
	return rbc
}

func (rbc *ReleaseBundleCreateCommand) SetReleaseBundleProject(rbProjectKey string) *ReleaseBundleCreateCommand {
	rbc.rbProjectKey = rbProjectKey
	return rbc
}

func (rbc *ReleaseBundleCreateCommand) SetBuildsSpecPath(buildsSpecPath string) *ReleaseBundleCreateCommand {
	rbc.buildsSpecPath = buildsSpecPath
	return rbc
}

func (rbc *ReleaseBundleCreateCommand) SetReleaseBundlesSpecPath(releaseBundlesSpecPath string) *ReleaseBundleCreateCommand {
	rbc.releaseBundlesSpecPath = releaseBundlesSpecPath
	return rbc
}

func (rbc *ReleaseBundleCreateCommand) CommandName() string {
	return "rb_create"
}

func (rbc *ReleaseBundleCreateCommand) ServerDetails() (*config.ServerDetails, error) {
	return rbc.serverDetails, nil
}

func (rbc *ReleaseBundleCreateCommand) Run() error {
	if err := validateArtifactoryVersionSupported(rbc.serverDetails); err != nil {
		return err
	}

	servicesManager, rbDetails, queryParams, err := rbc.getPrerequisites()
	if err != nil {
		return err
	}

	if rbc.buildsSpecPath != "" {
		return rbc.createFromBuilds(servicesManager, rbDetails, queryParams)
	}
	return rbc.createFromReleaseBundles(servicesManager, rbDetails, queryParams)
}
