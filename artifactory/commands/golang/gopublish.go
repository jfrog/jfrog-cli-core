package golang

import (
	"errors"
	"os/exec"
	"strings"

	commandutils "github.com/jfrog/jfrog-cli-core/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils/golang"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils/golang/project"
	_go "github.com/jfrog/jfrog-client-go/artifactory/services/go"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/version"
)

const minSupportedArtifactoryVersion = "6.2.0"

type GoPublishCommandArgs struct {
	buildConfiguration *utils.BuildConfiguration
	dependencies       string
	version            string
	detailedSummary    bool
	result             *commandutils.Result
	utils.RepositoryConfig
}

type GoPublishCommand struct {
	configFilePath      string
	internalCommandName string
	*GoPublishCommandArgs
}

func NewGoPublishCommand() *GoPublishCommand {
	return &GoPublishCommand{GoPublishCommandArgs: &GoPublishCommandArgs{result: new(commandutils.Result)}, internalCommandName: "rt_go_publish"}
}

func (gpc *GoPublishCommand) CommandName() string {
	return gpc.internalCommandName
}

func (gpc *GoPublishCommand) SetConfigFilePath(configFilePath string) *GoPublishCommand {
	gpc.configFilePath = configFilePath
	return gpc
}

func (gpc *GoPublishCommand) Run() error {
	err := validatePrerequisites()
	if err != nil {
		return err
	}

	err = golang.LogGoVersion()
	if err != nil {
		return err
	}
	// Read config file.
	vConfig, err := utils.ReadConfigFile(gpc.configFilePath, utils.YAML)
	if err != nil {
		return err
	}
	repoConfig, err := utils.GetRepoConfigByPrefix(gpc.configFilePath, utils.ProjectConfigDeployerPrefix, vConfig)
	if err != nil {
		return err
	}
	gpc.RepositoryConfig = *repoConfig
	serverDetails, err := gpc.ServerDetails()
	if errorutils.CheckError(err) != nil {
		return err
	}
	serviceManager, err := utils.CreateServiceManager(serverDetails, -1, false)
	if err != nil {
		return err
	}
	artifactoryVersion, err := serviceManager.GetConfig().GetServiceDetails().GetVersion()
	if err != nil {
		return err
	}

	version := version.NewVersion(artifactoryVersion)
	if !version.AtLeast(minSupportedArtifactoryVersion) {
		return errorutils.CheckError(errors.New("This operation requires Artifactory version 6.2.0 or higher. "))
	}

	buildName := gpc.buildConfiguration.BuildName
	buildNumber := gpc.buildConfiguration.BuildNumber
	projectKey := gpc.buildConfiguration.Project
	isCollectBuildInfo := len(buildName) > 0 && len(buildNumber) > 0
	if isCollectBuildInfo {
		err = utils.SaveBuildGeneralDetails(buildName, buildNumber, projectKey)
		if err != nil {
			return err
		}
	}

	goProject, err := project.Load(gpc.version, "")
	if err != nil {
		return err
	}

	// Publish the package to Artifactory
	summary, err := goProject.PublishPackage(gpc.TargetRepo(), buildName, buildNumber, projectKey, serviceManager)
	if err != nil {
		return err
	}

	result := gpc.Result()
	if gpc.dependencies != "" {
		// Publish the package dependencies to Artifactory
		depsList := strings.Split(gpc.dependencies, ",")
		err = goProject.LoadDependencies()
		if err != nil {
			return err
		}
		_, _, err := goProject.PublishDependencies(gpc.TargetRepo(), serviceManager, depsList)
		if err != nil {
			return err
		}
	}

	// From Artifactory's version 6.6.1 PublishPackage() will upload 3 files (zip, mod and info) and return summary.
	// Otherwise summary will be nil and only 1 file will be uploaded.
	if summary != nil {
		result.SetSuccessCount(summary.TotalSucceeded)
		result.SetFailCount(summary.TotalFailed)
		if gpc.detailedSummary {
			result.SetReader(summary.TransferDetailsReader)
		}
	} else {
		result.SetSuccessCount(1)
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
		err = utils.SaveBuildInfo(buildName, buildNumber, projectKey, goProject.BuildInfo(true, gpc.buildConfiguration.Module, gpc.RepositoryConfig.TargetRepo()))
	}

	return err
}

func (gpca *GoPublishCommandArgs) Result() *commandutils.Result {
	return gpca.result
}

func (gpca *GoPublishCommandArgs) SetVersion(version string) *GoPublishCommandArgs {
	gpca.version = version
	return gpca
}

func (gpca *GoPublishCommandArgs) SetDependencies(dependencies string) *GoPublishCommandArgs {
	gpca.dependencies = dependencies
	return gpca
}

func (gpca *GoPublishCommandArgs) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *GoPublishCommandArgs {
	gpca.buildConfiguration = buildConfiguration
	return gpca
}

func (gpca *GoPublishCommandArgs) SetDetailedSummary(detailedSummary bool) *GoPublishCommandArgs {
	gpca.detailedSummary = detailedSummary
	return gpca
}

func (gpca *GoPublishCommandArgs) IsDetailedSummary() bool {
	return gpca.detailedSummary
}

func validatePrerequisites() error {
	_, err := exec.LookPath("go")
	if err != nil {
		return errorutils.CheckError(err)
	}
	return nil
}
