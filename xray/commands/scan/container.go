package scan

import (
	"errors"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/container"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"os"
	"path/filepath"
)

const indexerEnvPrefix = "JFROG_INDEXER_"

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

	err = csc.setCredentialEnvsForIndexerApp()
	if err != nil {
		return errorutils.CheckError(err)
	}
	defer func() {
		e := csc.unsetCredentialEnvsForIndexerApp()
		if err == nil {
			err = errorutils.CheckError(e)
		}
	}()
	return csc.ScanCommand.Run()
}

// when indexing docker rpm files the indexer app needs connection with Xray Server to deal with the rpm files
func (csc *ContainerScanCommand) setCredentialEnvsForIndexerApp() error {
	err := os.Setenv(indexerEnvPrefix+"XRAY_URL", csc.serverDetails.XrayUrl)
	if err != nil {
		return err
	}
	if csc.serverDetails.AccessToken != "" {
		err = os.Setenv(indexerEnvPrefix+"XRAY_ACCESS_TOKEN", csc.serverDetails.AccessToken)
		if err != nil {
			return err
		}
	} else {
		err = os.Setenv(indexerEnvPrefix+"XRAY_USER", csc.serverDetails.User)
		if err != nil {
			return err
		}
		err = os.Setenv(indexerEnvPrefix+"XRAY_PASSWORD", csc.serverDetails.Password)
		if err != nil {
			return err
		}
	}
	return nil
}

func (csc *ContainerScanCommand) unsetCredentialEnvsForIndexerApp() error {
	err := os.Unsetenv(indexerEnvPrefix + "XRAY_URL")
	if err != nil {
		return err
	}
	err = os.Unsetenv(indexerEnvPrefix + "XRAY_ACCESS_TOKEN")
	if err != nil {
		return err
	}
	err = os.Unsetenv(indexerEnvPrefix + "XRAY_USER")
	if err != nil {
		return err
	}
	err = os.Unsetenv(indexerEnvPrefix + "XRAY_PASSWORD")
	if err != nil {
		return err
	}

	return nil
}

func (csc *ContainerScanCommand) CommandName() string {
	return "xr_container_scan"
}
