package pip

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"net/url"
	"os/exec"
)

type CommonExecutor struct {
	ServerDetails *config.ServerDetails
	Args          []string
	Repository    string
}

func (pce *CommonExecutor) prepare() (pipExecutablePath, pipIndexUrl string, err error) {
	log.Debug("Preparing prerequisites...")

	pipExecutablePath, err = exec.LookPath("pip")
	if err != nil {
		return
	}
	if pipExecutablePath == "" {
		return "", "", errorutils.CheckErrorf("could not find the 'pip' executable in the system PATH")
	}
	pipIndexUrl, err = getArtifactoryUrlWithCredentials(pce.ServerDetails, pce.Repository)
	return
}

func getArtifactoryUrlWithCredentials(serverDetails *config.ServerDetails, repository string) (string, error) {
	rtUrl, err := url.Parse(serverDetails.GetArtifactoryUrl())
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	username := serverDetails.GetUser()
	password := serverDetails.GetPassword()

	// Get credentials from access-token if exists.
	if serverDetails.GetAccessToken() != "" {
		username, err = auth.ExtractUsernameFromAccessToken(serverDetails.GetAccessToken())
		if err != nil {
			return "", err
		}
		password = serverDetails.GetAccessToken()
	}

	if username != "" && password != "" {
		rtUrl.User = url.UserPassword(username, password)
	}
	rtUrl.Path += "api/pypi/" + repository + "/simple"

	return rtUrl.String(), nil
}
