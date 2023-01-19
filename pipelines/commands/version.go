package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type VersionCommand struct {
	serverDetails *config.ServerDetails
}

func NewVersionCommand() *VersionCommand {
	return &VersionCommand{}
}

func (vc *VersionCommand) CommandName() string {
	return "version"
}

func (vc *VersionCommand) ServerDetails() (*config.ServerDetails, error) {
	return vc.serverDetails, nil
}

func (vc *VersionCommand) SetServerDetails(serverDetails *config.ServerDetails) *VersionCommand {
	vc.serverDetails = serverDetails
	return vc
}

func (vc *VersionCommand) Run() error {
	serviceManager, err := manager.CreateServiceManager(vc.serverDetails)
	if err != nil {
		return err
	}
	info, err := serviceManager.GetSystemInfo()
	if err != nil {
		return err
	}
	if info == nil {
		log.Output("Unable to fetch pipelines version")
		return nil
	}
	log.Output("Pipelines Server Version: ", info.Version)
	return nil
}
