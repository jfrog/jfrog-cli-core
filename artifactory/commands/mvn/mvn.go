package mvn

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/generic"
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	mvnutils "github.com/jfrog/jfrog-cli-core/v2/utils/mvn"
	xraycommands "github.com/jfrog/jfrog-cli-core/v2/xray/commands"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

type MvnCommand struct {
	goals            []string
	configPath       string
	insecureTls      bool
	configuration    *utils.BuildConfiguration
	serverDetails    *config.ServerDetails
	threads          int
	detailedSummary  bool
	xrayScan         bool
	scanOutputFormat xraycommands.OutputFormat
	result           *commandsutils.Result
	disableDeploy    bool
}

func NewMvnCommand() *MvnCommand {
	return &MvnCommand{}
}

func (mc *MvnCommand) SetServerDetails(serverDetails *config.ServerDetails) *MvnCommand {
	mc.serverDetails = serverDetails
	return mc
}

func (mc *MvnCommand) SetConfiguration(configuration *utils.BuildConfiguration) *MvnCommand {
	mc.configuration = configuration
	return mc
}

func (mc *MvnCommand) SetConfigPath(configPath string) *MvnCommand {
	mc.configPath = configPath
	return mc
}

func (mc *MvnCommand) SetGoals(goals []string) *MvnCommand {
	mc.goals = goals
	return mc
}

func (mc *MvnCommand) SetThreads(threads int) *MvnCommand {
	mc.threads = threads
	return mc
}

func (mc *MvnCommand) SetInsecureTls(insecureTls bool) *MvnCommand {
	mc.insecureTls = insecureTls
	return mc
}

func (mc *MvnCommand) SetDetailedSummary(detailedSummary bool) *MvnCommand {
	mc.detailedSummary = detailedSummary
	return mc
}

func (mc *MvnCommand) IsDetailedSummary() bool {
	return mc.detailedSummary
}

func (mc *MvnCommand) SetXrayScan(xrayScan bool) *MvnCommand {
	mc.xrayScan = xrayScan
	return mc
}

func (mc *MvnCommand) IsXrayScan() bool {
	return mc.xrayScan
}

func (mc *MvnCommand) SetScanOutputFormat(format xraycommands.OutputFormat) *MvnCommand {
	mc.scanOutputFormat = format
	return mc
}

func (mc *MvnCommand) Result() *commandsutils.Result {
	return mc.result
}

func (mc *MvnCommand) setResult(result *commandsutils.Result) *MvnCommand {
	mc.result = result
	return mc
}

func (mc *MvnCommand) SetDisableDeploy(disableDeploy bool) *MvnCommand {
	mc.disableDeploy = disableDeploy
	return mc
}

func (mc *MvnCommand) Run() error {
	deployableArtifactsFile := ""
	if mc.IsDetailedSummary() || mc.IsXrayScan() {
		tempFile, err := fileutils.CreateTempFile()
		if err != nil {
			return err
		}
		// If this is a Windows machine there is a need to modify the path for the build info file to match Java syntax with double \\
		deployableArtifactsFile = ioutils.DoubleWinPathSeparator(tempFile.Name())
		tempFile.Close()
	}

	err := mvnutils.RunMvn(mc.configPath, deployableArtifactsFile, mc.configuration, mc.goals, mc.threads, mc.insecureTls, mc.IsXrayScan())
	if err != nil {
		return err
	}
	if mc.IsXrayScan() {
		err = mc.unmarshalDeployableArtifacts(deployableArtifactsFile)
		if err != nil {
			return err
		}
		return mc.conditionalUpload()
	}
	if mc.IsDetailedSummary() {
		return mc.unmarshalDeployableArtifacts(deployableArtifactsFile)
	}
	return nil
}

// Returns the ServerDetails. The information returns from the config file provided.
func (mc *MvnCommand) ServerDetails() (*config.ServerDetails, error) {
	// Get the serverDetails from the config file.
	var err error
	if mc.serverDetails == nil {
		vConfig, err := utils.ReadConfigFile(mc.configPath, utils.YAML)
		if err != nil {
			return nil, err
		}
		mc.serverDetails, err = utils.GetServerDetails(vConfig)
	}
	return mc.serverDetails, err
}

func (mc *MvnCommand) unmarshalDeployableArtifacts(filesPath string) error {
	result, err := commandsutils.UnmarshalDeployableArtifacts(filesPath, mc.configPath, mc.IsXrayScan())
	if err != nil {
		return err
	}
	mc.setResult(result)
	return nil
}

func (mc *MvnCommand) CommandName() string {
	return "rt_maven"
}

// ConditionalUpload will scan the artifact using Xray and will upload them only if the scan passes with no
// violation.
func (mc *MvnCommand) conditionalUpload() error {
	// Initialize the server details (from config) if it hasn't been initialized yet.
	mc.ServerDetails()
	binariesSpecFile, pomSpecFile, err := commandsutils.ScanDeployableArtifacts(mc.result, mc.serverDetails, mc.threads, mc.scanOutputFormat)
	// If the detailed summary wasn't requested, the reader should be closed here.
	// (otherwise it will be closed by the detailed summary print method)
	if !mc.IsDetailedSummary() {
		e := mc.result.Reader().Close()
		if e != nil {
			return e
		}
	} else {
		mc.result.Reader().Reset()
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
		uploadConfiguration.Threads = mc.threads
		uploadCmd.SetUploadConfiguration(uploadConfiguration).SetBuildConfiguration(mc.configuration).SetSpec(binariesSpecFile).SetServerDetails(mc.serverDetails)
		err = uploadCmd.Run()
		if err != nil {
			return err
		}
	}
	if len(pomSpecFile.Files) > 0 {
		// Then Upload pom.xml's
		uploadCmd := generic.NewUploadCommand()
		uploadConfiguration := new(utils.UploadConfiguration)
		uploadConfiguration.Threads = mc.threads
		uploadCmd.SetUploadConfiguration(uploadConfiguration).SetBuildConfiguration(mc.configuration).SetSpec(pomSpecFile).SetServerDetails(mc.serverDetails)
		err = uploadCmd.Run()
	}
	return err
}
