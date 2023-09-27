package scan

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	xrayutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/wagoodman/dive/dive"
	"github.com/wagoodman/dive/dive/image"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	indexerEnvPrefix         = "JFROG_INDEXER_"
	DockerScanMinXrayVersion = "3.40.0"
	maxDisplayCommandLength  = 60
	layerDigestPrefix        = "sha256:"
	buildKitSuffix           = " # buildkit"
)

type DockerScanCommand struct {
	ScanCommand
	imageTag       string
	targetRepoPath string
	dockerFilePath string
	scanner        *bufio.Scanner
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
	// Check for dockerfile to scan
	var isDockerFileScanned bool
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
		isDockerFileScanned = true
	}

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

	// Map layers sha to build commands
	// If dockerfile exists, will also map to line number.
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
	err = xrayutils.NewResultsWriter(extendedScanResults).
		SetOutputFormat(dsc.outputFormat).
		SetIncludeVulnerabilities(dsc.includeVulnerabilities).
		SetIncludeLicenses(dsc.includeLicenses).
		SetPrintExtendedTable(dsc.printExtendedTable).
		SetIsMultipleRootProject(true).
		SetDockerCommandsMapping(dockerCommandsMapping, isDockerFileScanned).
		SetScanType(services.Docker).
		PrintScanResults()

	return dsc.ScanCommand.handlePossibleErrors(extendedScanResults.XrayResults, scanErrors, err)
}

func (dsc *DockerScanCommand) buildDockerImage() (err error) {
	if exists, _ := fileutils.IsFileExists(".dockerfile", false); !exists {
		return fmt.Errorf("didn't find Dockerfile in the provided path: %s", dsc.dockerFilePath)
	}
	if dsc.progress != nil {
		dsc.progress.SetHeadlineMsg("Building Docker image 🏗....️")
	}
	dsc.imageTag = "audittag"
	log.Info("Building docker image... ")
	var stderr bytes.Buffer
	dockerBuildCommand := exec.Command("docker", "build", ".", "-f", ".dockerfile", "-t", dsc.imageTag)
	dockerBuildCommand.Stderr = &stderr
	if err = dockerBuildCommand.Run(); err != nil {
		return fmt.Errorf("failed to build docker image. Is docker running on your computer? error: %s", err.Error())
	}
	log.Info("Successfully build image from dockerfile")
	return
}

func (dsc *DockerScanCommand) mapDockerLayerToCommand() (layersMapping map[string]services.DockerfileCommandDetails, err error) {
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
	layersMapping = make(map[string]services.DockerfileCommandDetails)
	for _, layer := range dockerImage.Layers {
		layerHash := strings.TrimPrefix(layer.Digest, layerDigestPrefix)
		layersMapping[layerHash] = services.DockerfileCommandDetails{LayerHash: layer.Digest, Command: formatCommand(layer)}
	}
	return dsc.mapDockerfileCommands(layersMapping)
}

func formatCommand(layer *image.Layer) string {
	command := trimSpacesInMiddle(layer.Command)
	command = strings.TrimSuffix(command, buildKitSuffix)
	if len(command) > maxDisplayCommandLength {
		command = command[:maxDisplayCommandLength] + " ..."
	}
	return command
}

func trimSpacesInMiddle(input string) string {
	parts := strings.Fields(input) // Split the string by spaces
	return strings.Join(parts, " ")
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

const (
	emptyDockerfileLine     = ""
	dockerfileCommentPrefix = "#"
	backslash               = "\\"
	fromCommand             = "FROM"
)

// Scans the dockerfile line by line and match docker commands to their respective lines.
// Lines which don't appear in dockerfile would get assigned to the corresponding FROM command.
func (dsc *DockerScanCommand) mapDockerfileCommands(dockerCommandsMap map[string]services.DockerfileCommandDetails) (map[string]services.DockerfileCommandDetails, error) {
	if dsc.scanner == nil {
		return dockerCommandsMap, nil
	}
	lineNumber := 1
	fromLineNumber := 1
	firstAppearanceFrom := true
	// Scan dockerfile line by line.
	for dsc.scanner.Scan() {
		scannedCommand := dsc.scanner.Text()
		// Skip comments in the dockerfile
		if strings.HasPrefix(scannedCommand, dockerfileCommentPrefix) || scannedCommand == emptyDockerfileLine {
			lineNumber++
			continue
		}
		// Read the next line as it is the same command.
		for strings.HasSuffix(scannedCommand, backslash) {
			dsc.scanner.Scan()
			lineNumber++
			scannedCommand += dsc.scanner.Text()
		}
		// Assign all the unassigned commands to the FROM command before moving on.
		if strings.Contains(scannedCommand, fromCommand) {
			if !firstAppearanceFrom {
				for key := range dockerCommandsMap {
					current := dockerCommandsMap[key]
					if len(current.Line) == 0 {
						current.Line = append(current.Line, strconv.Itoa(fromLineNumber))
					}
					dockerCommandsMap[key] = current
				}
			}
			fromLineNumber = lineNumber
			firstAppearanceFrom = false
		}

		// TODO optimize this
		for sha256, dockerfileCommandDetails := range dockerCommandsMap {
			current := dockerCommandsMap[sha256]
			if CommandContains(dockerfileCommandDetails.Command, scannedCommand) {
				current.Line = append(current.Line, strconv.Itoa(lineNumber))
				dockerCommandsMap[sha256] = current
				break
			}
		}
		lineNumber++
	}

	// Iterate again and assign all unassigned commands to that nearest FROM command
	for key := range dockerCommandsMap {
		current := dockerCommandsMap[key]
		if len(current.Line) == 0 {
			current.Line = append(current.Line, strconv.Itoa(fromLineNumber))
			dockerCommandsMap[key] = current
		}
	}
	// Check for scanner errors
	if err := dsc.scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning dockerfile:%s", err.Error())
	}
	return dockerCommandsMap, nil
}
func CommandContains(commandFromLayer, scannedCommand string) bool {
	// Normalize and split the commands into arguments
	args1 := strings.Fields(commandFromLayer)
	args2 := strings.Fields(scannedCommand)
	// Create a map to store the arguments of commandFromLayer
	argMap1 := make(map[string]bool)
	for _, arg := range args1 {
		argMap1[arg] = true
	}
	// Check if all arguments of scannedCommand are present in commandFromLayer
	for _, arg := range args2 {
		if !argMap1[arg] {
			return false
		}
	}
	return true
}
