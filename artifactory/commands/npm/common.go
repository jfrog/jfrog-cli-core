package npm

import (
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type CommonArgs struct {
	repo               string
	buildConfiguration *build.BuildConfiguration
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

func (ca *CommonArgs) SetBuildConfiguration(buildConfiguration *build.BuildConfiguration) *CommonArgs {
	ca.buildConfiguration = buildConfiguration
	return ca
}

func (ca *CommonArgs) SetRepo(repo string) *CommonArgs {
	ca.repo = repo
	return ca
}
