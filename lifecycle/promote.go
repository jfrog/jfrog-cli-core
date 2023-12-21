package lifecycle

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type ReleaseBundlePromoteCommand struct {
	releaseBundleCmd
	environment string
	overwrite   bool
}

func NewReleaseBundlePromoteCommand() *ReleaseBundlePromoteCommand {
	return &ReleaseBundlePromoteCommand{}
}

func (rbp *ReleaseBundlePromoteCommand) SetServerDetails(serverDetails *config.ServerDetails) *ReleaseBundlePromoteCommand {
	rbp.serverDetails = serverDetails
	return rbp
}

func (rbp *ReleaseBundlePromoteCommand) SetReleaseBundleName(releaseBundleName string) *ReleaseBundlePromoteCommand {
	rbp.releaseBundleName = releaseBundleName
	return rbp
}

func (rbp *ReleaseBundlePromoteCommand) SetReleaseBundleVersion(releaseBundleVersion string) *ReleaseBundlePromoteCommand {
	rbp.releaseBundleVersion = releaseBundleVersion
	return rbp
}

func (rbp *ReleaseBundlePromoteCommand) SetSigningKeyName(signingKeyName string) *ReleaseBundlePromoteCommand {
	rbp.signingKeyName = signingKeyName
	return rbp
}

func (rbp *ReleaseBundlePromoteCommand) SetSync(sync bool) *ReleaseBundlePromoteCommand {
	rbp.sync = sync
	return rbp
}

func (rbp *ReleaseBundlePromoteCommand) SetReleaseBundleProject(rbProjectKey string) *ReleaseBundlePromoteCommand {
	rbp.rbProjectKey = rbProjectKey
	return rbp
}

func (rbp *ReleaseBundlePromoteCommand) SetEnvironment(environment string) *ReleaseBundlePromoteCommand {
	rbp.environment = environment
	return rbp
}

func (rbp *ReleaseBundlePromoteCommand) SetOverwrite(overwrite bool) *ReleaseBundlePromoteCommand {
	rbp.overwrite = overwrite
	return rbp
}

func (rbp *ReleaseBundlePromoteCommand) CommandName() string {
	return "rb_promote"
}

func (rbp *ReleaseBundlePromoteCommand) ServerDetails() (*config.ServerDetails, error) {
	return rbp.serverDetails, nil
}

func (rbp *ReleaseBundlePromoteCommand) Run() error {
	if err := validateArtifactoryVersionSupported(rbp.serverDetails); err != nil {
		return err
	}

	servicesManager, rbDetails, params, err := rbp.getPrerequisites()
	if err != nil {
		return err
	}

	promotionResp, err := servicesManager.PromoteReleaseBundle(rbDetails, params, rbp.environment, rbp.overwrite)
	if err != nil {
		return err
	}
	content, err := json.Marshal(promotionResp)
	if err != nil {
		return err
	}
	log.Output(utils.IndentJson(content))
	return nil
}
