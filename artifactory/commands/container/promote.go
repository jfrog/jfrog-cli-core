package container

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
)

type DockerPromoteCommand struct {
	serverDetails *config.ServerDetails
	params        services.DockerPromoteParams
}

func NewDockerPromoteCommand() *DockerPromoteCommand {
	return &DockerPromoteCommand{}
}

func (dp *DockerPromoteCommand) Run() error {
	// Create Service Manager
	servicesManager, err := utils.CreateServiceManager(dp.serverDetails, false)
	if err != nil {
		return err
	}
	// Promote docker
	return servicesManager.PromoteDocker(dp.params)
}

func (dp *DockerPromoteCommand) CommandName() string {
	return "rt_docker_promote"
}

func (dp *DockerPromoteCommand) ServerDetails() (*config.ServerDetails, error) {
	return dp.serverDetails, nil
}

func (dp *DockerPromoteCommand) SetServerDetails(serverDetails *config.ServerDetails) *DockerPromoteCommand {
	dp.serverDetails = serverDetails
	return dp
}

func (dp *DockerPromoteCommand) SetParams(params services.DockerPromoteParams) *DockerPromoteCommand {
	dp.params = params
	return dp
}
