package container

import (
	"errors"
	"io/ioutil"
	"strings"

	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/artifactory/utils/container"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type BuildDockerCreateCommand struct {
	ContainerManagerCommand
	containerManagerType container.ContainerManagerType
	manifestSha256       string
}

func NewBuildDockerCreateCommand() *BuildDockerCreateCommand {
	return &BuildDockerCreateCommand{containerManagerType: container.Kaniko}
}

func (bpc *BuildDockerCreateCommand) SetImageNameWithDigest(filePath string) error {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Debug("ioutil.ReadFile failed with '%s'\n", err)
		return errorutils.CheckError(err)
	}
	splittedData := strings.Split(string(data), `@`)
	if len(splittedData) != 2 {
		return errorutils.CheckError(errors.New(`unknown file format, a valide file should look like: image-tag@sha256`))
	}
	bpc.imageTag, bpc.manifestSha256 = splittedData[0], strings.Trim(splittedData[1], "\n")
	if bpc.imageTag == "" || bpc.manifestSha256 == "" {
		return errorutils.CheckError(errors.New(`missing image-tag/sha256 in file: "` + filePath + `"`))
	}
	return nil
}

func (bpc *BuildDockerCreateCommand) Run() error {
	rtDetails, err := bpc.RtDetails()
	if err != nil {
		return err
	}
	// Skip port colon.
	imagePath := bpc.imageTag[strings.Index(bpc.imageTag, "/"):]
	if strings.LastIndex(imagePath, ":") == -1 {
		bpc.imageTag = bpc.imageTag + ":latest"
	}
	cm := container.NewContainerManager(bpc.containerManagerType)
	image := container.NewImage(bpc.imageTag)
	buildName := bpc.BuildConfiguration().BuildName
	buildNumber := bpc.BuildConfiguration().BuildNumber
	if err := utils.SaveBuildGeneralDetails(buildName, buildNumber); err != nil {
		return err
	}
	serviceManager, err := utils.CreateServiceManager(rtDetails, false)
	if err != nil {
		return err
	}
	builder, err := container.NewKanikoBuildInfoBuilder(image, bpc.Repo(), buildName, buildNumber, serviceManager, container.Push, cm, bpc.manifestSha256)
	if err != nil {
		return err
	}
	buildInfo, err := builder.Build(bpc.BuildConfiguration().Module)
	if err != nil {
		return err
	}
	return utils.SaveBuildInfo(buildName, buildNumber, buildInfo)
}

func (pc *BuildDockerCreateCommand) CommandName() string {
	return "rt_build_docker_create"
}

func (pc *BuildDockerCreateCommand) RtDetails() (*config.ArtifactoryDetails, error) {
	return pc.rtDetails, nil
}
