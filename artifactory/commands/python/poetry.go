package python

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jfrog/build-info-go/build"
	"github.com/jfrog/build-info-go/entities"
	buildinfoutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/python/dependencies"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/spf13/viper"
)

type PoetryCommand struct {
	PythonCommand
	// The uniq Artifactory repository name for poetry config file.
	poetryConfigRepoName string
}

const (
	baseConfigRepoName     = "jfrog-server"
	poetryConfigAuthPrefix = "http-basic."
	poetryConfigRepoPrefix = "repositories."
	pyproject              = "pyproject.toml"
	pyprojectBackup        = "pyproject.toml.backup"
)

func NewPoetryCommand() *PoetryCommand {
	return &PoetryCommand{
		PythonCommand:        PythonCommand{pythonTool: pythonutils.Poetry},
		poetryConfigRepoName: fmt.Sprintf("%s-%s", baseConfigRepoName, strconv.FormatInt(time.Now().UnixMilli(), 10)),
	}
}

func (pc *PoetryCommand) Run() (err error) {
	log.Info(fmt.Sprintf("Running Poetry %s.", pc.commandName))
	var buildConfiguration *utils.BuildConfiguration
	pc.args, buildConfiguration, err = utils.ExtractBuildDetailsFromArgs(pc.args)
	if err != nil {
		return err
	}
	pythonBuildInfo, err := utils.PrepareBuildPrerequisites(buildConfiguration)
	if err != nil {
		return
	}
	defer func() {
		if pythonBuildInfo != nil && err != nil {
			e := pythonBuildInfo.Clean()
			if e != nil {
				err = errors.New(err.Error() + "\n" + e.Error())
			}
		}
	}()
	err = pc.SetPypiRepoUrlWithCredentials()
	if err != nil {
		return err
	}
	defer func() {
		e := pc.cleanup()
		if err == nil {
			err = e
		}
	}()
	if pythonBuildInfo != nil {
		switch pc.commandName {
		case "install":
			return pc.install(buildConfiguration, pythonBuildInfo)
		case "publish":
			return pc.publish(buildConfiguration, pythonBuildInfo)
		default:
			// poetry native command
			return gofrogcmd.RunCmd(pc)

		}
	}
	return gofrogcmd.RunCmd(pc)
}

func (pc *PoetryCommand) install(buildConfiguration *utils.BuildConfiguration, pythonBuildInfo *build.Build) (err error) {
	var pythonModule *build.PythonModule
	pythonModule, err = pythonBuildInfo.AddPythonModule("", pc.pythonTool)
	if err != nil {
		return
	}
	if buildConfiguration.GetModule() != "" {
		pythonModule.SetName(buildConfiguration.GetModule())
	}
	var localDependenciesPath string
	localDependenciesPath, err = config.GetJfrogDependenciesPath()
	if err != nil {
		return
	}
	pythonModule.SetLocalDependenciesPath(localDependenciesPath)
	pythonModule.SetUpdateDepsChecksumInfoFunc(pc.UpdateDepsChecksumInfoFunc)

	return errorutils.CheckError(pythonModule.RunInstallAndCollectDependencies(pc.args))
}

func (pc *PoetryCommand) publish(buildConfiguration *utils.BuildConfiguration, pythonBuildInfo *build.Build) error {
	publishCmdArgs := append(pc.args, "-r "+pc.poetryConfigRepoName)
	// Collect build info by running the jf poetry install cmd
	pc.args = []string{}
	err := pc.install(buildConfiguration, pythonBuildInfo)
	if err != nil {
		return err
	}
	// Run the publish cmd
	pc.args = publishCmdArgs
	return gofrogcmd.RunCmd(pc)
}

func (pc *PoetryCommand) UpdateDepsChecksumInfoFunc(dependenciesMap map[string]entities.Dependency, srcPath string) error {
	servicesManager, err := utils.CreateServiceManager(pc.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}
	return dependencies.UpdateDepsChecksumInfo(dependenciesMap, srcPath, servicesManager, pc.repository)
}

func (pc *PoetryCommand) SetRepo(repo string) *PoetryCommand {
	pc.repository = repo
	return pc
}

func (pc *PoetryCommand) SetArgs(arguments []string) *PoetryCommand {
	pc.args = arguments
	return pc
}

func (pc *PoetryCommand) SetCommandName(commandName string) *PoetryCommand {
	pc.commandName = commandName
	return pc
}

func (pc *PoetryCommand) SetPypiRepoUrlWithCredentials() error {
	rtUrl, err := url.Parse(pc.serverDetails.GetArtifactoryUrl())
	if err != nil {
		return errorutils.CheckError(err)
	}

	username := pc.serverDetails.GetUser()
	password := pc.serverDetails.GetPassword()

	// Get credentials from access-token if exists.
	if pc.serverDetails.GetAccessToken() != "" {
		if username == "" {
			username = auth.ExtractUsernameFromAccessToken(pc.serverDetails.GetAccessToken())
		}
		password = pc.serverDetails.GetAccessToken()
	}
	rtUrl.Path += "api/pypi/" + pc.repository + "/simple"
	err = pc.configPoetryRepo(rtUrl.Scheme+"://"+rtUrl.Host+rtUrl.Path, username, password)
	if err != nil {
		return err
	}
	return nil
}

func (pc *PoetryCommand) configPoetryRepo(url, username, password string) error {
	// Add the poetry repository config
	err := runPoetryConfigCommand([]string{poetryConfigRepoPrefix + pc.poetryConfigRepoName, url}, false)
	if err != nil {
		return err
	}

	// Set the poetry repository credentials
	err = runPoetryConfigCommand([]string{poetryConfigAuthPrefix + pc.poetryConfigRepoName, username, password}, true)
	if err != nil {
		return err
	}

	// Add the repository config to the pyproject.toml
	currentDir, err := os.Getwd()
	if err != nil {
		return errorutils.CheckError(err)
	}
	err = fileutils.CopyFile(filepath.Join(currentDir, pyprojectBackup), filepath.Join(currentDir, pyproject))
	if err != nil {
		return err
	}
	return addRepoToPyprojectFile(filepath.Join(currentDir, pyproject), pc.poetryConfigRepoName, url)
}

func addRepoToPyprojectFile(filepath, poetryRepoName, repoUrl string) error {
	viper.SetConfigType("toml")
	viper.SetConfigFile(filepath)
	err := viper.ReadInConfig()
	if err != nil {
		return errorutils.CheckErrorf("Failed to read pyproject.toml: %s", err.Error())
	}
	viper.Set("tool.poetry.source", []map[string]string{{"name": poetryRepoName, "url": repoUrl}})
	err = viper.WriteConfig()
	if err != nil {
		return errorutils.CheckErrorf("Failed to add tool.poetry.source to pyproject.toml: %s", err.Error())
	}
	log.Info(fmt.Sprintf("Added tool.poetry.source name:%q url:%q", poetryRepoName, repoUrl))
	log.Info("Running Poetry update")
	cmd := buildinfoutils.NewCommand("poetry", "update", []string{})
	err = gofrogcmd.RunCmd(cmd)
	if err != nil {
		return errorutils.CheckErrorf("Poetry config command failed with: %s", err.Error())
	}
	return err
}

func (pc *PoetryCommand) cleanup() error {
	// Unset the poetry repository config
	err := runPoetryConfigCommand([]string{poetryConfigRepoPrefix + pc.poetryConfigRepoName, "--unset"}, false)
	if err != nil {
		return err
	}
	// Unset the poetry repository credentials
	err = runPoetryConfigCommand([]string{poetryConfigAuthPrefix + pc.poetryConfigRepoName, "--unset"}, false)
	if err != nil {
		return err
	}

	// Restore original pyproject.toml
	currentDir, err := os.Getwd()
	if err != nil {
		return errorutils.CheckError(err)
	}
	err = os.Remove(filepath.Join(currentDir, pyproject))
	if err != nil {
		return errorutils.CheckErrorf("Cleanup modified pyproject.toml failed with: %s", err.Error())
	}
	return fileutils.MoveFile(filepath.Join(currentDir, pyprojectBackup, pyproject), filepath.Join(currentDir, pyproject))
}

func (pc *PoetryCommand) CommandName() string {
	return "rt_python_poetry"
}

func (pc *PoetryCommand) SetServerDetails(serverDetails *config.ServerDetails) *PoetryCommand {
	pc.serverDetails = serverDetails
	return pc
}

func (pc *PoetryCommand) ServerDetails() (*config.ServerDetails, error) {
	return pc.serverDetails, nil
}

func (pc *PoetryCommand) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, string(pc.pythonTool))
	cmd = append(cmd, pc.commandName)
	cmd = append(cmd, pc.args...)
	return exec.Command(cmd[0], cmd[1:]...)
}

func (pc *PoetryCommand) GetEnv() map[string]string {
	return map[string]string{}
}

func (pc *PoetryCommand) GetStdWriter() io.WriteCloser {
	return nil
}

func (pc *PoetryCommand) GetErrWriter() io.WriteCloser {
	return nil
}

func runPoetryConfigCommand(args []string, maskArgs bool) error {
	logMessage := "config "
	if maskArgs {
		logMessage += "***"
	} else {
		logMessage += strings.Join(args, " ")
	}
	log.Info(fmt.Sprintf("Running Poetry %s", logMessage))
	cmd := buildinfoutils.NewCommand("poetry", "config", args)
	err := gofrogcmd.RunCmd(cmd)
	if err != nil {
		return errorutils.CheckErrorf("Poetry config command failed with: %s", err.Error())
	}
	return nil
}
