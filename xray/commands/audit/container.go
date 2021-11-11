package audit

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/container"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"path/filepath"
)

type ContainerScanCommand struct {
	ScanCommand
	containerManagerType container.ContainerManagerType
	imageTag             string
}

func NewEmptyContainerScanCommand() *ContainerScanCommand {
	return &ContainerScanCommand{ScanCommand: *NewScanCommand()}
}

func NewAuditContainerCommand(scanCommand ScanCommand) *ContainerScanCommand {
	return &ContainerScanCommand{ScanCommand: scanCommand}
}

func (csc *ContainerScanCommand) SetImageTag(imageTag string) *ContainerScanCommand {
	csc.imageTag = imageTag
	return csc
}

func (csc *ContainerScanCommand) SetContainerManagerType(containerManagerType container.ContainerManagerType) *ContainerScanCommand {
	csc.containerManagerType = containerManagerType
	return csc
}

func (csc *ContainerScanCommand) Run() (err error) {
	// Perform scan.
	cm := container.NewManager(csc.containerManagerType)
	image := container.NewImage(csc.imageTag)

	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		return err
	}
	defer func() {
		e := fileutils.RemoveTempDir(tempDirPath)
		if err == nil {
			err = e
		}
	}()

	// run docker/podman save command to create tar file from image and pass it to the indexer to perform layers scan.
	tarFilePath := filepath.Join(tempDirPath, "image.tar")
	err = cm.Save(image, tarFilePath)
	if err != nil {
		return errors.New("Failed running " + csc.containerManagerType.String() + " save command with error: " + err.Error())
	}

	filSpec := spec.NewBuilder().
		Pattern(tarFilePath).
		BuildSpec()
	csc.SetSpec(filSpec).SetThreads(1)
	return csc.ScanCommand.Run()
}

func (csc *ContainerScanCommand) CommandName() string {
	return "xr_scan_container"
}
