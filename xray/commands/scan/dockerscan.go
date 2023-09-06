package scan

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	xrayutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/wagoodman/dive/dive"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	indexerEnvPrefix         = "JFROG_INDEXER_"
	DockerScanMinXrayVersion = "3.40.0"
)

type DockerScanCommand struct {
	ScanCommand
	imageTag         string
	targetRepoPath   string
	dockerFilePath   string
	scanner          *bufio.Scanner
	hashToCommandMap map[string]services.DockerfileCommandDetails
	commandsMap      map[string]string
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

	// No image tag provided, meaning we work with a dockerfile
	if dsc.imageTag == "" {
		if err = dsc.buildDockerImage(); err != nil {
			return
		}
		// Load content of dockerfile to memory to allow search by file line.
		cleanUp, err := dsc.loadDockerfileToMemory()
		defer cleanUp()
		if err != nil {
			return err
		}
	}

	// Run the 'docker save' command, to create tar file from the docker image, and pass it to the indexer-app
	if dsc.progress != nil {
		dsc.progress.SetHeadlineMsg("Creating image archive 📦...")
	}
	log.Info("Creating image archive...")
	imageTarPath := filepath.Join(tempDirPath, "image.tar")

	dockerSaveCmd := exec.Command("docker", "save", dsc.imageTag, "-o", imageTarPath)
	var stderr bytes.Buffer
	dockerSaveCmd.Stderr = &stderr
	err = dockerSaveCmd.Run()
	if err != nil {
		return fmt.Errorf("failed running command: '%s' with error: %s - %s", strings.Join(dockerSaveCmd.Args, " "), err.Error(), stderr.String())
	}

	dockerCommandsMapping, err := dsc.mapDockerLayerToCommand()
	if err != nil {
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

	extendedScanResults, cleanup, scanErrors, err := dsc.ScanCommand.binaryScan()
	defer cleanup()
	if err != nil {
		return
	}
	// Print results
	err = xrutils.NewResultsWriter(extendedScanResults).
		SetOutputFormat(dsc.outputFormat).
		SetIncludeVulnerabilities(dsc.includeVulnerabilities).
		SetIncludeLicenses(dsc.includeLicenses).
		SetPrintExtendedTable(dsc.printExtendedTable).
		SetIsMultipleRootProject(true).
		SetDockerCommandsMapping(dockerCommandsMapping).
		SetScanType(services.Docker).
		PrintScanResults()

	return dsc.ScanCommand.handlePossibleErrors(extendedScanResults.XrayResults, scanErrors, err)
}

func (dsc *DockerScanCommand) buildDockerImage() (err error) {
	if exists, _ := fileutils.IsFileExists(".dockerfile", false); !exists {
		return fmt.Errorf("didn't find Dockerfile in the provided path: %s", dsc.dockerFilePath)
	}
	if dsc.progress != nil {
		dsc.progress.SetHeadlineMsg(fmt.Sprintf("Building Docker image from: %s  🏗️...", dsc.dockerFilePath))
	}
	log.Info("Building docker image")
	dsc.imageTag = "audittag"
	dockerBuildCommand := exec.Command("docker", "build", ".", "-f", ".dockerfile", "-t", dsc.imageTag)
	if err = dockerBuildCommand.Run(); err != nil {
		return fmt.Errorf("failed to build docker image,error: %s", err.Error())
	}
	log.Info("Successfully build image from dockerfile")
	return
}

func (dsc *DockerScanCommand) mapDockerLayerToCommand() (layerToDockerfileCommand map[string]services.DockerfileCommandDetails, err error) {
	log.Debug("Mapping docker layers into commands ")
	resolver, err := dive.GetImageResolver(dive.SourceDockerEngine)
	if err != nil {
		return
	}
	dockerImage, err := resolver.Fetch(dsc.imageTag)
	if err != nil {
		return
	}
	// Create mapping between sha256 hash to dockerfile Command.
	layerToDockerfileCommand = make(map[string]services.DockerfileCommandDetails)
	commandToHash := make(map[string]string)
	for _, layer := range dockerImage.Layers {
		layerHash := strings.TrimPrefix(layer.Digest, "sha256:")
		command := layer.Command
		if !strings.HasPrefix(layer.Command, "#") {
			command = strings.Split(layer.Command, "#")[0]
		}
		command = strings.TrimSpace(command)
		layerToDockerfileCommand[layerHash] = services.DockerfileCommandDetails{LayerHash: layer.Digest, Command: command}
		commandToHash[command] = layerHash
	}
	layerToDockerfileCommand = dsc.enrichCommandWithLineNumbers(layerToDockerfileCommand, commandToHash)
	return
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

func (dsc *DockerScanCommand) loadDockerfileToMemory() (cleanUp func(), err error) {
	file, err := os.Open(".dockerfile")
	if err != nil {
		err = fmt.Errorf("failed while trying to load dockerfile")
		return
	}
	cleanUp = func() {
		err = file.Close()
	}
	dsc.scanner = bufio.NewScanner(file)
	return
}

func (dsc *DockerScanCommand) enrichCommandWithLineNumbers(dockerCommandsMap map[string]services.DockerfileCommandDetails, commandToHash map[string]string) map[string]services.DockerfileCommandDetails {
	lineNumber := 0

	// Loop through each line of the file
	for dsc.scanner.Scan() {
		scannedCommand := dsc.scanner.Text()
		if strings.HasPrefix(scannedCommand, "#") || scannedCommand == "" {
			// Skip comments in the dockerfile
			lineNumber++
			continue
		}
		// TODO -> Extact map matching is not a good solution, need to think of somethign else
		// TODO RUN /bin/sh -c curl -sL https://deb.nodesource.com/setup_14.x | bash -
		// tODO RUN curl -sL https://deb.nodesource.com/setup_14.x | bash -
		// TODO for exmaple
		commandHash := commandToHash[scannedCommand]
		cmdDetails, exsists := dockerCommandsMap[commandHash]
		if exsists {
			cmdDetails.Line = strconv.Itoa(lineNumber)
		}

		lineNumber++
	}
	// Check for scanner errors
	if err := dsc.scanner.Err(); err != nil {
		fmt.Println("Error scanning file:", err)
	}

	return dockerCommandsMap
}
