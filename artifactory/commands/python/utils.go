package python

import (
	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/gofrog/io"
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"net/url"
)

const (
	pipenvRemoteRegistryFlag = "--pypi-mirror"
	pipRemoteRegistryFlag    = "-i"
	poetryConfigAuthPrefix   = "http-basic."
	poetryConfigRepoPrefix   = "repositories."
	pyproject                = "pyproject.toml"
)

// Get the pypi repository url and the credentials.
func GetPypiRepoUrlWithCredentials(serverDetails *config.ServerDetails, repository string, isCurationCmd bool) (*url.URL, string, string, error) {
	rtUrl, err := url.Parse(serverDetails.GetArtifactoryUrl())
	if err != nil {
		return nil, "", "", errorutils.CheckError(err)
	}

	username := serverDetails.GetUser()
	password := serverDetails.GetPassword()

	// Get credentials from access-token if exists.
	if serverDetails.GetAccessToken() != "" {
		if username == "" {
			username = auth.ExtractUsernameFromAccessToken(serverDetails.GetAccessToken())
		}
		password = serverDetails.GetAccessToken()
	}
	if isCurationCmd {
		rtUrl = rtUrl.JoinPath(coreutils.CurationPassThroughApi)
	}
	rtUrl = rtUrl.JoinPath("api/pypi", repository, "simple")
	return rtUrl, username, password, err
}

func GetPypiRemoteRegistryFlag(tool pythonutils.PythonTool) string {
	if tool == pythonutils.Pip {
		return pipRemoteRegistryFlag
	}
	return pipenvRemoteRegistryFlag
}

// Get the pypi repository embedded credentials URL (https://<user>:<token>@<your-artifactory-url>/artifactory/api/pypi/<repo-name>/simple)
func GetPypiRepoUrl(serverDetails *config.ServerDetails, repository string, isCurationCmd bool) (string, error) {
	rtUrl, username, password, err := GetPypiRepoUrlWithCredentials(serverDetails, repository, isCurationCmd)
	if err != nil {
		return "", err
	}
	if password != "" {
		rtUrl.User = url.UserPassword(username, password)
	}
	return rtUrl.String(), err
}

func RunConfigCommand(buildTool project.ProjectType, args []string) error {
	log.Debug("Running", buildTool.String(), "config command...")
	configCmd := io.NewCommand(buildTool.String(), "config", args)
	err := gofrogcmd.RunCmd(configCmd)
	if err != nil {
		return errorutils.CheckErrorf(buildTool.String()+" config command failed with: %s", err.Error())
	}
	return nil
}
