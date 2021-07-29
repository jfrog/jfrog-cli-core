package npmutils

import (
	"errors"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/version"
	"io"
	"io/ioutil"
	"os/exec"

	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

func Version(executablePath string) (*version.Version, error) {
	pipeReader, pipeWriter := io.Pipe()
	defer pipeReader.Close()
	defer pipeWriter.Close()
	var npmError error

	configListCmdConfig := createVersionCmdConfig(executablePath, pipeWriter)
	go func() {
		npmError = gofrogcmd.RunCmd(configListCmdConfig)
	}()

	data, err := ioutil.ReadAll(pipeReader)
	if err != nil {
		return nil, errorutils.CheckError(err)
	}

	if npmError != nil {
		return nil, errorutils.CheckError(npmError)
	}

	return version.NewVersion(string(data)), nil
}

func createVersionCmdConfig(executablePath string, pipeWriter *io.PipeWriter) *NpmConfig {
	return &NpmConfig{
		Npm:       executablePath,
		Command:   []string{"-version"},
		StrWriter: pipeWriter,
		ErrWriter: nil,
	}
}

func GetNpmVersionAndExecPath() (*version.Version, string, error) {
	log.Debug("Getting npm executable path and version")
	npmExecPath, err := exec.LookPath("npm")
	if err != nil {
		return nil, "", errorutils.CheckError(err)
	}

	if npmExecPath == "" {
		return nil, "", errorutils.CheckError(errors.New("could not find 'npm' executable"))
	}

	log.Debug("Using npm executable:", npmExecPath)

	npmVersion, err := Version(npmExecPath)
	if err != nil {
		return nil, "", err
	}
	log.Debug("Using npm version:", npmVersion)
	return npmVersion, npmExecPath, nil
}
