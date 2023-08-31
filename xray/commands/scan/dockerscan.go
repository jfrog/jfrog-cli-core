package scan

import (
	"bytes"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/utils"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
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
)

type DockerScanCommand struct {
	ScanCommand
	imageTag       string
	targetRepoPath string
	dockerFilePath string
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
	_, xrayVersion, err := utils.CreateXrayServiceManagerAndGetVersion(dsc.ScanCommand.serverDetails)
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

	if dsc.imageTag == "" {
		//if exists, _ := fileutils.IsFileExists(dsc.dockerFilePath, false); !exists {
		//	return fmt.Errorf("didn't find Dockerfile in the provided path: %s", dsc.dockerFilePath)
		//}
		if dsc.progress != nil {
			dsc.progress.SetHeadlineMsg("Building Docker image 🏗️...")
		}
		log.Info("Building docker image")
		dsc.imageTag = "audittag"

		dockerBuildCommand := exec.Command("docker", "build", ".", "-f", ".dockerfile", "-t", dsc.imageTag)
		if err = dockerBuildCommand.Run(); err != nil {
			return fmt.Errorf("failed to build docker image,error: %s", err.Error())
		}
		log.Info("successfully build docker image from dockerfile")
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

	dockerCommandsMapping, err := getDockerCommandsMapping(dsc.imageTag)
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

	// Replace sha_256 with commands
	for _, res := range extendedScanResults.XrayResults {
		for _, vul := range res.Vulnerabilities {
			for _, cop := range vul.Components {
				compos := &cop.ImpactPaths[0][1]
				suffix := strings.TrimSuffix(strings.TrimPrefix(compos.FullPath, "sha256__"), ".tar")[:50]
				asn := dockerCommandsMapping[suffix]
				compos.FullPath = asn
				compos.ComponentId = asn
			}
		}
	}

	// Print results
	if err = xrutils.PrintScanResults(extendedScanResults,
		scanErrors,
		dsc.ScanCommand.outputFormat,
		dsc.ScanCommand.includeVulnerabilities,
		dsc.ScanCommand.includeLicenses,
		true,
		dsc.ScanCommand.printExtendedTable, true, nil,
	); err != nil {
		return
	}
	return dsc.ScanCommand.handlePossibleErrors(extendedScanResults.XrayResults, scanErrors, err)
}

func getDockerCommandsMapping(imageTag string) (layers map[string]string, err error) {
	resolver, err := dive.GetImageResolver(1)
	if err != nil {
		return
	}
	dockerImage, err := resolver.Fetch(imageTag)
	if err != nil {
		return
	}
	// sha256 digest -> command
	commandsMapping := make(map[string]string)
	for _, layer := range dockerImage.Layers {
		commandsMapping[strings.TrimPrefix(layer.Digest, "sha256:")] = strings.TrimSuffix(layer.Command, "# buildkit")
	}
	return commandsMapping, nil
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
