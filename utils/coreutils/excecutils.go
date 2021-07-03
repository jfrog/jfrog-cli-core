package coreutils

import (
	"io"
	"os/exec"
)

// Command used to execute general commands.
type GeneralExecCmd struct {
	ExecPath string
	Command  []string
}

func (pluginCmd *GeneralExecCmd) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, pluginCmd.ExecPath)
	cmd = append(cmd, pluginCmd.Command...)
	return exec.Command(cmd[0], cmd[1:]...)
}

func (pluginCmd *GeneralExecCmd) GetEnv() map[string]string {
	return map[string]string{}
}

func (pluginCmd *GeneralExecCmd) GetStdWriter() io.WriteCloser {
	return nil
}

func (pluginCmd *GeneralExecCmd) GetErrWriter() io.WriteCloser {
	return nil
}
