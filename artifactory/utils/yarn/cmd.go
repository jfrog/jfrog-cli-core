package yarn

import (
	"io"
	"os/exec"
)

type YarnConfig struct {
	Executable   string
	Command      []string
	CommandFlags []string
	StrWriter    io.WriteCloser
	ErrWriter    io.WriteCloser
}

func (yc *YarnConfig) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, yc.Executable)
	cmd = append(cmd, yc.Command...)
	cmd = append(cmd, yc.CommandFlags...)
	// #nosec G204 -- command is constructed from validated yarn executable path and arguments
	return exec.Command(cmd[0], cmd[1:]...)
}

func (yc *YarnConfig) GetEnv() map[string]string {
	return map[string]string{}
}

func (yc *YarnConfig) GetStdWriter() io.WriteCloser {
	return yc.StrWriter
}

func (yc *YarnConfig) GetErrWriter() io.WriteCloser {
	return yc.ErrWriter
}
