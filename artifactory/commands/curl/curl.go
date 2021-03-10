package curl

import (
	"github.com/jfrog/jfrog-cli-core/common/commands"
)

type RtCurlCommand struct {
	commands.CurlCommand
}

func NewRtCurlCommand(curlCommand commands.CurlCommand) *RtCurlCommand {
	return &RtCurlCommand{curlCommand}
}

func (curlCmd *RtCurlCommand) CommandName() string {
	return "rt_curl"
}
