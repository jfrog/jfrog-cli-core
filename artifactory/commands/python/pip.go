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

type PipCommand struct {
	PythonCommand
}

func NewPipCommand() *PipCommand {
	return &PipCommand{PythonCommand: *NewPythonCommand(pythonutils.Pip)}
}

func (pc *PipCommand) Run() (err error) {
	return pc.PythonCommand.Run()
}

func (pc *PipCommand) UpdateDepsChecksumInfoFunc(dependenciesMap map[string]entities.Dependency, srcPath string) error {
	servicesManager, err := utils.CreateServiceManager(pc.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}
	return dependencies.UpdateDepsChecksumInfo(dependenciesMap, srcPath, servicesManager, pc.repository)
}

func (pc *PipCommand) SetRepo(repo string) *PipCommand {
	pc.PythonCommand.SetRepo(repo)
	return pc
}

func (pc *PipCommand) SetArgs(arguments []string) *PipCommand {
	pc.PythonCommand.SetArgs(arguments)
	return pc
}

func (pc *PipCommand) SetCommandName(commandName string) *PipCommand {
	pc.PythonCommand.SetCommandName(commandName)
	return pc
}

func (pc *PipCommand) CommandName() string {
	return "rt_python_pip"
}

func (pc *PipCommand) SetServerDetails(serverDetails *config.ServerDetails) *PipCommand {
	pc.PythonCommand.SetServerDetails(serverDetails)
	return pc
}

func (pc *PipCommand) ServerDetails() (*config.ServerDetails, error) {
	return pc.serverDetails, nil
}

func (pc *PipCommand) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, string(pc.pythonTool))
	cmd = append(cmd, pc.commandName)
	cmd = append(cmd, pc.args...)
	return exec.Command(cmd[0], cmd[1:]...)
}

func (pc *PipCommand) GetEnv() map[string]string {
	return map[string]string{}
}

func (pc *PipCommand) GetStdWriter() io.WriteCloser {
	return nil
}

func (pc *PipCommand) GetErrWriter() io.WriteCloser {
	return nil
}
