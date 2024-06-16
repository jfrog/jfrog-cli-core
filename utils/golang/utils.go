package goutils

import (
	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io"
	"net/url"
	"os/exec"
	"path"
	"strings"
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

type GoProxyUrlParams struct {
	// Fallback to retrieve the modules directly from the source if
	// the module failed to be retrieved from the proxy.
	// add |direct to the end of the url.
	// example: https://gocenter.io|direct
	Direct bool
	// The path from baseUrl to the standard Go repository path
	// URL structure: <baseUrl>/<EndpointPrefix>/api/go/<repoName>
	EndpointPrefix string
}

func (gdu *GoProxyUrlParams) BuildUrl(url *url.URL, repoName string) string {
	url.Path = path.Join(url.Path, gdu.EndpointPrefix, "api/go/", repoName)

	return gdu.addDirect(url.String())
}

func (gdu *GoProxyUrlParams) addDirect(url string) string {
	if gdu.Direct && !strings.HasSuffix(url, "|direct") {
		return url + "|direct"
	}
	return url
}

func GetArtifactoryRemoteRepoUrl(serverDetails *config.ServerDetails, repo string, goProxyParams GoProxyUrlParams) (string, error) {
	authServerDetails, err := serverDetails.CreateArtAuthConfig()
	if err != nil {
		return "", err
	}
	return getArtifactoryApiUrl(repo, authServerDetails, goProxyParams)
}

// Gets the URL of the specified repository Go API in Artifactory.
// The URL contains credentials (username and access token or password).
func getArtifactoryApiUrl(repoName string, details auth.ServiceDetails, goProxyParams GoProxyUrlParams) (string, error) {
	rtUrl, err := url.Parse(details.GetUrl())
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	username := details.GetUser()
	password := details.GetPassword()

	// Get credentials from access-token if exists.
	if details.GetAccessToken() != "" {
		log.Debug("Using proxy with access-token.")
		if username == "" {
			username = auth.ExtractUsernameFromAccessToken(details.GetAccessToken())
		}
		password = details.GetAccessToken()
	}
	if password != "" {
		rtUrl.User = url.UserPassword(username, password)
	}

	return goProxyParams.BuildUrl(rtUrl, repoName), nil
}
