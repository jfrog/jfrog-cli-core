package oc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/container"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/version"
)

const minSupportedOcVersion = "3.0.0"

type OcStartBuildCommand struct {
	executablePath     string
	serverId           string
	repo               string
	ocArgs             []string
	serverDetails      *config.ServerDetails
	buildConfiguration *utils.BuildConfiguration
}

func NewOcStartBuildCommand() *OcStartBuildCommand {
	return &OcStartBuildCommand{}
}

func (osb *OcStartBuildCommand) Run() error {
	log.Info("Running oc start-build...")
	var err error
	if err = osb.validateAllowedOptions(); err != nil {
		return err
	}

	osb.serverDetails, err = config.GetSpecificConfig(osb.serverId, true, true)
	if err != nil {
		return err
	}

	if err = osb.setOcExecutable(); err != nil {
		return err
	}
	if err = osb.validateOcVersion(); err != nil {
		return err
	}

	// Run the build on OpenShift
	ocBuildName, err := startBuild(osb.executablePath, osb.ocArgs)
	if err != nil {
		return err
	}
	log.Info("Build", ocBuildName, "finished successfully.")

	buildName := osb.buildConfiguration.BuildName
	buildNumber := osb.buildConfiguration.BuildNumber
	project := osb.buildConfiguration.Project

	if buildName == "" {
		return nil
	}

	log.Info("Collecting build info...")
	// Get the new image details from OpenShift
	imageTag, manifestSha256, err := getImageDetails(osb.executablePath, ocBuildName)
	if err != nil {
		return err
	}

	if err := utils.SaveBuildGeneralDetails(buildName, buildNumber, project); err != nil {
		return err
	}
	serviceManager, err := utils.CreateServiceManager(osb.serverDetails, -1, false)
	if err != nil {
		return err
	}
	image := container.NewImage(imageTag)
	builder, err := container.NewBuildInfoBuilderForKanikoOrOpenShift(image, osb.repo, buildName, buildNumber, project, serviceManager, container.Push, manifestSha256)
	if err != nil {
		return err
	}
	buildInfo, err := builder.Build(osb.buildConfiguration.Module)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("oc start-build finished successfully."))
	return utils.SaveBuildInfo(buildName, buildNumber, project, buildInfo)
}

func (osb *OcStartBuildCommand) ServerDetails() (*config.ServerDetails, error) {
	return osb.serverDetails, nil
}

func (osb *OcStartBuildCommand) CommandName() string {
	return "rt_oc_start_build"
}

func (osb *OcStartBuildCommand) setOcExecutable() error {
	ocExecPath, err := exec.LookPath("oc")
	if err != nil {
		return errorutils.CheckError(err)
	}

	osb.executablePath = ocExecPath
	log.Debug("Found OpenShift CLI executable at:", osb.executablePath)
	return nil
}

func (osb *OcStartBuildCommand) SetOcArgs(args []string) *OcStartBuildCommand {
	osb.ocArgs = args
	return osb
}

func (osb *OcStartBuildCommand) SetRepo(repo string) *OcStartBuildCommand {
	osb.repo = repo
	return osb
}

func (osb *OcStartBuildCommand) SetServerId(serverId string) *OcStartBuildCommand {
	osb.serverId = serverId
	return osb
}

func (osb *OcStartBuildCommand) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *OcStartBuildCommand {
	osb.buildConfiguration = buildConfiguration
	return osb
}

func (osb *OcStartBuildCommand) validateAllowedOptions() error {
	notAllowedOcOptions := []string{"-w", "--wait", "--template", "-o", "--output"}
	for _, arg := range osb.ocArgs {
		for _, optionName := range notAllowedOcOptions {
			if arg == optionName || strings.HasPrefix(arg, optionName+"=") {
				return errorutils.CheckErrorf("the %s option is not allowed", optionName)
			}
		}
	}
	return nil
}

func (osb *OcStartBuildCommand) validateOcVersion() error {
	ocVersionStr, err := getOcVersion(osb.executablePath)
	if err != nil {
		return err
	}
	trimmedVersion := strings.TrimPrefix(ocVersionStr, "v")
	ocVersion := version.NewVersion(trimmedVersion)
	if ocVersion.Compare(minSupportedOcVersion) > 0 {
		return errorutils.CheckErrorf(
			"JFrog CLI oc start-build command requires OpenShift CLI version " + minSupportedOcVersion + " or higher")
	}
	return nil
}

func startBuild(executablePath string, ocFlags []string) (ocBuildName string, err error) {
	cmdArgs := []string{"start-build", "-w", "--template={{.metadata.name}}{{\"\\n\"}}"}
	cmdArgs = append(cmdArgs, ocFlags...)

	log.Debug("Running command: oc", strings.Join(cmdArgs, " "))
	cmd := exec.Command(executablePath, cmdArgs...)
	outputReader, err := cmd.StdoutPipe()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	// The build name is in the first line of the output
	scanner := bufio.NewScanner(outputReader)
	scanner.Scan()
	ocBuildName = scanner.Text()

	// Print the output to stderr
	_, err = io.Copy(os.Stderr, outputReader)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	err = errorutils.CheckError(convertExitError(cmd.Wait()))
	return
}

func getImageDetails(executablePath, ocBuildName string) (imageTag, manifestSha256 string, err error) {
	cmdArgs := []string{"get", "build", ocBuildName, "--template={{.status.outputDockerImageReference}}@{{.status.output.to.imageDigest}}"}
	log.Debug("Running command: oc", strings.Join(cmdArgs, " "))
	outputBytes, err := exec.Command(executablePath, cmdArgs...).Output()
	if err != nil {
		return "", "", errorutils.CheckError(convertExitError(err))
	}
	output := string(outputBytes)
	splitOutput := strings.Split(strings.TrimSpace(output), "@")
	if len(splitOutput) != 2 {
		return "", "", errorutils.CheckErrorf("Unable to parse image tag and digest of build %s. Output from OpenShift CLI: %s", ocBuildName, output)
	}

	return splitOutput[0], splitOutput[1], nil
}

func getOcVersion(executablePath string) (string, error) {
	cmdArgs := []string{"version", "-o=json"}
	outputBytes, err := exec.Command(executablePath, cmdArgs...).Output()
	if err != nil {
		return "", errorutils.CheckError(convertExitError(err))
	}
	var versionRes ocVersionResponse
	err = json.Unmarshal(outputBytes, &versionRes)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	return versionRes.ClientVersion.GitVersion, nil
}

func convertExitError(err error) error {
	if exitError, ok := err.(*exec.ExitError); ok {
		return errors.New(string(exitError.Stderr))
	}
	return err
}

type ocVersionResponse struct {
	ClientVersion clientVersion `json:"clientVersion,omitempty"`
}

type clientVersion struct {
	GitVersion string `json:"gitVersion,omitempty"`
}
