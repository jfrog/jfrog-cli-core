package container

import (
	"strings"

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
	if pc.containerManagerType == container.Docker {
		err := container.ValidateClientApiVersion()
		if err != nil {
			return err
		}
	}
	// Perform login
	rtDetails, err := pc.RtDetails()
	if errorutils.CheckError(err) != nil {
		return err
	}
	if !pc.skipLogin {
		loginConfig := &container.ContainerManagerLoginConfig{ArtifactoryDetails: rtDetails}
		err = container.ContainerManagerLogin(pc.imageTag, loginConfig, pc.containerManagerType)
		if err != nil {
			return err
		}
	}
	// Perform push
	if strings.LastIndex(pc.imageTag, ":") == -1 {
		pc.imageTag = pc.imageTag + ":latest"
	}
	cm := container.NewContainerManager(pc.containerManagerType)
	image := container.NewImage(pc.imageTag)
	err = cm.Push(image)
	if err != nil {
		return err
	}
	// Return if no build name and number was provided
	if pc.buildConfiguration.BuildName == "" || pc.buildConfiguration.BuildNumber == "" {
		return nil
	}
	if err := utils.SaveBuildGeneralDetails(pc.buildConfiguration.BuildName, pc.buildConfiguration.BuildNumber); err != nil {
		return err
	}
	serviceManager, err := utils.CreateServiceManagerWithThreads(rtDetails, false, pc.threads)
	if err != nil {
		return err
	}
	builder, err := container.NewBuildInfoBuilder(image, pc.Repo(), pc.BuildConfiguration().BuildName, pc.BuildConfiguration().BuildNumber, serviceManager, container.Push, cm)
	if err != nil {
		return err
	}
	buildInfo, err := builder.Build(pc.BuildConfiguration().Module)
	if err != nil {
		return err
	}
	return utils.SaveBuildInfo(pc.BuildConfiguration().BuildName, pc.BuildConfiguration().BuildNumber, buildInfo)
}

func (pc *PushCommand) CommandName() string {
	return "rt_container_push"
}

func (pc *PushCommand) RtDetails() (*config.ArtifactoryDetails, error) {
	return pc.rtDetails, nil
}
