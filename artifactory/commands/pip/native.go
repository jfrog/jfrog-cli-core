package pip

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/pip"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type PipNativeCommand struct {
	cmdName string
	*PipCommand
}

// NewPipNativeCommand represents any pip command which is not "install".
func NewPipNativeCommand(cmdName string) *PipNativeCommand {
	return &PipNativeCommand{cmdName: cmdName, PipCommand: &PipCommand{}}
}

func (pnc *PipNativeCommand) Run() error {
	log.Info("Running pip %s.", pnc.cmdName)

	err := pnc.prepare()
	if err != nil {
		return err
	}

	pipNative := &pip.NativeExecutor{CmdName: pnc.cmdName, CommonExecutor: pip.CommonExecutor{Args: pnc.args, ServerDetails: pnc.rtDetails, Repository: pnc.repository}}
	return pipNative.Run()
}

func (pnc *PipNativeCommand) prepare() (err error) {
	// Filter out build flags.
	pnc.args, _, err = utils.ExtractBuildDetailsFromArgs(pnc.args)
	return
}

func (pnc *PipNativeCommand) CommandName() string {
	return "rt_pip_native"
}

func (pnc *PipNativeCommand) ServerDetails() (*config.ServerDetails, error) {
	return pnc.rtDetails, nil
}
