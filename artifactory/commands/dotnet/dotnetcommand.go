package dotnet

import (
	"fmt"
	"github.com/jfrog/build-info-go/build"
	"github.com/jfrog/build-info-go/build/utils/dotnet"
	"github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
)

const SourceName = "JFrogCli"

type DotnetCommand struct {
	toolchainType      dotnet.ToolchainType
	subCommand         string
	argAndFlags        []string
	repoName           string
	solutionPath       string
	useNugetAddSource  bool
	useNugetV2         bool
	buildConfiguration *utils.BuildConfiguration
	serverDetails      *config.ServerDetails
	buildInfoModule    *build.DotnetModule
}

func (dc *DotnetCommand) SetServerDetails(serverDetails *config.ServerDetails) *DotnetCommand {
	dc.serverDetails = serverDetails
	return dc
}

func (dc *DotnetCommand) SetBuildConfiguration(buildConfiguration *utils.BuildConfiguration) *DotnetCommand {
	dc.buildConfiguration = buildConfiguration
	return dc
}

func (dc *DotnetCommand) SetToolchainType(toolchainType dotnet.ToolchainType) *DotnetCommand {
	dc.toolchainType = toolchainType
	return dc
}

func (dc *DotnetCommand) SetSolutionPath(solutionPath string) *DotnetCommand {
	dc.solutionPath = solutionPath
	return dc
}

func (dc *DotnetCommand) SetRepoName(repoName string) *DotnetCommand {
	dc.repoName = repoName
	return dc
}

func (dc *DotnetCommand) SetUseNugetV2(useNugetV2 bool) *DotnetCommand {
	dc.useNugetV2 = useNugetV2
	return dc
}

func (dc *DotnetCommand) SetArgAndFlags(argAndFlags []string) *DotnetCommand {
	dc.argAndFlags = argAndFlags
	return dc
}

func (dc *DotnetCommand) SetBasicCommand(subCommand string) *DotnetCommand {
	dc.subCommand = subCommand
	return dc
}

func (dc *DotnetCommand) SetUseNugetAddSource(useNugetAddSource bool) *DotnetCommand {
	dc.useNugetAddSource = useNugetAddSource
	return dc
}

func (dc *DotnetCommand) ServerDetails() (*config.ServerDetails, error) {
	return dc.serverDetails, nil
}

func (dc *DotnetCommand) CommandName() string {
	return "rt_" + dc.toolchainType.String()
}

// Exec all consume type nuget commands, install, update, add, restore.
func (dc *DotnetCommand) Exec() error {
	log.Info("Running " + dc.toolchainType.String() + "...")

	buildName, err := dc.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := dc.buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}

	buildInfoService := utils.CreateBuildInfoService()
	npmBuild, err := buildInfoService.GetOrCreateBuildWithProject(buildName, buildNumber, dc.buildConfiguration.GetProject())
	if err != nil {
		return errorutils.CheckError(err)
	}
	dc.buildInfoModule, err = npmBuild.AddDotnetModule(dc.solutionPath)
	if err != nil {
		return errorutils.CheckError(err)
	}
	if dc.buildConfiguration.GetModule() != "" {
		dc.buildInfoModule.SetName(dc.buildConfiguration.GetModule())
	}
	// TODO: setargs
	//dc.buildInfoModule.SetArgs(filteredYarnArgs)
	// TODO: remove logger
	if err = dc.buildInfoModule.Build(log.Logger()); err != nil {
		//return dc.restoreConfigurationsAndError(err)
		return err
	}
	log.Info(fmt.Sprintf("%s finished successfully.", dc.toolchainType))
	return nil
}

// Set Artifactory repo as source using the toolchain's `add source` command
func (dc *DotnetCommand) AddNugetAuthToConfig(cmdType dotnet.ToolchainType, configFile *os.File, sourceUrl, user, password string) error {
	content := dotnet.ConfigFileTemplate
	_, err := configFile.WriteString(content)
	if err != nil {
		return errorutils.CheckError(err)
	}
	// We need to close the config file to let the toolchain modify it.
	err = configFile.Close()
	if err != nil {
		return errorutils.CheckError(err)
	}
	return addSourceToNugetConfig(cmdType, configFile.Name(), sourceUrl, user, password)
}

// Runs nuget sources add command
func addSourceToNugetConfig(cmdType dotnet.ToolchainType, configFileName, sourceUrl, user, password string) error {
	cmd, err := dotnet.CreateDotnetAddSourceCmd(cmdType, sourceUrl)
	if err != nil {
		return err
	}

	flagPrefix := cmdType.GetTypeFlagPrefix()
	cmd.CommandFlags = append(cmd.CommandFlags, flagPrefix+"configfile", configFileName)
	cmd.CommandFlags = append(cmd.CommandFlags, flagPrefix+"name", SourceName)
	cmd.CommandFlags = append(cmd.CommandFlags, flagPrefix+"username", user)
	cmd.CommandFlags = append(cmd.CommandFlags, flagPrefix+"password", password)
	output, err := io.RunCmdOutput(cmd)
	log.Debug("Running command: Add sources. Output:", output)
	return err
}
