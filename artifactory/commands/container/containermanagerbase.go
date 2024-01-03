package container

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/container"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

const (
	// Artifactory 'MinRtVersionForRepoFetching' version and above, returns the image's repository in Artifactory.
	MinRtVersionForRepoFetching = "7.33.3"
)

type ContainerCommandBase struct {
	image              *container.Image
	repo               string
	buildConfiguration *build.BuildConfiguration
	serverDetails      *config.ServerDetails
}

func (ccb *ContainerCommandBase) ImageTag() string {
	return ccb.image.Name()
}

func (ccb *ContainerCommandBase) SetImageTag(imageTag string) *ContainerCommandBase {
	ccb.image = container.NewImage(imageTag)
	return ccb
}

// Returns the repository name that contains this image.
func (ccb *ContainerCommandBase) GetRepo() (string, error) {
	// The repository name is saved after first calling this function.
	if ccb.repo != "" {
		return ccb.repo, nil
	}

	serviceManager, err := utils.CreateServiceManager(ccb.serverDetails, -1, 0, false)
	if err != nil {
		return "", err
	}
	ccb.repo, err = ccb.image.GetRemoteRepo(serviceManager)
	return ccb.repo, err
}

func (ccb *ContainerCommandBase) SetRepo(repo string) *ContainerCommandBase {
	ccb.repo = repo
	return ccb
}

// Since 'RtMinVersion' version of Artifactory we can fetch the docker repository without the user input (witch is deprecated).
func (ccb *ContainerCommandBase) IsGetRepoSupported() (bool, error) {
	serviceManager, err := utils.CreateServiceManager(ccb.serverDetails, -1, 0, false)
	if err != nil {
		return false, err
	}
	currentVersion, err := serviceManager.GetVersion()
	if err != nil {
		return false, err
	}
	err = clientutils.ValidateMinimumVersion(clientutils.Artifactory, currentVersion, MinRtVersionForRepoFetching)
	return err == nil, nil
}

func (ccb *ContainerCommandBase) BuildConfiguration() *build.BuildConfiguration {
	return ccb.buildConfiguration
}

func (ccb *ContainerCommandBase) SetBuildConfiguration(buildConfiguration *build.BuildConfiguration) *ContainerCommandBase {
	ccb.buildConfiguration = buildConfiguration
	return ccb
}

func (ccb *ContainerCommandBase) ServerDetails() *config.ServerDetails {
	return ccb.serverDetails
}

func (ccb *ContainerCommandBase) SetServerDetails(serverDetails *config.ServerDetails) *ContainerCommandBase {
	ccb.serverDetails = serverDetails
	return ccb
}

func (ccb *ContainerCommandBase) init() error {
	toCollect, err := ccb.buildConfiguration.IsCollectBuildInfo()
	if err != nil || !toCollect {
		return err
	}
	if ccb.repo != "" {
		return nil
	}
	// Check we have all we need to collect build-info.
	ok, err := ccb.IsGetRepoSupported()
	if err != nil {
		return err
	}
	if !ok {
		return errorutils.CheckErrorf("Collecting docker build-info with this command requires Artifactory version %s or higher", MinRtVersionForRepoFetching)
	}
	return nil
}
