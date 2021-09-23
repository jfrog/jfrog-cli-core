package container

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/container"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

type PullCommand struct {
	ContainerManagerCommand
	containerManagerType container.ContainerManagerType
}

func NewPullCommand(containerManagerType container.ContainerManagerType) *PullCommand {
	return &PullCommand{containerManagerType: containerManagerType}
}

// Pull image and create build info if needed
func (pc *PullCommand) Run() error {
	if pc.containerManagerType == container.DockerClient {
		err := container.ValidateClientApiVersion()
		if err != nil {
			return err
		}
	}
	serverDetails, err := pc.ServerDetails()
	if errorutils.CheckError(err) != nil {
		return err
	}
	// Perform login
	if err := pc.PerformLogin(serverDetails, pc.containerManagerType); err != nil {
		return err
	}
	// Perform pull.
	cm := container.NewManager(pc.containerManagerType)
	image := container.NewImage(pc.imageTag)
	err = cm.Pull(image)
	if err != nil {
		return err
	}
	buildName := pc.BuildConfiguration().BuildName
	buildNumber := pc.BuildConfiguration().BuildNumber
	project := pc.BuildConfiguration().Project
	// Return if no build name and number was provided
	if buildName == "" || buildNumber == "" {
		return nil
	}
	if err := utils.SaveBuildGeneralDetails(buildName, buildNumber, project); err != nil {
		return err
	}
	serviceManager, err := utils.CreateServiceManager(serverDetails, -1, false)
	if err != nil {
		return err
	}
	builder, err := container.NewBuildInfoBuilderForDockerOrPodman(image, pc.Repo(), buildName, buildNumber, project, serviceManager, container.Pull, cm)
	if err != nil {
		return err
	}
	buildInfo, err := builder.Build(pc.BuildConfiguration().Module)
	if err != nil {
		return err
	}
	return utils.SaveBuildInfo(buildName, buildNumber, project, buildInfo)
}

func (pc *PullCommand) CommandName() string {
	return "rt_docker_pull"
}

func (pc *PullCommand) ServerDetails() (*config.ServerDetails, error) {
	return pc.serverDetails, nil
}
