package goutils

import (
	"github.com/jfrog/gocmd/cmd"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io"
	"os/exec"
)

type Cmd struct {
	Go           string
	Command      []string
	CommandFlags []string
	Dir          string
	StrWriter    io.WriteCloser
	ErrWriter    io.WriteCloser
}

func (config *Cmd) GetCmd() (cmd *exec.Cmd) {
	var cmdStr []string
	cmdStr = append(cmdStr, config.Go)
	cmdStr = append(cmdStr, config.Command...)
	cmdStr = append(cmdStr, config.CommandFlags...)
	cmd = exec.Command(cmdStr[0], cmdStr[1:]...)
	cmd.Dir = config.Dir
	return
}

func (config *Cmd) GetEnv() map[string]string {
	return map[string]string{}
}

func (config *Cmd) GetStdWriter() io.WriteCloser {
	return config.StrWriter
}

func (config *Cmd) GetErrWriter() io.WriteCloser {
	return config.ErrWriter
}

func LogGoVersion() error {
	output, err := GetGoVersion()
	if err != nil {
		return errorutils.CheckError(err)
	}
	log.Info("Using go:", output)
	return nil
}

func GetGoVersion() (string, error) {
	path, err := cmd.GetGoVersion()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return path, nil
}

func GetCachePath() (string, error) {
	path, err := cmd.GetCachePath()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return path, nil
}

func GetGoModCachePath() (string, error) {
	path, err := cmd.GetGoModCachePath()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return path, nil
}

func GetProjectRoot() (string, error) {
	path, err := cmd.GetProjectRoot()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return path, nil
}

func GetModuleName(projectDir string) (string, error) {
	path, err := cmd.GetModuleNameByDir(projectDir)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return path, nil
}

func GetDependenciesList(projectDir string) (map[string]bool, error) {
	deps, err := cmd.GetDependenciesList(projectDir)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return deps, nil
}

func GetDependenciesGraph(projectDir string) (map[string][]string, error) {
	deps, err := cmd.GetDependenciesGraph(projectDir)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return deps, nil
}
