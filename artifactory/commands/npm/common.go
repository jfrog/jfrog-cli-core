package npm

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

const (
	npmConfigAuthEnv       = "npm_config_%s:_auth"
	npmVersionForLegacyEnv = "9.3.1"
	npmLegacyConfigAuthEnv = "npm_config__auth"
)

type CommonArgs struct {
	repo               string
	buildConfiguration *utils.BuildConfiguration
	npmArgs            []string
	serverDetails      *config.ServerDetails
}

func (ca *CommonArgs) SetServerDetails(serverDetails *config.ServerDetails) *CommonArgs {
	ca.serverDetails = serverDetails
	return ca
}

func (ca *CommonArgs) SetNpmArgs(npmArgs []string) *CommonArgs {
	ca.npmArgs = npmArgs
	return ca
}

func (ca *CommonArgs) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *CommonArgs {
	ca.buildConfiguration = buildConfiguration
	return ca
}

func (ca *CommonArgs) SetRepo(repo string) *CommonArgs {
	ca.repo = repo
	return ca
}
