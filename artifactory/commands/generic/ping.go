package generic

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
)

type PingCommand struct {
	serverDetails *config.ServerDetails
	response      []byte
}

func NewPingCommand() *PingCommand {
	return &PingCommand{}
}

func (pc *PingCommand) Response() []byte {
	return pc.response
}

func (pc *PingCommand) ServerDetails() (*config.ServerDetails, error) {
	return pc.serverDetails, nil
}

func (pc *PingCommand) SetServerDetails(serverDetails *config.ServerDetails) *PingCommand {
	pc.serverDetails = serverDetails
	return pc
}

func (pc *PingCommand) CommandName() string {
	return "rt_ping"
}

func (pc *PingCommand) Run() error {
	var err error
	pc.response, err = pc.Ping()
	if err != nil {
		return err
	}
	return nil
}

func (pc *PingCommand) Ping() ([]byte, error) {
	servicesManager, err := utils.CreateServiceManager(pc.serverDetails, -1, 0, false)
	if err != nil {
		return nil, err
	}
	return servicesManager.Ping()
}
