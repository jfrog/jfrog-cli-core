package goutils

import (
	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io"
	"os/exec"
)

type GoCmdConfig struct {
	Go           string
	Command      []string
	CommandFlags []string
	Dir          string
	StrWriter    io.WriteCloser
	ErrWriter    io.WriteCloser
}

func NewGoCmdConfig() (*GoCmdConfig, error) {
	execPath, err := exec.LookPath("go")
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return &GoCmdConfig{Go: execPath}, nil
}

func (config *GoCmdConfig) GetCmd() (cmd *exec.Cmd) {
	var cmdStr []string
	cmdStr = append(cmdStr, config.Go)
	cmdStr = append(cmdStr, config.Command...)
	cmdStr = append(cmdStr, config.CommandFlags...)
	cmd = exec.Command(cmdStr[0], cmdStr[1:]...)
	cmd.Dir = config.Dir
	return
}

func (config *GoCmdConfig) GetEnv() map[string]string {
	return map[string]string{}
}

func (config *GoCmdConfig) GetStdWriter() io.WriteCloser {
	return config.StrWriter
}

func (config *GoCmdConfig) GetErrWriter() io.WriteCloser {
	return config.ErrWriter
}

func LogGoVersion() error {
	version, err := utils.GetParsedGoVersion()
	if err != nil {
		return errorutils.CheckError(err)
	}
	log.Info("Using go:", version.GetVersion())
	return nil
}

func GetGoModCachePath() (string, error) {
	path, err := utils.GetGoModCachePath()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return path, nil
}

func GetProjectRoot() (string, error) {
	path, err := utils.GetProjectRoot()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return path, nil
}

func GetModuleName(projectDir string) (string, error) {
	path, err := utils.GetModuleNameByDir(projectDir, log.Logger)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return path, nil
}

func GetDependenciesList(projectDir string) (map[string]bool, error) {
	deps, err := utils.GetDependenciesList(projectDir, log.Logger)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return deps, nil
}

func GetDependenciesGraph(projectDir string) (map[string][]string, error) {
	deps, err := utils.GetDependenciesGraph(projectDir, log.Logger)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return deps, nil
}
