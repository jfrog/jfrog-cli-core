package container

import (
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/container"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type ContainerManagerCommand struct {
	imageTag           string
	repo               string
	buildConfiguration *utils.BuildConfiguration
	serverDetails      *config.ServerDetails
	skipLogin          bool
}

func (cmc *ContainerManagerCommand) ImageTag() string {
	return cmc.imageTag
}

func (cmc *ContainerManagerCommand) SetImageTag(imageTag string) *ContainerManagerCommand {
	cmc.imageTag = imageTag
	// Remove base URL from the image tag.
	index := strings.Index(imageTag, "/")
	imageRelativePath := imageTag
	if index != -1 {
		imageRelativePath = imageTag[index:]
	}
	// Use the default image tag if none exists.
	if strings.LastIndex(imageRelativePath, ":") == -1 {
		cmc.imageTag += ":latest"
	}
	return cmc
}

func (cmc *ContainerManagerCommand) Repo() string {
	return cmc.repo
}

func (cmc *ContainerManagerCommand) SetRepo(repo string) *ContainerManagerCommand {
	cmc.repo = repo
	return cmc
}

func (cmc *ContainerManagerCommand) BuildConfiguration() *utils.BuildConfiguration {
	return cmc.buildConfiguration
}

func (cmc *ContainerManagerCommand) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *ContainerManagerCommand {
	cmc.buildConfiguration = buildConfiguration
	return cmc
}

func (cmc *ContainerManagerCommand) SetSkipLogin(skipLogin bool) *ContainerManagerCommand {
	cmc.skipLogin = skipLogin
	return cmc
}

func (cmc *ContainerManagerCommand) ServerDetails() *config.ServerDetails {
	return cmc.serverDetails
}

func (cmc *ContainerManagerCommand) SetServerDetails(serverDetails *config.ServerDetails) *ContainerManagerCommand {
	cmc.serverDetails = serverDetails
	return cmc
}

func (cmc *ContainerManagerCommand) PerformLogin(serverDetails *config.ServerDetails, containerManagerType container.ContainerManagerType) error {
	if !cmc.skipLogin {
		loginConfig := &container.ContainerManagerLoginConfig{ServerDetails: serverDetails}
		return container.ContainerManagerLogin(cmc.imageTag, loginConfig, containerManagerType)
	}
	return nil
}
