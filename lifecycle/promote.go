package lifecycle

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type ReleaseBundlePromote struct {
	releaseBundleCmd
	environment string
	overwrite   bool
}

func NewReleaseBundlePromote() *ReleaseBundlePromote {
	return &ReleaseBundlePromote{}
}

func (rbp *ReleaseBundlePromote) SetServerDetails(serverDetails *config.ServerDetails) *ReleaseBundlePromote {
	rbp.serverDetails = serverDetails
	return rbp
}

func (rbp *ReleaseBundlePromote) SetReleaseBundleName(releaseBundleName string) *ReleaseBundlePromote {
	rbp.releaseBundleName = releaseBundleName
	return rbp
}

func (rbp *ReleaseBundlePromote) SetReleaseBundleVersion(releaseBundleVersion string) *ReleaseBundlePromote {
	rbp.releaseBundleVersion = releaseBundleVersion
	return rbp
}

func (rbp *ReleaseBundlePromote) SetSigningKeyName(signingKeyName string) *ReleaseBundlePromote {
	rbp.signingKeyName = signingKeyName
	return rbp
}

func (rbp *ReleaseBundlePromote) SetSync(sync bool) *ReleaseBundlePromote {
	rbp.sync = sync
	return rbp
}

func (rbp *ReleaseBundlePromote) SetReleaseBundleProject(rbProjectKey string) *ReleaseBundlePromote {
	rbp.rbProjectKey = rbProjectKey
	return rbp
}

func (rbp *ReleaseBundlePromote) SetEnvironment(environment string) *ReleaseBundlePromote {
	rbp.environment = environment
	return rbp
}

func (rbp *ReleaseBundlePromote) SetOverwrite(overwrite bool) *ReleaseBundlePromote {
	rbp.overwrite = overwrite
	return rbp
}

func (rbp *ReleaseBundlePromote) CommandName() string {
	return "rb_promote"
}

func (rbp *ReleaseBundlePromote) ServerDetails() (*config.ServerDetails, error) {
	return rbp.serverDetails, nil
}

func (rbp *ReleaseBundlePromote) Run() error {
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
