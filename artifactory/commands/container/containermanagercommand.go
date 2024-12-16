package container

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/container"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

// General utils for docker/podman commands
type ContainerCommand struct {
	ContainerCommandBase
	skipLogin            bool
	cmdParams            []string
	containerManagerType container.ContainerManagerType
}

func NewContainerManagerCommand(containerManagerType container.ContainerManagerType) *ContainerCommand {
	return &ContainerCommand{
		containerManagerType: containerManagerType,
	}
}

func (cm *ContainerCommand) SetSkipLogin(skipLogin bool) *ContainerCommand {
	cm.skipLogin = skipLogin
	return cm
}

func (cm *ContainerCommand) SetCmdParams(cmdParams []string) *ContainerCommand {
	cm.cmdParams = cmdParams
	return cm
}

func (cm *ContainerCommand) PerformLogin(serverDetails *config.ServerDetails, containerManagerType container.ContainerManagerType) error {
	if !cm.skipLogin {
		// Exclude refreshable tokens when working with external tools (build tools, curl, etc)
		// Otherwise refresh Token may be expireted and docker login will fail.
		if serverDetails.ServerId != "" {
			var err error
			serverDetails, err = config.GetSpecificConfig(serverDetails.ServerId, true, true)
			if err != nil {
				return err
			}
		}
		loginConfig := &container.ContainerManagerLoginConfig{ServerDetails: serverDetails}
		imageRegistry, err := cm.image.GetRegistry()
		if err != nil {
			return err
		}
		return container.ContainerManagerLogin(imageRegistry, loginConfig, containerManagerType)
	}
	return nil
}
