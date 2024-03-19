package oc

import (
	"bufio"
	"encoding/json"
	"errors"
	"github.com/jfrog/gofrog/version"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/container"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const minSupportedOcVersion = "3.0.0"

type OcStartBuildCommand struct {
	executablePath     string
	serverId           string
	repo               string
	ocArgs             []string
	serverDetails      *config.ServerDetails
	buildConfiguration *build.BuildConfiguration
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
	buildName, err := osb.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := osb.buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}
	project := osb.buildConfiguration.GetProject()
	if buildName == "" {
		return nil
	}

	log.Info("Collecting build info...")
	// Get the new image details from OpenShift
	imageTag, manifestSha256, err := getImageDetails(osb.executablePath, ocBuildName)
	if err != nil {
		return err
	}

	if err := build.SaveBuildGeneralDetails(buildName, buildNumber, project); err != nil {
		return err
	}
	serviceManager, err := utils.CreateServiceManager(osb.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}
	image := container.NewImage(imageTag)
	builder, err := container.NewRemoteAgentBuildInfoBuilder(image, osb.repo, buildName, buildNumber, project, serviceManager, manifestSha256)
	if err != nil {
		return err
	}
	buildInfo, err := builder.Build(osb.buildConfiguration.GetModule())
	if err != nil {
		return err
	}

	log.Info("oc start-build finished successfully.")
	return build.SaveBuildInfo(buildName, buildNumber, project, buildInfo)
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

func (osb *OcStartBuildCommand) SetBuildConfiguration(buildConfiguration *build.BuildConfiguration) *OcStartBuildCommand {
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
	outputPr, outputPw, err := os.Pipe()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	outputWriter := io.MultiWriter(os.Stderr, outputPw)
	cmd.Stdout = outputWriter
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	// The build name is in the first line of the output
	scanner := bufio.NewScanner(outputPr)
	scanner.Scan()
	ocBuildName = scanner.Text()

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
		// If an error occurred, maybe it's because the '-o' flag is not supported in OpenShift CLI v3 and below.
		// Try executing this command without this flag.
		return getOldOcVersion(executablePath)
	}
	var versionRes ocVersionResponse
	err = json.Unmarshal(outputBytes, &versionRes)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	return versionRes.ClientVersion.GitVersion, nil
}

// Running 'oc version' without the '-o=json' flag that is not supported in OpenShift CLI v3 and below.
func getOldOcVersion(executablePath string) (string, error) {
	outputBytes, err := exec.Command(executablePath, "version").Output()
	if err != nil {
		return "", errorutils.CheckError(convertExitError(err))
	}
	// In OpenShift CLI v3 the output of 'oc version' looks like this:
	// oc v3.0.0
	// kubernetes v1.11.0
	// [...]
	// Get the first line of the output
	scanner := bufio.NewScanner(strings.NewReader(string(outputBytes)))
	scanner.Scan()
	firstLine := scanner.Text()
	if !strings.HasPrefix(firstLine, "oc v") {
		return "", errorutils.CheckErrorf("Could not parse OpenShift CLI version. JFrog CLI oc start-build command requires OpenShift CLI version " + minSupportedOcVersion + " or higher.")
	}
	return strings.TrimPrefix(firstLine, "oc "), nil
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
