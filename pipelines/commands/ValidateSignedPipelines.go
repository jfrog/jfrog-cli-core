package commands

import (
	"github.com/jfrog/jfrog-cli-core/v2/pipelines/manager"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type ValidateSignedPipelinesCommand struct {
	serverDetails        *config.ServerDetails
	artifactType         string
	buildName            string
	buildNumber          string
	projectKey           string
	artifactPath         string
	releaseBundleName    string
	releaseBundleVersion string
}

func NewValidateSignedPipelinesCommand() *ValidateSignedPipelinesCommand {
	return &ValidateSignedPipelinesCommand{}
}

func (vspc *ValidateSignedPipelinesCommand) ServerDetails() (*config.ServerDetails, error) {
	return vspc.serverDetails, nil
}

func (vspc *ValidateSignedPipelinesCommand) SetServerDetails(serverDetails *config.ServerDetails) *ValidateSignedPipelinesCommand {
	vspc.serverDetails = serverDetails
	return vspc
}

func (vspc *ValidateSignedPipelinesCommand) CommandName() string {
	return "pl_validate_signed_pipelines"
}

func (vspc *ValidateSignedPipelinesCommand) SetArtifactType(artifact string) *ValidateSignedPipelinesCommand {
	vspc.artifactType = artifact
	return vspc
}

func (vspc *ValidateSignedPipelinesCommand) SetBuildName(name string) *ValidateSignedPipelinesCommand {
	vspc.buildName = name
	return vspc
}

func (vspc *ValidateSignedPipelinesCommand) SetBuildNumber(number string) *ValidateSignedPipelinesCommand {
	vspc.buildNumber = number
	return vspc
}

func (vspc *ValidateSignedPipelinesCommand) SetProjectKey(project string) *ValidateSignedPipelinesCommand {
	vspc.projectKey = project
	return vspc
}

func (vspc *ValidateSignedPipelinesCommand) SetArtifactPath(artifact string) *ValidateSignedPipelinesCommand {
	vspc.artifactPath = artifact
	return vspc
}

func (vspc *ValidateSignedPipelinesCommand) SetReleaseBundleName(name string) *ValidateSignedPipelinesCommand {
	vspc.releaseBundleName = name
	return vspc
}

func (vspc *ValidateSignedPipelinesCommand) SetReleaseBundleVersion(version string) *ValidateSignedPipelinesCommand {
	vspc.releaseBundleVersion = version
	return vspc
}

func (vspc *ValidateSignedPipelinesCommand) Run() error {
	serviceManager, err := manager.CreateServiceManager(vspc.serverDetails)
	if err != nil {
		return err
	}
	err = serviceManager.ValidateSignedPipelines(vspc.artifactType, vspc.buildName, vspc.buildNumber, vspc.projectKey, vspc.artifactPath, vspc.releaseBundleName, vspc.releaseBundleVersion)
	return err
}
