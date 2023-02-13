package npm

import (
	"fmt"
	"os"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type GenericCommandArgs struct {
	CommonArgs
}

// GenericCommand represents any npm command which is not "install", "ci" or "publish".
type GenericCommand struct {
	*GenericCommandArgs
}

func NewNpmGenericCommand(cmdName string) *GenericCommand {
	return &GenericCommand{
		GenericCommandArgs: &GenericCommandArgs{CommonArgs: CommonArgs{cmdName: cmdName}},
	}
}

func (gc *GenericCommand) CommandName() string {
	return "rt_npm_generic"
}

func (gca *GenericCommandArgs) ServerDetails() (*config.ServerDetails, error) {
	return gca.serverDetails, nil
}

func (gc *GenericCommand) Run() (err error) {
	if err = gc.PreparePrerequisites("", false); err != nil {
		return
	}
	return gc.runNpmGenericCommand()
}

func (gca *GenericCommandArgs) runNpmGenericCommand() error {
	log.Debug(fmt.Sprintf("Running npm %s command.", gca.cmdName))
	npmCmdConfig := &npmutils.NpmConfig{
		Npm:          gca.executablePath,
		Command:      gca.npmArgs,
		CommandFlags: nil,
		StrWriter:    nil,
		ErrWriter:    nil,
	}
	command := npmCmdConfig.GetCmd()
	command.Stderr = os.Stderr
	command.Stdout = os.Stderr
	return coreutils.ConvertExitCodeError(errorutils.CheckError(command.Run()))
}
