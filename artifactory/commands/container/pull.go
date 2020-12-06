package container

import (
	"strings"

	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils/container"
	"github.com/jfrog/jfrog-cli-core/utils/config"
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
	// Perform pull.
	// Skip port colon.
	imagePath := pc.imageTag[strings.Index(pc.imageTag, "/"):]
	if strings.LastIndex(imagePath, ":") == -1 {
		pc.imageTag = pc.imageTag + ":latest"
	}
	cm := container.NewContainerManager(pc.containerManagerType)
	image := container.NewImage(pc.imageTag)
	err = cm.Pull(image)
	if err != nil {
		return err
	}
	buildName := pc.BuildConfiguration().BuildName
	buildNumber := pc.BuildConfiguration().BuildNumber
	// Return if no build name and number was provided
	if buildName == "" || buildNumber == "" {
		return nil
	}
	if err := utils.SaveBuildGeneralDetails(buildName, buildNumber); err != nil {
		return err
	}
	serviceManager, err := utils.CreateServiceManager(rtDetails, false)
	if err != nil {
		return err
	}
	builder, err := container.NewBuildInfoBuilder(image, pc.Repo(), buildName, buildNumber, serviceManager, container.Pull, cm)
	if err != nil {
		return err
	}
	buildInfo, err := builder.Build(pc.BuildConfiguration().Module)
	if err != nil {
		return err
	}
	return utils.SaveBuildInfo(buildName, buildNumber, buildInfo)
}

func (pc *PullCommand) CommandName() string {
	return "rt_container_pull"
}

func (pc *PullCommand) RtDetails() (*config.ArtifactoryDetails, error) {
	return pc.rtDetails, nil
}
