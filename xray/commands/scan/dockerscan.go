package scan

import (
	"bytes"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	xrayutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/wagoodman/dive/dive"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	indexerEnvPrefix         = "JFROG_INDEXER_"
	DockerScanMinXrayVersion = "3.40.0"
	layerDigestPrefix        = "sha256:"
	// Suffix added while analyzing docker layers, remove it for better readability.
	buildKitSuffix = " # buildkit"
)

type DockerScanCommand struct {
	ScanCommand
	imageTag       string
	targetRepoPath string
	// Maps layer hash to dockerfile command
	dockerfileCommandsMapping map[string]string
}

func NewDockerScanCommand() *DockerScanCommand {
	return &DockerScanCommand{ScanCommand: *NewScanCommand()}
}

func (dsc *DockerScanCommand) SetImageTag(imageTag string) *DockerScanCommand {
	dsc.imageTag = imageTag
	return dsc
}

func (dsc *DockerScanCommand) SetTargetRepoPath(repoPath string) *DockerScanCommand {
	dsc.targetRepoPath = repoPath
	return dsc
}

// DockerScan scan will save a docker image as .tar file and will prefore binary scan on it.
func (dsc *DockerScanCommand) Run() (err error) {
	// Validate Xray minimum version
	_, xrayVersion, err := xrayutils.CreateXrayServiceManagerAndGetVersion(dsc.ScanCommand.serverDetails)
	if err != nil {
		return err
	}
	err = clientutils.ValidateMinimumVersion(clientutils.Xray, xrayVersion, DockerScanMinXrayVersion)
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
	if dsc.progress != nil {
		dsc.progress.SetHeadlineMsg("Creating image archive 📦")
	}
	log.Info("Creating image archive...")
	imageTarPath := filepath.Join(tempDirPath, "image.tar")

	dockerSaveCmd := exec.Command("docker", "save", dsc.imageTag, "-o", imageTarPath)
	var stderr bytes.Buffer
	dockerSaveCmd.Stderr = &stderr
	if err = dockerSaveCmd.Run(); err != nil {
		return fmt.Errorf("failed running command: '%s' with error: %s - %s", strings.Join(dockerSaveCmd.Args, " "), err.Error(), stderr.String())
	}

	// Map layers sha256 checksum names to dockerfile line commands
	if err = dsc.mapDockerLayerToCommand(); err != nil {
		return
	}

	// Perform scan on image.tar
	dsc.SetSpec(spec.NewBuilder().
		Pattern(imageTarPath).
		Target(dsc.targetRepoPath).
		BuildSpec()).SetThreads(1)

	if err = dsc.setCredentialEnvsForIndexerApp(); err != nil {
		return errorutils.CheckError(err)
	}
	defer func() {
		e := dsc.unsetCredentialEnvsForIndexerApp()
		if err == nil {
			err = errorutils.CheckError(e)
		}
	}()
	// Preform binary scan.
	extendedScanResults, cleanup, scanErrors, err := dsc.ScanCommand.binaryScan()
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return
	}

	// Print results with docker commands mapping.
	err = xrayutils.NewResultsWriter(extendedScanResults).
		SetOutputFormat(dsc.outputFormat).
		SetIncludeVulnerabilities(dsc.includeVulnerabilities).
		SetIncludeLicenses(dsc.includeLicenses).
		SetPrintExtendedTable(dsc.printExtendedTable).
		SetIsMultipleRootProject(true).
		SetDockerCommandsMapping(dsc.dockerfileCommandsMapping).
		PrintScanResults()

	return dsc.ScanCommand.handlePossibleErrors(extendedScanResults.XrayResults, scanErrors, err)
}

func (dsc *DockerScanCommand) mapDockerLayerToCommand() (err error) {
	log.Debug("Mapping docker layers into commands...")
	resolver, err := dive.GetImageResolver(dive.SourceDockerEngine)
	if err != nil {
		return errorutils.CheckErrorf("failed to map docker layers, is docker running on your machine? error message: %s", err.Error())
	}
	dockerImage, err := resolver.Fetch(dsc.imageTag)
	if err != nil {
		return
	}
	// Create mapping between sha256 hash to dockerfile Command.
	layersMapping := make(map[string]string)
	for _, layer := range dockerImage.Layers {
		layerHash := strings.TrimPrefix(layer.Digest, layerDigestPrefix)
		layersMapping[layerHash] = cleanDockerfileCommand(layer.Command)
	}
	dsc.dockerfileCommandsMapping = layersMapping
	return
}

// DockerScan command could potentiality have double spaces,
// Reconstruct the command with only one space between arguments.
// Example: command from dive: "RUN apt-get install &&     apt-get install #builtkit".
// Will resolve to a cleaner command RUN apt-get install && apt-get install
func cleanDockerfileCommand(rawCommand string) string {
	fields := strings.Fields(rawCommand)
	command := strings.Join(fields, " ")
	return strings.TrimSuffix(command, buildKitSuffix)
}

// When indexing RPM files inside the docker container, the indexer-app needs to connect to the Xray Server.
// This is because RPM indexing is performed on the server side. This method therefore sets the Xray credentials as env vars to be read and used by the indexer-app.
func (dsc *DockerScanCommand) setCredentialEnvsForIndexerApp() error {
	err := os.Setenv(indexerEnvPrefix+"XRAY_URL", dsc.serverDetails.XrayUrl)
	if err != nil {
		return err
	}
	if dsc.serverDetails.AccessToken != "" {
		err = os.Setenv(indexerEnvPrefix+"XRAY_ACCESS_TOKEN", dsc.serverDetails.AccessToken)
		if err != nil {
			return err
		}
	} else {
		err = os.Setenv(indexerEnvPrefix+"XRAY_USER", dsc.serverDetails.User)
		if err != nil {
			return err
		}
		err = os.Setenv(indexerEnvPrefix+"XRAY_PASSWORD", dsc.serverDetails.Password)
		if err != nil {
			return err
		}
	}
	return nil
}

func (dsc *DockerScanCommand) unsetCredentialEnvsForIndexerApp() error {
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

func (dsc *DockerScanCommand) CommandName() string {
	return "xr_docker_scan"
}
