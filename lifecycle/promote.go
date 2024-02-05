package lifecycle

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type ReleaseBundlePromoteCommand struct {
	releaseBundleCmd
	environment          string
	includeReposPatterns []string
	excludeReposPatterns []string
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

func (rbp *ReleaseBundlePromoteCommand) SetIncludeReposPatterns(includeReposPatterns []string) *ReleaseBundlePromoteCommand {
	rbp.includeReposPatterns = includeReposPatterns
	return rbp
}

func (rbp *ReleaseBundlePromoteCommand) SetExcludeReposPatterns(excludeReposPatterns []string) *ReleaseBundlePromoteCommand {
	rbp.excludeReposPatterns = excludeReposPatterns
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

	servicesManager, rbDetails, queryParams, err := rbp.getPrerequisites()
	if err != nil {
		return err
	}

	promotionParams := services.RbPromotionParams{
		Environment:            rbp.environment,
		IncludedRepositoryKeys: rbp.includeReposPatterns,
		ExcludedRepositoryKeys: rbp.excludeReposPatterns,
	}

	promotionResp, err := servicesManager.PromoteReleaseBundle(rbDetails, queryParams, rbp.signingKeyName, promotionParams)
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
