package container

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/container"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type SaveCommand struct {
	ContainerManagerCommand
	containerManagerType container.ContainerManagerType
	outputPath           string
}

func NewSaveCommand(containerManagerType container.ContainerManagerType, outputPath string) *SaveCommand {
	return &SaveCommand{containerManagerType: containerManagerType, outputPath: outputPath}
}

// Pull image and create build info if needed
func (sc *SaveCommand) Run() error {
	if sc.containerManagerType == container.DockerClient {
		err := container.ValidateClientApiVersion()
		if err != nil {
			return err
		}
	}
	// Perform pull.
	cm := container.NewManager(sc.containerManagerType)
	image := container.NewImage(sc.imageTag)
	return cm.Save(image, sc.outputPath)
}

func (sc *SaveCommand) CommandName() string {
	return "rt_docker_save"
}

func (sc *SaveCommand) ServerDetails() (*config.ServerDetails, error) {
	return sc.serverDetails, nil
}
