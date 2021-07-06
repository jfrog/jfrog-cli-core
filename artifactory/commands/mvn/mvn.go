package mvn

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/generic"
	commandsutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/commands/mvn"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

const (
	mavenExtractorDependencyVersion = "2.27.0"
	classworldsConfFileName         = "classworlds.conf"
	MavenHome                       = "M2_HOME"
)

type MvnCommand struct {
	goals           []string
	configPath      string
	insecureTls     bool
	configuration   *utils.BuildConfiguration
	serverDetails   *config.ServerDetails
	threads         int
	detailedSummary bool
	xrayScan        bool
	result          *commandsutils.Result
	disableDeploy   bool
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

func (mc *MvnCommand) Result() *commandsutils.Result {
	return mc.result
}

func (mc *MvnCommand) SetResult(result *commandsutils.Result) *MvnCommand {
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
		deployableArtifactsFile = tempFile.Name()
		tempFile.Close()
	}

	err := mvn.RunMvn(mc.configPath, deployableArtifactsFile, mc.configuration, mc.goals, mc.threads, mc.insecureTls, mc.IsXrayScan())
	if err != nil {
		return err
	}

	if mc.IsDetailedSummary() || mc.IsXrayScan() {
		return mc.unmarshalDeployableArtifacts(deployableArtifactsFile)
	}
	if mc.IsXrayScan() {
		return mc.conditionalUpload()
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
	result, err := commandsutils.UnmarshalDeployableArtifacts(filesPath)
	if err != nil {
		return err
	}
	mc.SetResult(result)
	return nil
}

func (mc *MvnCommand) CommandName() string {
	return "rt_maven"
}

func (mc *MvnCommand) conditionalUpload() error {
	binariesSpecFile, pomSpecFile, err := commandsutils.ScanDeployableArtifacts(mc.result, mc.serverDetails)
	// First upload binaries
	uploadCmd := generic.NewUploadCommand()
	uploadCmd.SetBuildConfiguration(mc.configuration).SetSpec(binariesSpecFile).SetServerDetails(mc.serverDetails)
	err = uploadCmd.Run()
	if err != nil {
		return err
	}
	// Then Upload pom.xml's
	uploadCmd = generic.NewUploadCommand()
	uploadCmd.SetBuildConfiguration(mc.configuration).SetSpec(pomSpecFile).SetServerDetails(mc.serverDetails)
	return uploadCmd.Run()
}
