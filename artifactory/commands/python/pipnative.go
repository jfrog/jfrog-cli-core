package python

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type PipNativeCommand struct {
	PythonCommand
}

// NewPipNativeCommand represents any pip command which is not "install".
func NewPipNativeCommand() *PipNativeCommand {
	return &PipNativeCommand{PythonCommand: PythonCommand{}}
}

func (pnc *PipNativeCommand) Run() (err error) {
	log.Info("Running pip %s.", pnc.commandName)

	pipExecutablePath, err := getExecutablePath("pip")
	if err != nil {
		return nil
	}
	pnc.executable = pipExecutablePath

	err = pnc.setPypiRepoUrlWithCredentials(pnc.serverDetails, pnc.repository, utils.Pip)
	if err != nil {
		return nil
	}

	return coreutils.ConvertExitCodeError(errorutils.CheckError(pnc.GetCmd().Run()))
}

func (pnc *PipNativeCommand) CommandName() string {
	return "rt_pip_native"
}

func (pnc *PipNativeCommand) ServerDetails() (*config.ServerDetails, error) {
	return pnc.serverDetails, nil
}
