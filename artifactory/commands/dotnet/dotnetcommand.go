package dotnet

import (
	"errors"
	"fmt"
	"github.com/jfrog/build-info-go/build"
	"github.com/jfrog/build-info-go/build/utils/dotnet"
	"github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"strings"
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

func (dc *DotnetCommand) GetToolchain() dotnet.ToolchainType {
	return dc.toolchainType
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
	dotnetBuild, err := buildInfoService.GetOrCreateBuildWithProject(buildName, buildNumber, dc.buildConfiguration.GetProject())
	if err != nil {
		return errorutils.CheckError(err)
	}
	dc.buildInfoModule, err = dotnetBuild.AddDotnetModule(dc.solutionPath)
	if err != nil {
		return errorutils.CheckError(err)
	}
	fallbackFunc, err := dc.prepareDotnetBuildInfoModule()
	if err != nil {
		return err
	}
	defer fallbackFunc()
	dc.buildInfoModule.SetArgsAndFlags(dc.argAndFlags)
	if err = dc.buildInfoModule.Build(); err != nil {
		return err
	}
	log.Info(fmt.Sprintf("%s finished successfully.", dc.toolchainType))
	return nil
}

// prepareDotnetBuildInfoModule prepare dotnet modules with the cli parameters and environment
func (dc *DotnetCommand) prepareDotnetBuildInfoModule() (func(), error) {
	dc.buildInfoModule.SetName(dc.buildConfiguration.GetModule())
	dc.buildInfoModule.SetSubcommand(dc.subCommand)
	// TODO: check if needed
	//dc.buildInfoModule.SetSolutionPath(dc.solutionPath)
	dc.buildInfoModule.SetArgAndFlags(dc.argAndFlags)
	dc.buildInfoModule.SetToolchainType(dc.toolchainType)
	cmd, err := dc.createCmd()
	if err != nil {
		return nil, err
	}
	return dc.prepareConfigFile(cmd)
}

func (dc *DotnetCommand) createCmd() (*dotnet.Cmd, error) {
	c, err := dotnet.NewToolchainCmd(dc.toolchainType)
	if err != nil {
		return nil, err
	}
	if dc.subCommand != "" {
		c.Command = append(c.Command, strings.Split(dc.subCommand, " ")...)
	}
	c.CommandFlags = dc.argAndFlags
	return c, nil
}

// Changes the working directory if provided.
// Returns the path to the solution
func changeWorkingDir(newWorkingDir string) (string, error) {
	var err error
	if newWorkingDir != "" {
		err = os.Chdir(newWorkingDir)
	} else {
		newWorkingDir, err = os.Getwd()
	}

	return newWorkingDir, err
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

// Checks if the user provided input such as -configfile flag or -Source flag.
// If those flags were provided, NuGet will use the provided configs (default config file or the one with -configfile)
// If neither provided, we are initializing our own config.
func (dc *DotnetCommand) prepareConfigFile(cmd *dotnet.Cmd) (func(), error) {
	// Use temp dir to save config file, so that config will be removed at the end.
	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		return nil, err
	}

	dc.solutionPath, err = changeWorkingDir(dc.solutionPath)
	if err != nil {
		return nil, err
	}
	cmdFlag := dc.GetToolchain().GetTypeFlagPrefix() + "configfile"
	currentConfigPath, err := getFlagValueIfExists(cmdFlag, cmd)
	if err != nil {
		return nil, err
	}
	if currentConfigPath != "" {
		return nil, nil
	}

	cmdFlag = cmd.GetToolchain().GetTypeFlagPrefix() + "source"
	sourceCommandValue, err := getFlagValueIfExists(cmdFlag, cmd)
	if err != nil {
		return nil, err
	}
	if sourceCommandValue != "" {
		return nil, nil
	}

	configFile, err := dc.InitNewConfig(tempDirPath)
	if err == nil {
		cmd.CommandFlags = append(cmd.CommandFlags, cmd.GetToolchain().GetTypeFlagPrefix()+"configfile", configFile.Name())
	}
	return func() {
		fileutils.RemoveTempDir(tempDirPath)
	}, err
}

// Got to here, means that neither of the flags provided and we need to init our own config.
func (dc *DotnetCommand) InitNewConfig(configDirPath string) (configFile *os.File, err error) {
	// Initializing a new NuGet config file that NuGet will use into a temp file
	configFile, err = ioutil.TempFile(configDirPath, "jfrog.cli.nuget.")
	if errorutils.CheckError(err) != nil {
		return
	}
	log.Debug("Nuget config file created at:", configFile.Name())
	defer func() {
		e := configFile.Close()
		if err == nil {
			err = errorutils.CheckError(e)
		}
	}()

	// We will prefer to write the NuGet configuration using the `nuget add source` command (addSourceToNugetConfig)
	// Currently the NuGet configuration utility doesn't allow setting protocolVersion.
	// Until that is supported, the templated method must be used.
	err = dc.addSourceToNugetTemplate(configFile)
	return
}

// Adds a source to the nuget config template
func (dc *DotnetCommand) addSourceToNugetTemplate(configFile *os.File) error {
	sourceUrl, user, password, err := dc.getSourceDetails()
	if err != nil {
		return err
	}

	// Specify the protocolVersion
	protoVer := "3"
	if dc.useNugetV2 {
		protoVer = "2"
	}

	// Format the templates
	_, err = fmt.Fprintf(configFile, dotnet.ConfigFileFormat, sourceUrl, protoVer, user, password)
	return err
}

func (dc *DotnetCommand) getSourceDetails() (sourceURL, user, password string, err error) {
	var u *url.URL
	u, err = url.Parse(dc.serverDetails.ArtifactoryUrl)
	if errorutils.CheckError(err) != nil {
		return
	}
	nugetApi := "api/nuget/v3"
	if dc.useNugetV2 {
		nugetApi = "api/nuget"
	}
	u.Path = path.Join(u.Path, nugetApi, dc.repoName)
	sourceURL = u.String()

	user = dc.serverDetails.User
	password = dc.serverDetails.Password
	// If access-token is defined, extract user from it.
	serverDetails, err := dc.ServerDetails()
	if errorutils.CheckError(err) != nil {
		return
	}
	if serverDetails.AccessToken != "" {
		log.Debug("Using access-token details for nuget authentication.")
		user, err = auth.ExtractUsernameFromAccessToken(serverDetails.AccessToken)
		if err != nil {
			return
		}
		password = serverDetails.AccessToken
	}
	return
}

// Returns the value of the flag if exists
func getFlagValueIfExists(cmdFlag string, cmd *dotnet.Cmd) (string, error) {
	for i := 0; i < len(cmd.CommandFlags); i++ {
		if !strings.EqualFold(cmd.CommandFlags[i], cmdFlag) {
			continue
		}
		if i+1 == len(cmd.CommandFlags) {
			return "", errors.New(fmt.Sprintf("%s flag was provided without value", cmdFlag))
		}
		return cmd.CommandFlags[i+1], nil
	}

	return "", nil
}
