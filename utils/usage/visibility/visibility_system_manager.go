package visibility

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type VisibilitySystemManager struct {
	serverDetails *config.ServerDetails
}

func NewVisibilitySystemManager(serverDetails *config.ServerDetails) *VisibilitySystemManager {
	return &VisibilitySystemManager{
		serverDetails: serverDetails,
	}
}

func (vsm *VisibilitySystemManager) SendUsage(commandName string) error {
	manager, err := utils.CreateJfConnectServiceManager(vsm.serverDetails, 0, 0)
	if err != nil {
		return err
	}
	return manager.PostVisibilityMetric(newCommandsCountMetric(commandName))
}
