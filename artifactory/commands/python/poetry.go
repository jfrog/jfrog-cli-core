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
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	python "github.com/jfrog/jfrog-cli-core/v2/utils/python"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/slices"
	"io"
	"os/exec"
)

type PoetryCommand struct {
	PythonCommand
	// The uniq Artifactory repository name for poetry config file.
	poetryConfigRepoName string
}

const baseConfigRepoName = "jfrog-server"

func NewPoetryCommand() *PoetryCommand {
	return &PoetryCommand{
		PythonCommand:        *NewPythonCommand(pythonutils.Poetry),
		poetryConfigRepoName: baseConfigRepoName,
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
	publishCmdArgs := append(slices.Clone(pc.args), "-r "+pc.poetryConfigRepoName)
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
	rtUrl, username, password, err := python.GetPypiRepoUrlWithCredentials(pc.serverDetails, pc.repository)
	if err != nil {
		return err
	}
	if password != "" {
		return python.ConfigPoetryRepo(
			rtUrl.Scheme+"://"+rtUrl.Host+rtUrl.Path,
			username,
			password,
			pc.poetryConfigRepoName)
	}
	return nil
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
