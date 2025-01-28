package dotnet

import (
	"errors"
	"fmt"
	"github.com/jfrog/build-info-go/build"
	"github.com/jfrog/build-info-go/build/utils/dotnet"
	frogio "github.com/jfrog/gofrog/io"
	commonBuild "github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"net/url"
	"os"
	"path"
	"strings"
)

const (
	// SourceName should match the one in the config file template.
	SourceName        = "JFrogCli"
	configFilePattern = "jfrog.cli.nuget."

	dotnetTestError = `the command failed with an error.
Note that JFrog CLI does not restore dependencies during a 'dotnet test' command, so if needed, run a preceding 'dotnet restore'.
The initial error is:
`
	noRestoreFlag = "--no-restore"
)

type DotnetCommand struct {
	toolchainType dotnet.ToolchainType
	subCommand    string
	argAndFlags   []string
	repoName      string
	solutionPath  string
	useNugetV2    bool
	// By default, package sources are required to use HTTPS. This option allows sources to use HTTP.
	allowInsecureConnections bool
	buildConfiguration       *commonBuild.BuildConfiguration
	serverDetails            *config.ServerDetails
}

func (dc *DotnetCommand) SetServerDetails(serverDetails *config.ServerDetails) *DotnetCommand {
	dc.serverDetails = serverDetails
	return dc
}

func (dc *DotnetCommand) SetBuildConfiguration(buildConfiguration *commonBuild.BuildConfiguration) *DotnetCommand {
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

func (dc *DotnetCommand) SetAllowInsecureConnections(allowInsecureConnections bool) *DotnetCommand {
	dc.allowInsecureConnections = allowInsecureConnections
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
func (dc *DotnetCommand) Exec() (err error) {
	log.Info("Running " + dc.toolchainType.String() + "...")
	buildName, err := dc.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := dc.buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}

	buildInfoService := commonBuild.CreateBuildInfoService()
	dotnetBuild, err := buildInfoService.GetOrCreateBuildWithProject(buildName, buildNumber, dc.buildConfiguration.GetProject())
	if err != nil {
		return errorutils.CheckError(err)
	}
	buildInfoModule, err := dotnetBuild.AddDotnetModules(dc.solutionPath)
	if err != nil {
		return errorutils.CheckError(err)
	}
	callbackFunc, err := dc.prepareDotnetBuildInfoModule(buildInfoModule)
	if err != nil {
		return err
	}
	defer func() {
		if callbackFunc != nil {
			err = errors.Join(err, callbackFunc())
		}
	}()
	if err = buildInfoModule.CalcDependencies(); err != nil {
		if dc.isDotnetTestCommand() {
			return errors.New(dotnetTestError + err.Error())
		}
		return err
	}
	log.Info(fmt.Sprintf("%s finished successfully.", dc.toolchainType))
	return nil
}

// prepareDotnetBuildInfoModule prepare dotnet modules with the provided cli parameters.
// In case no config file was provided - creates a temporary one.
func (dc *DotnetCommand) prepareDotnetBuildInfoModule(buildInfoModule *build.DotnetModule) (func() error, error) {
	callbackFunc, err := dc.prepareConfigFileIfNeeded()
	if err != nil {
		return nil, err
	}
	buildInfoModule.SetName(dc.buildConfiguration.GetModule())
	buildInfoModule.SetSubcommand(dc.subCommand)
	buildInfoModule.SetArgAndFlags(dc.argAndFlags)
	buildInfoModule.SetToolchainType(dc.toolchainType)
	return callbackFunc, nil
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

	return newWorkingDir, errorutils.CheckError(err)
}

// Runs nuget/dotnet source add command
func AddSourceToNugetConfig(cmdType dotnet.ToolchainType, sourceUrl, user, password string) error {
	cmd, err := dotnet.CreateDotnetAddSourceCmd(cmdType, sourceUrl)
	if err != nil {
		return err
	}

	flagPrefix := cmdType.GetTypeFlagPrefix()
	cmd.CommandFlags = append(cmd.CommandFlags, flagPrefix+"name", SourceName)
	cmd.CommandFlags = append(cmd.CommandFlags, flagPrefix+"username", user)
	cmd.CommandFlags = append(cmd.CommandFlags, flagPrefix+"password", password)

	// Support http connection if requested
	if strings.HasPrefix(sourceUrl, "http://") {
		if cmdType == dotnet.DotnetCore {
			cmd.CommandFlags = append(cmd.CommandFlags, flagPrefix+"allow-insecure-connections")
		} else {
			cmd.CommandFlags = append(cmd.CommandFlags, flagPrefix+"AllowInsecureConnections")
		}
	}

	_, errOut, _, err := frogio.RunCmdWithOutputParser(cmd, false)
	if err != nil {
		return fmt.Errorf("%s\nfailed to add source: %w", errOut, err)
	}
	return nil
}

// RemoveSourceFromNugetConfigIfExists runs the nuget/dotnet source remove command.
// Removes the source if it exists in the configuration.
func RemoveSourceFromNugetConfigIfExists(cmdType dotnet.ToolchainType) error {
	cmd, err := dotnet.NewToolchainCmd(cmdType)
	if err != nil {
		return err
	}
	if cmdType == dotnet.DotnetCore {
		cmd.Command = append(cmd.Command, "nuget", "remove", "source", SourceName)
	} else {
		cmd.Command = append(cmd.Command, "sources", "remove")
		cmd.CommandFlags = append(cmd.CommandFlags, "-name", SourceName)
	}

	stdOut, stdErr, _, err := frogio.RunCmdWithOutputParser(cmd, false)
	if err != nil {
		if strings.Contains(stdOut+stdErr, "Unable to find") || strings.Contains(stdOut+stdErr, "does not exist") {
			return nil
		}
		return errorutils.CheckErrorf("%s\nfailed to remove source: %s", stdErr, err.Error())
	}
	return nil
}

// Checks if the user provided input such as -configfile flag or -Source flag.
// If those flags were provided, NuGet will use the provided configs (default config file or the one with -configfile)
// If neither provided, we are initializing our own config.
func (dc *DotnetCommand) prepareConfigFileIfNeeded() (cleanup func() error, err error) {
	dc.solutionPath, err = changeWorkingDir(dc.solutionPath)
	if err != nil {
		return
	}

	if dc.isDotnetTestCommand() {
		// The dotnet test command does not support the configfile flag.
		// To avoid resolving from a registry that is not Artifactory, we add the no-restore flag and require the user to run a restore before the test command.
		dc.argAndFlags = append(dc.argAndFlags, noRestoreFlag)
		return
	}

	cmdFlag := dc.GetToolchain().GetTypeFlagPrefix() + "configfile"
	currentConfigPath, err := getFlagValueIfExists(cmdFlag, dc.argAndFlags)
	if err != nil {
		return
	}
	if currentConfigPath != "" {
		return
	}

	cmdFlag = dc.GetToolchain().GetTypeFlagPrefix() + "source"
	sourceCommandValue, err := getFlagValueIfExists(cmdFlag, dc.argAndFlags)
	if err != nil {
		return
	}
	if sourceCommandValue != "" {
		return
	}

	// Use temp dir to save config file, so that config will be removed at the end.
	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		return
	}
	cleanup = func() error {
		return fileutils.RemoveTempDir(tempDirPath)
	}

	configFile, err := InitNewConfig(tempDirPath, dc.repoName, dc.serverDetails, dc.useNugetV2, dc.allowInsecureConnections)
	if err == nil {
		dc.argAndFlags = append(dc.argAndFlags, dc.GetToolchain().GetTypeFlagPrefix()+"configfile", configFile.Name())
	}
	return
}

func (dc *DotnetCommand) isDotnetTestCommand() bool {
	return dc.GetToolchain() == dotnet.DotnetCore && dc.subCommand == "test"
}

// Returns the value of the flag if exists
func getFlagValueIfExists(cmdFlag string, argAndFlags []string) (string, error) {
	for i := 0; i < len(argAndFlags); i++ {
		if !strings.EqualFold(argAndFlags[i], cmdFlag) {
			continue
		}
		if i+1 == len(argAndFlags) {
			return "", errorutils.CheckErrorf(cmdFlag, " flag was provided without value")
		}
		return argAndFlags[i+1], nil
	}

	return "", nil
}

// InitNewConfig is used when neither of the flags were provided, and we need to init our own config.
func InitNewConfig(configDirPath, repoName string, server *config.ServerDetails, useNugetV2, allowInsecureConnections bool) (configFile *os.File, err error) {
	// Initializing a new NuGet config file that NuGet will use into a temp file
	configFile, err = os.CreateTemp(configDirPath, configFilePattern)
	if errorutils.CheckError(err) != nil {
		return
	}
	log.Debug("Nuget config file created at:", configFile.Name())
	defer func() {
		err = errors.Join(err, errorutils.CheckError(configFile.Close()))
	}()

	// We would prefer to write the NuGet configuration using the `nuget add source` command,
	// but the NuGet configuration utility doesn't currently allow setting protocolVersion.
	// Until that is supported, the templated method must be used.
	err = addSourceToNugetTemplate(configFile, server, useNugetV2, repoName, allowInsecureConnections)
	return
}

// Adds a source to the nuget config template
func addSourceToNugetTemplate(configFile *os.File, server *config.ServerDetails, useNugetV2 bool, repoName string, allowInsecureConnections bool) error {
	sourceUrl, user, password, err := GetSourceDetails(server, repoName, useNugetV2)
	if err != nil {
		return err
	}

	// Specify the protocolVersion
	protoVer := "3"
	if useNugetV2 {
		protoVer = "2"
	}

	// Format the templates
	_, err = fmt.Fprintf(configFile, dotnet.ConfigFileFormat, sourceUrl, protoVer, allowInsecureConnections, user, password)
	return err
}

func GetSourceDetails(details *config.ServerDetails, repoName string, useNugetV2 bool) (sourceURL, user, password string, err error) {
	var u *url.URL
	u, err = url.Parse(details.ArtifactoryUrl)
	if errorutils.CheckError(err) != nil {
		return
	}
	nugetApi := "api/nuget/v3"
	if useNugetV2 {
		nugetApi = "api/nuget"
	}
	u.Path = path.Join(u.Path, nugetApi, repoName)
	// Append "index.json" for NuGet V3
	if !useNugetV2 {
		u.Path = path.Join(u.Path, "index.json")
	}
	sourceURL = u.String()

	user = details.User
	password = details.Password
	// If access-token is defined, extract user from it.
	if details.AccessToken != "" {
		log.Debug("Using access-token details for nuget authentication.")
		if user == "" {
			user = auth.ExtractUsernameFromAccessToken(details.AccessToken)
		}
		password = details.AccessToken
	}
	return
}
