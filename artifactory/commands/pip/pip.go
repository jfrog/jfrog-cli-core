package pip

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type PipCommand struct {
	rtDetails  *config.ServerDetails
	args       []string
	repository string
}

func (pc *PipCommand) SetServerDetails(serverDetails *config.ServerDetails) *PipCommand {
	pc.rtDetails = serverDetails
	return pc
}

func (pc *PipCommand) SetRepo(repo string) *PipCommand {
	pc.repository = repo
	return pc
}

func (pc *PipCommand) SetArgs(arguments []string) *PipCommand {
	pc.args = arguments
	return pc
}

type PipCommandInterface interface {
	SetServerDetails(rtDetails *config.ServerDetails) *PipCommand
	SetRepo(repo string) *PipCommand
	SetArgs(arguments []string) *PipCommand
	ServerDetails() (*config.ServerDetails, error)
	CommandName() string
	Run() error
}
