package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type VersionCommand struct {
	serverDetails *config.ServerDetails
}

func NewVersionCommand() *VersionCommand {
	return &VersionCommand{}
}

func (vc *VersionCommand) ServerDetails() (*config.ServerDetails, error) {
	return vc.serverDetails, nil
}

func (vc *VersionCommand) SetServerDetails(serverDetails *config.ServerDetails) *VersionCommand {
	vc.serverDetails = serverDetails
	return vc
}

func (vc *VersionCommand) Run() (string, error) {
	serviceManager, err := manager.CreateServiceManager(vc.serverDetails)
	if err != nil {
		return "", err
	}
	info, sysInfoErr := serviceManager.GetSystemInfo()
	if err != nil {
		return "", sysInfoErr
	}
	if info == nil {
		return "", nil
	}
	return info.Version, nil
}
