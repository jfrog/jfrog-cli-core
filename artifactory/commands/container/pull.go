package container

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/container"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

type PullCommand struct {
	ContainerCommand
}

func NewPullCommand(containerManagerType container.ContainerManagerType) *PullCommand {
	return &PullCommand{
		ContainerCommand: ContainerCommand{
			containerManagerType: containerManagerType,
		},
	}
}

func (pc *PullCommand) Run() error {
	if err := pc.init(); err != nil {
		return err
	}
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
	err = cm.RunNativeCmd(pc.cmdParams)
	if err != nil {
		return err
	}
	toCollect, err := pc.buildConfiguration.IsCollectBuildInfo()
	if err != nil || !toCollect {
		return err
	}
	buildName, err := pc.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := pc.buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}
	project := pc.BuildConfiguration().GetProject()
	serviceManager, err := utils.CreateServiceManager(serverDetails, -1, 0, false)
	if err != nil {
		return err
	}
	repo, err := pc.GetRepo()
	if err != nil {
		return err
	}
	builder, err := container.NewLocalAgentBuildInfoBuilder(pc.image, repo, buildName, buildNumber, project, serviceManager, container.Pull, cm)
	if err != nil {
		return err
	}
	if err := build.SaveBuildGeneralDetails(buildName, buildNumber, project); err != nil {
		return err
	}
	buildInfoModule, err := builder.Build(pc.BuildConfiguration().GetModule())
	if err != nil || buildInfoModule == nil {
		return err
	}
	return build.SaveBuildInfo(buildName, buildNumber, project, buildInfoModule)
}

func (pc *PullCommand) CommandName() string {
	return "rt_docker_pull"
}

func (pc *PullCommand) ServerDetails() (*config.ServerDetails, error) {
	return pc.serverDetails, nil
}
