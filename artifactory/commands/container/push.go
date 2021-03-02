package container

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils/container"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

type PushCommand struct {
	ContainerManagerCommand
	threads              int
	containerManagerType container.ContainerManagerType
}

func NewPushCommand(containerManager container.ContainerManagerType) *PushCommand {
	return &PushCommand{containerManagerType: containerManager}
}

func (pc *PushCommand) Threads() int {
	return pc.threads
}

func (pc *PushCommand) SetThreads(threads int) *PushCommand {
	pc.threads = threads
	return pc
}

// Push image and create build info if needed
func (pc *PushCommand) Run() error {
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
	// Perform push.
	cm := container.NewManager(pc.containerManagerType)
	image := container.NewImage(pc.imageTag)
	err = cm.Push(image)
	if err != nil {
		return err
	}
	// Return if no build name and number was provided
	if pc.buildConfiguration.BuildName == "" || pc.buildConfiguration.BuildNumber == "" {
		return nil
	}
	if err := utils.SaveBuildGeneralDetails(pc.buildConfiguration.BuildName, pc.buildConfiguration.BuildNumber, pc.buildConfiguration.Project); err != nil {
		return err
	}
	serviceManager, err := utils.CreateServiceManagerWithThreads(serverDetails, false, pc.threads)
	if err != nil {
		return err
	}
	builder, err := container.NewBuildInfoBuilder(image, pc.Repo(), pc.BuildConfiguration().BuildName, pc.BuildConfiguration().BuildNumber, pc.BuildConfiguration().Project, serviceManager, container.Push, cm)
	if err != nil {
		return err
	}
	buildInfo, err := builder.Build(pc.BuildConfiguration().Module)
	if err != nil {
		return err
	}
	return utils.SaveBuildInfo(pc.BuildConfiguration().BuildName, pc.BuildConfiguration().BuildNumber, pc.BuildConfiguration().Project, buildInfo)
}

func (pc *PushCommand) CommandName() string {
	return "rt_docker_push"
}

func (pc *PushCommand) ServerDetails() (*config.ServerDetails, error) {
	return pc.serverDetails, nil
}
