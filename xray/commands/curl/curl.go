package curl

import (
	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
)

type XrCurlCommand struct {
	commands.CurlCommand
}

func NewXrCurlCommand(curlCommand commands.CurlCommand) *XrCurlCommand {
	return &XrCurlCommand{curlCommand}
}

func (curlCmd *XrCurlCommand) CommandName() string {
	return "xr_curl"
}
