package gradle

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/generic"
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	gradleutils "github.com/jfrog/jfrog-cli-core/v2/utils/gradle"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/spf13/viper"
)

type GradleCommand struct {
	tasks              []string
	configPath         string
	configuration      *utils.BuildConfiguration
	serverDetails      *config.ServerDetails
	threads            int
	detailedSummary    bool
	xrayScan           bool
	scanOutputFormat   xrutils.OutputFormat
	result             *commandsutils.Result
	deploymentDisabled bool
	// File path for Gradle extractor in which all build's artifacts details will be listed at the end of the build.
	buildArtifactsDetailsFile string
}

func NewGradleCommand() *GradleCommand {
	return &GradleCommand{}
}

// Returns the ServerDetails. The information returns from the config file provided.
func (gc *GradleCommand) ServerDetails() (*config.ServerDetails, error) {
	// Get the serverDetails from the config file.
	var err error
	if gc.serverDetails == nil {
		vConfig, err := utils.ReadConfigFile(gc.configPath, utils.YAML)
		if err != nil {
			return nil, err
		}
		gc.serverDetails, err = utils.GetServerDetails(vConfig)
		if err != nil {
			return nil, err
		}
	}
	return gc.serverDetails, err
}

func (gc *GradleCommand) SetServerDetails(serverDetails *config.ServerDetails) *GradleCommand {
	gc.serverDetails = serverDetails
	return gc
}

func (gc *GradleCommand) init() (vConfig *viper.Viper, err error) {
	// Read config
	vConfig, err = utils.ReadConfigFile(gc.configPath, utils.YAML)
	if err != nil {
		return
	}
	if gc.IsXrayScan() && !vConfig.IsSet("deployer") {
		err = errorutils.CheckErrorf("Conditional upload can only be performed if deployer is set in the config")
		return
	}
	// Gradle extractor is needed to run, in order to get the details of the build's artifacts.
	// Gradle's extractor deploy build artifacts. This should be disabled since there is no intent to deploy anything or deploy upon Xray scan results.
	gc.deploymentDisabled = gc.IsXrayScan() || !vConfig.IsSet("deployer")
	if gc.shouldCreateBuildArtifactsFile() {
		// Created a file that will contain all the details about the build's artifacts
		tempFile, err := fileutils.CreateTempFile()
		if err != nil {
			return nil, err
		}
		// If this is a Windows machine there is a need to modify the path for the build info file to match Java syntax with double \\
		gc.buildArtifactsDetailsFile = ioutils.DoubleWinPathSeparator(tempFile.Name())
		if err = tempFile.Close(); errorutils.CheckError(err) != nil {
			return nil, err
		}
	}
	return
}

// Gradle extractor generates the details of the build's artifacts.
// This is required for Xray scan and for the detailed summary.
// We can either scan or print the generated artifacts.
func (gc *GradleCommand) shouldCreateBuildArtifactsFile() bool {
	return (gc.IsDetailedSummary() && !gc.deploymentDisabled) || gc.IsXrayScan()
}

func (gc *GradleCommand) Run() error {
	vConfig, err := gc.init()
	if err != nil {
		return err
	}
	err = gradleutils.RunGradle(vConfig, gc.tasks, gc.buildArtifactsDetailsFile, gc.configuration, gc.threads, gc.IsXrayScan())
	if err != nil {
		return err
	}
	if gc.buildArtifactsDetailsFile != "" {
		err = gc.unmarshalDeployableArtifacts(gc.buildArtifactsDetailsFile)
		if err != nil {
			return err
		}
		if gc.IsXrayScan() {
			return gc.conditionalUpload()
		}
	}
	return nil
}

func (gc *GradleCommand) unmarshalDeployableArtifacts(filesPath string) error {
	result, err := commandsutils.UnmarshalDeployableArtifacts(filesPath, gc.configPath, gc.IsXrayScan())
	if err != nil {
		return err
	}
	gc.setResult(result)
	return nil
}

// ConditionalUpload will scan the artifact using Xray and will upload them only if the scan passes with no
// violation.
func (gc *GradleCommand) conditionalUpload() error {
	// Initialize the server details (from config) if it hasn't been initialized yet.
	_, err := gc.ServerDetails()
	if err != nil {
		return err
	}
	binariesSpecFile, pomSpecFile, err := commandsutils.ScanDeployableArtifacts(gc.result, gc.serverDetails, gc.threads, gc.scanOutputFormat)
	// If the detailed summary wasn't requested, the reader should be closed here.
	// (otherwise it will be closed by the detailed summary print method)
	if !gc.detailedSummary {
		e := gc.result.Reader().Close()
		if e != nil {
			return e
		}
	} else {
		gc.result.Reader().Reset()
	}
	if err != nil {
		return err
	}
	// The case scan failed
	if binariesSpecFile == nil {
		return nil
	}
	// First upload binaries
	if len(binariesSpecFile.Files) > 0 {
		uploadCmd := generic.NewUploadCommand()
		uploadConfiguration := new(utils.UploadConfiguration)
		uploadConfiguration.Threads = gc.threads
		uploadCmd.SetUploadConfiguration(uploadConfiguration).SetBuildConfiguration(gc.configuration).SetSpec(binariesSpecFile).SetServerDetails(gc.serverDetails)
		err = uploadCmd.Run()
		if err != nil {
			return err
		}
	}
	if len(pomSpecFile.Files) > 0 {
		// Then Upload pom.xml's
		uploadCmd := generic.NewUploadCommand()
		uploadCmd.SetBuildConfiguration(gc.configuration).SetSpec(pomSpecFile).SetServerDetails(gc.serverDetails)
		err = uploadCmd.Run()
	}
	return err
}

func (gc *GradleCommand) CommandName() string {
	return "rt_gradle"
}

func (gc *GradleCommand) SetConfiguration(configuration *utils.BuildConfiguration) *GradleCommand {
	gc.configuration = configuration
	return gc
}

func (gc *GradleCommand) SetConfigPath(configPath string) *GradleCommand {
	gc.configPath = configPath
	return gc
}

func (gc *GradleCommand) SetTasks(tasks []string) *GradleCommand {
	gc.tasks = tasks
	return gc
}

func (gc *GradleCommand) SetThreads(threads int) *GradleCommand {
	gc.threads = threads
	return gc
}

func (gc *GradleCommand) SetDetailedSummary(detailedSummary bool) *GradleCommand {
	gc.detailedSummary = detailedSummary
	return gc
}

func (gc *GradleCommand) IsDetailedSummary() bool {
	return gc.detailedSummary
}

func (gc *GradleCommand) SetXrayScan(xrayScan bool) *GradleCommand {
	gc.xrayScan = xrayScan
	return gc
}

func (gc *GradleCommand) IsXrayScan() bool {
	return gc.xrayScan
}

func (gc *GradleCommand) SetScanOutputFormat(format xrutils.OutputFormat) *GradleCommand {
	gc.scanOutputFormat = format
	return gc
}

func (gc *GradleCommand) Result() *commandsutils.Result {
	return gc.result
}

func (gc *GradleCommand) setResult(result *commandsutils.Result) *GradleCommand {
	gc.result = result
	return gc
}
