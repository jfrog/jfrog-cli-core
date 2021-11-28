package python

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type PipenvNativeCommand struct {
	PythonCommand
}

// NewPipNativeCommand represents any pip command which is not "install".
func NewPipenvNativeCommand() *PipenvNativeCommand {
	return &PipenvNativeCommand{PythonCommand: PythonCommand{}}
}

func (penc *PipenvNativeCommand) Run() (err error) {
	log.Info("Running pipenv %s.", penc.commandName)

	pipenvExecutablePath, err := getExecutablePath("pipenv")
	if err != nil {
		return nil
	}
	penc.executable = pipenvExecutablePath

	err = penc.setPypiRepoUrlWithCredentials(penc.serverDetails, penc.repository, utils.Pipenv)
	if err != nil {
		return nil
	}

	return coreutils.ConvertExitCodeError(errorutils.CheckError(penc.GetCmd().Run()))
}

func (penc *PipenvNativeCommand) CommandName() string {
	return "rt_pipenv_native"
}

func (penc *PipenvNativeCommand) ServerDetails() (*config.ServerDetails, error) {
	return penc.serverDetails, nil
}
