package buildinfo

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
)

type BuildDiscardCommand struct {
	serverDetails *config.ServerDetails
	services.DiscardBuildsParams
}

func NewBuildDiscardCommand() *BuildDiscardCommand {
	return &BuildDiscardCommand{}
}

func (buildDiscard *BuildDiscardCommand) SetServerDetails(serverDetails *config.ServerDetails) *BuildDiscardCommand {
	buildDiscard.serverDetails = serverDetails
	return buildDiscard
}

func (buildDiscard *BuildDiscardCommand) SetDiscardBuildsParams(params services.DiscardBuildsParams) *BuildDiscardCommand {
	buildDiscard.DiscardBuildsParams = params
	return buildDiscard
}

func (buildDiscard *BuildDiscardCommand) Run() error {
	servicesManager, err := utils.CreateServiceManager(buildDiscard.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}
	return servicesManager.DiscardBuilds(buildDiscard.DiscardBuildsParams)
}

func (buildDiscard *BuildDiscardCommand) ServerDetails() (*config.ServerDetails, error) {
	return buildDiscard.serverDetails, nil
}

func (buildDiscard *BuildDiscardCommand) CommandName() string {
	return "rt_build_discard"
}
