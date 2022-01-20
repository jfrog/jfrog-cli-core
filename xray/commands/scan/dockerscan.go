package scan

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	indexerEnvPrefix         = "JFROG_INDEXER_"
	DockerScanMinXrayVersion = "3.40.0"
)

type DockerScanCommand struct {
	ScanCommand
	imageTag string
}

func NewDockerScanCommand() *DockerScanCommand {
	return &DockerScanCommand{ScanCommand: *NewScanCommand()}
}

func (csc *DockerScanCommand) SetImageTag(imageTag string) *DockerScanCommand {
	csc.imageTag = imageTag
	return csc
}

func (csc *DockerScanCommand) Run() (err error) {
	// Validate Xray minimum version
	_, xrayVersion, err := commands.CreateXrayServiceManagerAndGetVersion(csc.ScanCommand.serverDetails)
	if err != nil {
		return err
	}
	err = commands.ValidateXrayMinimumVersion(xrayVersion, DockerScanMinXrayVersion)
	if err != nil {
		return err
	}

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

	// Run the 'docker save' command, to create tar file from the docker image, and pass it to the indexer-app
	imageTarPath := filepath.Join(tempDirPath, "image.tar")
	dockerSaveCmd := exec.Command("docker", "save", csc.imageTag, "-o", imageTarPath)
	var stderr bytes.Buffer
	dockerSaveCmd.Stderr = &stderr
	err = dockerSaveCmd.Run()
	if err != nil {
		return errors.New(fmt.Sprintf("Failed running command: '%s' with error: %s - %s", strings.Join(dockerSaveCmd.Args, " "), err.Error(), stderr.String()))
	}

	// Perform scan on image.tar
	csc.SetSpec(spec.NewBuilder().Pattern(imageTarPath).BuildSpec()).SetThreads(1)
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

// When indexing RPM files inside the docker container, the indexer-app needs to connect to the Xray Server.
// This is because RPM indexing is performed on the server side. This method therefore sets the Xray credentials as env vars to be read and used by the indexer-app.
func (csc *DockerScanCommand) setCredentialEnvsForIndexerApp() error {
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

func (csc *DockerScanCommand) unsetCredentialEnvsForIndexerApp() error {
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

func (csc *DockerScanCommand) CommandName() string {
	return "xr_docker_scan"
}
