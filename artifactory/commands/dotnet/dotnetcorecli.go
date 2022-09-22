package dotnet

import (
	"github.com/jfrog/build-info-go/build/utils/dotnet"
)

type DotnetCoreCliCommand struct {
	*DotnetCommand
}

func NewDotnetCoreCliCommand() *DotnetCoreCliCommand {
	dotnetCoreCliCmd := DotnetCoreCliCommand{&DotnetCommand{}}
	dotnetCoreCliCmd.SetToolchainType(dotnet.DotnetCore)
	return &dotnetCoreCliCmd
}

func (dccc *DotnetCoreCliCommand) Run() (err error) {
	return dccc.Exec()
}
