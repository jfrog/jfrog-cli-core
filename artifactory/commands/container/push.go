package container

import (
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/container"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	servicesutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
)

type PushCommand struct {
	ContainerManagerCommand
	threads              int
	containerManagerType container.ContainerManagerType
	detailedSummary      bool
	result               *commandsutils.Result
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

func (pc *PushCommand) SetDetailedSummary(detailedSummary bool) *PushCommand {
	pc.detailedSummary = detailedSummary
	return pc
}

func (pc *PushCommand) IsDetailedSummary() bool {
	return pc.detailedSummary
}

func (pc *PushCommand) Result() *commandsutils.Result {
	return pc.result
}

func (pc *PushCommand) SetResult(result *commandsutils.Result) *PushCommand {
	pc.result = result
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
	// Return if build-info and detailed summary were not requested.
	toCollect, err := pc.buildConfiguration.IsCollectBuildInfo()
	if err != nil {
		return err
	}
	if !toCollect && !pc.IsDetailedSummary() {
		return nil
	}
	buildName, err := pc.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := pc.buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}
	if err := utils.SaveBuildGeneralDetails(buildName, buildNumber, pc.buildConfiguration.GetProject()); err != nil {
		return err
	}
	serviceManager, err := utils.CreateServiceManagerWithThreads(serverDetails, false, pc.threads, -1, 0)
	if err != nil {
		return err
	}
	builder, err := container.NewLocalAgentBuildInfoBuilder(image, pc.Repo(), buildName, buildNumber, pc.BuildConfiguration().GetProject(), serviceManager, container.Push, cm)
	if err != nil {
		return err
	}
	// Save buildinfo if needed
	if toCollect {
		buildInfo, err := builder.Build(pc.BuildConfiguration().GetModule())
		if err != nil {
			return err
		}
		err = utils.SaveBuildInfo(buildName, buildNumber, pc.BuildConfiguration().GetProject(), buildInfo)
		if err != nil {
			return err
		}
	}
	// Save detailed summary if needed
	if pc.IsDetailedSummary() {
		if !toCollect {
			// If we saved buildinfo earlier, this update already happened.
			builder.SetDryRun(true)
			_, err = builder.Build("")
			if err != nil {
				return err
			}
		}
		artifactsDetails := layersMapToFileTransferDetails(serverDetails.ArtifactoryUrl, builder.GetLayers())
		tempFile, err := clientutils.SaveFileTransferDetailsInTempFile(artifactsDetails)
		if err != nil {
			return err
		}
		result := new(commandsutils.Result)
		result.SetReader(content.NewContentReader(tempFile, "files"))
		result.SetSuccessCount(len(*artifactsDetails))
		pc.SetResult(result)
	}
	return nil
}

func layersMapToFileTransferDetails(artifactoryUrl string, layers *[]servicesutils.ResultItem) *[]clientutils.FileTransferDetails {
	var details []clientutils.FileTransferDetails
	for _, layer := range *layers {
		sha256 := ""
		for _, property := range layer.Properties {
			if property.Key == "sha256" {
				sha256 = property.Value
			}
		}
		target := artifactoryUrl + layer.Repo + "/" + layer.Path + "/" + layer.Name
		details = append(details, clientutils.FileTransferDetails{TargetPath: target, Sha256: sha256})
	}
	return &details
}

func (pc *PushCommand) CommandName() string {
	return "rt_docker_push"
}

func (pc *PushCommand) ServerDetails() (*config.ServerDetails, error) {
	return pc.serverDetails, nil
}
