package oc

import (
	"io"
	"os/exec"
)

type OcCommandConfig struct {
	Executable   string
	Command      []string
	CommandFlags []string
	StrWriter    io.WriteCloser
	ErrWriter    io.WriteCloser
}

func (occ *OcCommandConfig) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, occ.Executable)
	cmd = append(cmd, occ.Command...)
	cmd = append(cmd, occ.CommandFlags...)
	return exec.Command(cmd[0], cmd[1:]...)
}

func (occ *OcCommandConfig) GetEnv() map[string]string {
	return map[string]string{}
}

func (occ *OcCommandConfig) GetStdWriter() io.WriteCloser {
	return occ.StrWriter
}

func (occ *OcCommandConfig) GetErrWriter() io.WriteCloser {
	return occ.ErrWriter
}
