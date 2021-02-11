package npm

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
)

type NpmCommand struct {
	repo               string
	buildConfiguration *utils.BuildConfiguration
	npmArgs            []string
	serverDetails      *config.ServerDetails
}

func (nc *NpmCommand) SetServerDetails(serverDetails *config.ServerDetails) *NpmCommand {
	nc.serverDetails = serverDetails
	return nc
}

func (nc *NpmCommand) SetNpmArgs(npmArgs []string) *NpmCommand {
	nc.npmArgs = npmArgs
	return nc
}

func (nc *NpmCommand) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *NpmCommand {
	nc.buildConfiguration = buildConfiguration
	return nc
}

func (nc *NpmCommand) SetRepo(repo string) *NpmCommand {
	nc.repo = repo
	return nc
}
