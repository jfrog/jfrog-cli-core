package python

import (
	"errors"
	"fmt"
	"github.com/jfrog/build-info-go/build"
	"github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/python/dependencies"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	buildUtils "github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type PoetryCommand struct {
	PythonCommand
}

func NewPoetryCommand() *PoetryCommand {
	return &PoetryCommand{
		PythonCommand: *NewPythonCommand(pythonutils.Poetry),
	}
}

func (pc *PoetryCommand) Run() (err error) {
	log.Info(fmt.Sprintf("Running Poetry %s.", pc.commandName))
	var buildConfiguration *buildUtils.BuildConfiguration
	pc.args, buildConfiguration, err = buildUtils.ExtractBuildDetailsFromArgs(pc.args)
	if err != nil {
		return err
	}
	pythonBuildInfo, err := buildUtils.PrepareBuildPrerequisites(buildConfiguration)
	if err != nil {
		return
	}
	defer func() {
		if pythonBuildInfo != nil && err != nil {
			err = errors.Join(err, pythonBuildInfo.Clean())
		}
	}()
	err = pc.SetPypiRepoUrlWithCredentials()
	if err != nil {
		return err
	}
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

func (pc *PoetryCommand) install(buildConfiguration *buildUtils.BuildConfiguration, pythonBuildInfo *build.Build) (err error) {
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

func (pc *PoetryCommand) publish(buildConfiguration *buildUtils.BuildConfiguration, pythonBuildInfo *build.Build) error {
	publishCmdArgs := append(slices.Clone(pc.args), "-r "+pc.repository)
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
	rtUrl, username, password, err := GetPypiRepoUrlWithCredentials(pc.serverDetails, pc.repository, false)
	if err != nil {
		return err
	}
	if password != "" {
		return ConfigPoetryRepo(
			rtUrl.Scheme+"://"+rtUrl.Host+rtUrl.Path,
			username,
			password,
			pc.repository)
	}
	return nil
}

func ConfigPoetryRepo(url, username, password, configRepoName string) error {
	err := RunPoetryConfig(url, username, password, configRepoName)
	if err != nil {
		return err
	}

	// Add the repository config to the pyproject.toml
	currentDir, err := os.Getwd()
	if err != nil {
		return errorutils.CheckError(err)
	}
	if err = addRepoToPyprojectFile(filepath.Join(currentDir, pyproject), configRepoName, url); err != nil {
		return err
	}
	return poetryUpdate()
}

func RunPoetryConfig(url, username, password, configRepoName string) error {
	// Add the poetry repository config
	// poetry config repositories.<repo-name> https://<your-artifactory-url>/artifactory/api/pypi/<repo-name>/simple
	err := RunConfigCommand(project.Poetry, []string{poetryConfigRepoPrefix + configRepoName, url})
	if err != nil {
		return err
	}

	// Set the poetry repository credentials
	// poetry config http-basic.<repo-name> <user> <password/token>
	return RunConfigCommand(project.Poetry, []string{poetryConfigAuthPrefix + configRepoName, username, password})
}

func poetryUpdate() (err error) {
	log.Info("Running Poetry update")
	cmd := gofrogcmd.NewCommand("poetry", "update", []string{})
	err = gofrogcmd.RunCmd(cmd)
	if err != nil {
		return errorutils.CheckErrorf("Poetry config command failed with: %s", err.Error())
	}
	return
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
	return err
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
