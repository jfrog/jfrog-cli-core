package npm

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
)

// GenericCommand represents any npm command which is not "install", "ci" or "publish".
type GenericCommand struct {
	*CommonArgs
}

func NewNpmGenericCommand(cmdName string) *GenericCommand {
	return &GenericCommand{
		CommonArgs: &CommonArgs{cmdName: cmdName},
	}
}

func (gc *GenericCommand) CommandName() string {
	return "rt_npm_generic"
}

func (gc *GenericCommand) ServerDetails() (*config.ServerDetails, error) {
	return gc.serverDetails, nil
}

func (gc *GenericCommand) Run() (err error) {
	if err = gc.PreparePrerequisites("", false); err != nil {
		return
	}
	log.Debug(fmt.Sprintf("Running npm %s command.", gc.cmdName))
	npmCmdConfig := &npmutils.NpmConfig{
		Npm:          gc.executablePath,
		Command:      gc.npmArgs,
		CommandFlags: nil,
		StrWriter:    nil,
		ErrWriter:    nil,
	}
	command := npmCmdConfig.GetCmd()
	command.Stderr = os.Stderr
	command.Stdout = os.Stderr
	return coreutils.ConvertExitCodeError(errorutils.CheckError(command.Run()))
}
