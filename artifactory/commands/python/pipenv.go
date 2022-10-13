package python

import (
	"io"
	"os/exec"

	"github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/python/dependencies"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type PipenvCommand struct {
	PythonCommand
}

func NewPipenvCommand() *PipenvCommand {
	return &PipenvCommand{PythonCommand: PythonCommand{pythonTool: pythonutils.Pipenv}}
}

func (pc *PipenvCommand) Run() (err error) {
	return pc.PythonCommand.Run()
}

func (pc *PipenvCommand) UpdateDepsChecksumInfoFunc(dependenciesMap map[string]entities.Dependency, srcPath string) error {
	servicesManager, err := utils.CreateServiceManager(pc.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}
	return dependencies.UpdateDepsChecksumInfo(dependenciesMap, srcPath, servicesManager, pc.repository)
}

func (pc *PipenvCommand) SetRepo(repo string) *PipenvCommand {
	pc.PythonCommand.SetRepo(repo)
	return pc
}

func (pc *PipenvCommand) SetArgs(arguments []string) *PipenvCommand {
	pc.PythonCommand.SetArgs(arguments)
	return pc
}

func (pc *PipenvCommand) SetCommandName(commandName string) *PipenvCommand {
	pc.PythonCommand.SetCommandName(commandName)
	return pc
}

func (pc *PipenvCommand) CommandName() string {
	return "rt_python_pipenv"
}

func (pc *PipenvCommand) SetServerDetails(serverDetails *config.ServerDetails) *PipenvCommand {
	pc.PythonCommand.SetServerDetails(serverDetails)
	return pc
}

func (pc *PipenvCommand) ServerDetails() (*config.ServerDetails, error) {
	return pc.serverDetails, nil
}

func (pc *PipenvCommand) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, string(pc.pythonTool))
	cmd = append(cmd, pc.commandName)
	cmd = append(cmd, pc.args...)
	return exec.Command(cmd[0], cmd[1:]...)
}

func (pc *PipenvCommand) GetEnv() map[string]string {
	return map[string]string{}
}

func (pc *PipenvCommand) GetStdWriter() io.WriteCloser {
	return nil
}

func (pc *PipenvCommand) GetErrWriter() io.WriteCloser {
	return nil
}
