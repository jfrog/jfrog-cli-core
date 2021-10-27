package npmutils

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/version"
	"os/exec"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

func Version(executablePath string) (*version.Version, error) {
	data, err := getVersion(executablePath)
	if err != nil {
		return nil, err
	}
	return version.NewVersion(data), nil
}

func getVersion(executablePath string) (string, error) {
	command := exec.Command(executablePath, "-version")
	stderr := bytes.NewBuffer([]byte{})
	stdout := bytes.NewBuffer([]byte{})
	command.Stderr = stderr
	command.Stdout = stdout
	err := command.Run()
	return stdout.String(), coreutils.ConvertExitCodeError(errorutils.CheckError(errors.New(fmt.Sprintf("%s - %s", err, stderr))))
}

func GetNpmVersionAndExecPath() (*version.Version, string, error) {
	log.Debug("Getting npm executable path and version")
	npmExecPath, err := exec.LookPath("npm")
	if err != nil {
		return nil, "", errorutils.CheckError(err)
	}

	if npmExecPath == "" {
		return nil, "", errorutils.CheckError(errors.New("could not find the 'npm' executable in the system PATH"))
	}

	log.Debug("Using npm executable:", npmExecPath)

	npmVersion, err := Version(npmExecPath)
	if err != nil {
		return nil, "", err
	}
	log.Debug("Using npm version:", npmVersion.GetVersion())
	return npmVersion, npmExecPath, nil
}
