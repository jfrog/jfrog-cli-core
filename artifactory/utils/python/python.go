package python

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"io"
	"net/url"
	"os/exec"
)

func (pc *PythonCmd) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, pc.Executable)
	cmd = append(cmd, pc.Command)
	cmd = append(cmd, pc.CommandArgs...)
	return exec.Command(cmd[0], cmd[1:]...)
}

func (pc *PythonCmd) GetEnv() map[string]string {
	return pc.EnvVars
}

func (pc *PythonCmd) GetStdWriter() io.WriteCloser {
	return pc.StrWriter
}

func (pc *PythonCmd) GetErrWriter() io.WriteCloser {
	return pc.ErrWriter
}

func (pc *PythonCmd) SetPypiRepoUrlWithCredentials(serverDetails *config.ServerDetails, repository string) error {
	rtUrl, err := url.Parse(serverDetails.GetArtifactoryUrl())
	if err != nil {
		return errorutils.CheckError(err)
	}

	username := serverDetails.GetUser()
	password := serverDetails.GetPassword()

	// Get credentials from access-token if exists.
	if serverDetails.GetAccessToken() != "" {
		username, err = auth.ExtractUsernameFromAccessToken(serverDetails.GetAccessToken())
		if err != nil {
			return err
		}
		password = serverDetails.GetAccessToken()
	}

	if username != "" && password != "" {
		rtUrl.User = url.UserPassword(username, password)
	}
	rtUrl.Path += "api/pypi/" + repository + "/simple"

	pc.CommandArgs = append(pc.CommandArgs, rtUrl.String())
	return nil
}

type PythonCmd struct {
	Executable  string
	Command     string
	CommandArgs []string
	EnvVars     map[string]string
	StrWriter   io.WriteCloser
	ErrWriter   io.WriteCloser
}
