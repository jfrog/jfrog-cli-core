package oc

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/container"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/oc"
	"os/exec"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/version"
)

const minSupportedOcVersion = "3.0.0"

type OcStartBuildCommand struct {
	executablePath     string
	repo               string
	ocArgs             []string
	threads            int
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

	var filteredOcArgs []string
	var serverId string
	osb.repo, serverId, osb.threads, osb.buildConfiguration, filteredOcArgs, err = extractOcOptionsFromArgs(osb.ocArgs)
	if err != nil {
		return err
	}
	osb.serverDetails, err = config.GetSpecificConfig(serverId, true, true)
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
	ocBuildName, err := oc.StartBuild(osb.executablePath, filteredOcArgs)
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
	imageTag, manifestSha256, err := oc.GetImageDetails(osb.executablePath, ocBuildName)
	if err != nil {
		return err
	}

	if err := utils.SaveBuildGeneralDetails(buildName, buildNumber, project); err != nil {
		return err
	}
	serviceManager, err := utils.CreateServiceManagerWithThreads(osb.serverDetails, false, osb.threads, -1)
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

func (osb *OcStartBuildCommand) SetArgs(args []string) *OcStartBuildCommand {
	osb.ocArgs = args
	return osb
}

func (osb *OcStartBuildCommand) validateAllowedOptions() error {
	notAllowedOcOptions := []string{"-w", "--wait", "--template", "-o", "--output"}
	for _, arg := range osb.ocArgs {
		for _, optionName := range notAllowedOcOptions {
			if arg == optionName || strings.HasPrefix(arg, optionName+"=") {
				return errorutils.CheckError(errors.New(fmt.Sprintf("the %s option is not allowed", optionName)))
			}
		}
	}
	return nil
}

func (osb *OcStartBuildCommand) validateOcVersion() error {
	ocVersionStr, err := oc.Version(osb.executablePath)
	if err != nil {
		return err
	}
	trimmedVersion := strings.TrimPrefix(ocVersionStr, "v")
	ocVersion := version.NewVersion(trimmedVersion)
	if ocVersion.Compare(minSupportedOcVersion) > 0 {
		return errorutils.CheckError(errors.New(fmt.Sprintf(
			"JFrog CLI oc start-build command requires OpenShift CLI version " + minSupportedOcVersion + " or higher")))
	}
	return nil
}

func extractOcOptionsFromArgs(args []string) (repo, serverId string, threads int, buildConfig *utils.BuildConfiguration, cleanArgs []string, err error) {
	// Extract repo
	flagIndex, valueIndex, repo, err := coreutils.FindFlag("--repo", args)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&args, flagIndex, valueIndex)
	if flagIndex == -1 {
		err = errorutils.CheckError(errors.New("the --repo option is mandatory"))
		return
	}

	// Extract server-id
	flagIndex, valueIndex, serverId, err = coreutils.FindFlag("--server-id", args)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&args, flagIndex, valueIndex)

	// Extract threads
	threads = 3
	var numOfThreads string
	flagIndex, valueIndex, numOfThreads, err = coreutils.FindFlag("--threads", args)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&args, flagIndex, valueIndex)
	if numOfThreads != "" {
		threads, err = strconv.Atoi(numOfThreads)
		if err != nil {
			err = errorutils.CheckError(err)
			return
		}
	}

	// Extract build details
	cleanArgs, buildConfig, err = utils.ExtractBuildDetailsFromArgs(args)

	return
}
