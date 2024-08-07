package python

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/jfrog/build-info-go/build"
	"github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/python/dependencies"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	buildUtils "github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	python "github.com/jfrog/jfrog-cli-core/v2/utils/python"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io"
	"os"
	"os/exec"
	"strings"
)

type PythonCommand struct {
	serverDetails *config.ServerDetails
	pythonTool    pythonutils.PythonTool
	commandName   string
	args          []string
	repository    string
}

func NewPythonCommand(pythonTool pythonutils.PythonTool) *PythonCommand {
	return &PythonCommand{pythonTool: pythonTool}
}

func (pc *PythonCommand) Run() (err error) {
	log.Info("Running", string(pc.pythonTool), pc.commandName)
	var buildConfiguration *buildUtils.BuildConfiguration
	pc.args, buildConfiguration, err = buildUtils.ExtractBuildDetailsFromArgs(pc.args)
	if err != nil {
		return
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
		return
	}

	if pythonBuildInfo != nil && pc.commandName == "install" {
		// Need to collect build info
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
		err = errorutils.CheckError(pythonModule.RunInstallAndCollectDependencies(pc.args))
	} else {
		// Python native command
		for k, v := range pc.GetEnv() {
			if err := os.Setenv(k, v); err != nil {
				return err
			}
		}

		cmd := pc.GetCmd()
		errBuffer := bytes.NewBuffer([]byte{})
		cmd.Stderr = errBuffer

		err = cmd.Run()
		if err != nil {
			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				err = errors.New(err.Error())
			}
			return fmt.Errorf("error while running '%s': %s\n%s", strings.Join(cmd.Args, " "), err.Error(), strings.TrimSpace(errBuffer.String()))
		}
	}
	return
}

func (pc *PythonCommand) UpdateDepsChecksumInfoFunc(dependenciesMap map[string]entities.Dependency, srcPath string) error {
	servicesManager, err := utils.CreateServiceManager(pc.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}
	return dependencies.UpdateDepsChecksumInfo(dependenciesMap, srcPath, servicesManager, pc.repository)
}

func (pc *PythonCommand) SetRepo(repo string) *PythonCommand {
	pc.repository = repo
	return pc
}

func (pc *PythonCommand) SetArgs(arguments []string) *PythonCommand {
	pc.args = arguments
	return pc
}

func (pc *PythonCommand) SetCommandName(commandName string) *PythonCommand {
	pc.commandName = commandName
	return pc
}

func (pc *PythonCommand) SetPypiRepoUrlWithCredentials() error {
	rtUrl, err := python.GetPypiRepoUrl(pc.serverDetails, pc.repository, false)
	if err != nil {
		return err
	}
	pc.args = append(pc.args, python.GetPypiRemoteRegistryFlag(pc.pythonTool), rtUrl)
	return nil
}

func (pc *PythonCommand) SetServerDetails(serverDetails *config.ServerDetails) *PythonCommand {
	pc.serverDetails = serverDetails
	return pc
}

func (pc *PythonCommand) ServerDetails() (*config.ServerDetails, error) {
	return pc.serverDetails, nil
}

func (pc *PythonCommand) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, string(pc.pythonTool))
	cmd = append(cmd, pc.commandName)
	cmd = append(cmd, pc.args...)
	return exec.Command(cmd[0], cmd[1:]...)
}

func (pc *PythonCommand) GetEnv() map[string]string {
	return map[string]string{}
}

func (pc *PythonCommand) GetStdWriter() io.WriteCloser {
	return nil
}

func (pc *PythonCommand) GetErrWriter() io.WriteCloser {
	return nil
}
