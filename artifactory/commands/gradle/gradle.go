package gradle

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/generic"
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	gradleutils "github.com/jfrog/jfrog-cli-core/v2/utils/gradle"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	xraycommands "github.com/jfrog/jfrog-cli-core/v2/xray/commands"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

type GradleCommand struct {
	tasks            string
	configPath       string
	configuration    *utils.BuildConfiguration
	serverDetails    *config.ServerDetails
	threads          int
	detailedSummary  bool
	xrayScan         bool
	scanOutputFormat xraycommands.OutputFormat
	result           *commandsutils.Result
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
	}
	return gc.serverDetails, err
}

func (gc *GradleCommand) SetServerDetails(serverDetails *config.ServerDetails) *GradleCommand {
	gc.serverDetails = serverDetails
	return gc
}

func (gc *GradleCommand) Run() error {
	deployableArtifactsFile := ""
	if gc.IsDetailedSummary() || gc.IsXrayScan() {
		tempFile, err := fileutils.CreateTempFile()
		if err != nil {
			return err
		}
		// If this is a Windows machine there is a need to modify the path for the build info file to match Java syntax with double \\
		deployableArtifactsFile = ioutils.DoubleWinPathSeparator(tempFile.Name())
		tempFile.Close()
	}

	err := gradleutils.RunGradle(gc.tasks, gc.configPath, deployableArtifactsFile, gc.configuration, gc.threads, false, gc.IsXrayScan())
	if err != nil {
		return err
	}
	if gc.IsXrayScan() {
		err = gc.unmarshalDeployableArtifacts(deployableArtifactsFile)
		if err != nil {
			return err
		}
		return gc.conditionalUpload()
	}
	if gc.IsDetailedSummary() {
		return gc.unmarshalDeployableArtifacts(deployableArtifactsFile)
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
	gc.ServerDetails()
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

func (gc *GradleCommand) SetTasks(tasks string) *GradleCommand {
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

func (gc *GradleCommand) SetScanOutputFormat(format xraycommands.OutputFormat) *GradleCommand {
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
