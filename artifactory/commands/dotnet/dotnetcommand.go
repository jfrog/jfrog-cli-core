package dotnet

import (
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
	"net/url"
	"os"
	"path"
	"strings"
)

const (
	SourceName        = "JFrogCli"
	configFilePattern = "jfrog.cli.nuget."
)

type DotnetCommand struct {
	toolchainType      dotnet.ToolchainType
	subCommand         string
	argAndFlags        []string
	repoName           string
	solutionPath       string
	useNugetV2         bool
	buildConfiguration *utils.BuildConfiguration
	serverDetails      *config.ServerDetails
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

	buildInfoService := utils.CreateBuildInfoService()
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
			e := callbackFunc()
			if err == nil {
				err = e
			}
		}
	}()
	if err = buildInfoModule.CalcDependencies(); err != nil {
		return err
	}
	log.Info(fmt.Sprintf("%s finished successfully.", dc.toolchainType))
	return nil
}

// prepareDotnetBuildInfoModule prepare dotnet modules with the provided cli parameters.
// In case no config file was provided - creates a temporary one.
func (dc *DotnetCommand) prepareDotnetBuildInfoModule(buildInfoModule *build.DotnetModule) (func() error, error) {
	callbackFunc, err := dc.prepareConfigFile()
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
	log.Debug("'Add sources' command executed. Output:", output)
	return err
}

// Checks if the user provided input such as -configfile flag or -Source flag.
// If those flags were provided, NuGet will use the provided configs (default config file or the one with -configfile)
// If neither provided, we are initializing our own config.
func (dc *DotnetCommand) prepareConfigFile() (cleanup func() error, err error) {
	dc.solutionPath, err = changeWorkingDir(dc.solutionPath)
	if err != nil {
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

	configFile, err := dc.InitNewConfig(tempDirPath)
	if err == nil {
		dc.argAndFlags = append(dc.argAndFlags, dc.GetToolchain().GetTypeFlagPrefix()+"configfile", configFile.Name())
	}
	return
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

// The fact that we here, means that neither of the flags were provided, and we need to init our own config.
func (dc *DotnetCommand) InitNewConfig(configDirPath string) (configFile *os.File, err error) {
	// Initializing a new NuGet config file that NuGet will use into a temp file
	configFile, err = os.CreateTemp(configDirPath, configFilePattern)
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

	// We would prefer to write the NuGet configuration using the `nuget add source` command,
	// but the NuGet configuration utility doesn't currently allow setting protocolVersion.
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
