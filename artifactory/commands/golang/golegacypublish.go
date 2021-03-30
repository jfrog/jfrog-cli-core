package golang

import (
	"errors"
	"strings"

	commandutils "github.com/jfrog/jfrog-cli-core/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils/golang"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils/golang/project"
	_go "github.com/jfrog/jfrog-client-go/artifactory/services/go"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/version"
)

type GoLegacyPublishCommand struct {
	internalCommandName string
	publishPackage      bool
	*GoPublishCommandArgs
}

func NewGoLegacyPublishCommand() *GoLegacyPublishCommand {
	return &GoLegacyPublishCommand{GoPublishCommandArgs: &GoPublishCommandArgs{result: new(commandutils.Result)}, internalCommandName: "rt_go_legacy_publish"}
}

func (glpc *GoLegacyPublishCommand) CommandName() string {
	return glpc.internalCommandName
}

func (glpc *GoLegacyPublishCommand) SetPublishPackage(publishPackage bool) *GoLegacyPublishCommand {
	glpc.publishPackage = publishPackage
	return glpc
}

func (glpc *GoLegacyPublishCommand) Run() error {
	err := validatePrerequisites()
	if err != nil {
		return err
	}

	err = golang.LogGoVersion()
	if err != nil {
		return err
	}

	serverDetails, err := glpc.ServerDetails()
	if errorutils.CheckError(err) != nil {
		return err
	}
	serviceManager, err := utils.CreateServiceManager(serverDetails, false)
	if err != nil {
		return err
	}
	artifactoryVersion, err := serviceManager.GetConfig().GetServiceDetails().GetVersion()
	if err != nil {
		return err
	}

	version := version.NewVersion(artifactoryVersion)
	if !version.AtLeast(minSupportedArtifactoryVersion) {
		return errorutils.CheckError(errors.New("This operation requires Artifactory version 6.2.0 or higher."))
	}

	buildName := glpc.buildConfiguration.BuildName
	buildNumber := glpc.buildConfiguration.BuildNumber
	projectKey := glpc.buildConfiguration.Project
	isCollectBuildInfo := len(buildName) > 0 && len(buildNumber) > 0
	if isCollectBuildInfo {
		err = utils.SaveBuildGeneralDetails(buildName, buildNumber, projectKey)
		if err != nil {
			return err
		}
	}

	goProject, err := project.Load(glpc.version, "")
	if err != nil {
		return err
	}

	// Publish the package to Artifactory
	if glpc.publishPackage {
		err = goProject.PublishPackage(glpc.TargetRepo(), buildName, buildNumber, projectKey, serviceManager)
		if err != nil {
			return err
		}
	}

	result := glpc.Result()
	if glpc.dependencies != "" {
		// Publish the package dependencies to Artifactory
		depsList := strings.Split(glpc.dependencies, ",")
		err = goProject.LoadDependencies()
		if err != nil {
			return err
		}
		succeeded, failed, err := goProject.PublishDependencies(glpc.TargetRepo(), serviceManager, depsList)
		result.SetSuccessCount(succeeded)
		result.SetFailCount(failed)
		if err != nil {
			return err
		}
	}
	if glpc.publishPackage {
		result.SetSuccessCount(result.SuccessCount() + 1)
	}

	// Publish the build-info to Artifactory
	if isCollectBuildInfo {
		if len(goProject.Dependencies()) == 0 {
			// No dependencies were published but those dependencies need to be loaded for the build info.
			goProject.LoadDependencies()
		}
		err = goProject.CreateBuildInfoDependencies(version.AtLeast(_go.ArtifactoryMinSupportedVersionForInfoFile))
		if err != nil {
			return err
		}
		err = utils.SaveBuildInfo(buildName, buildNumber, projectKey, goProject.BuildInfo(true, glpc.buildConfiguration.Module, glpc.RepositoryConfig.TargetRepo()))
	}

	return err
}
