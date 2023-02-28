package utils

import (
	"fmt"
	buildinfoutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/spf13/viper"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	pipenvRemoteRegistryFlag = "--pypi-mirror"
	pipRemoteRegistryFlag    = "-i"
	poetryConfigAuthPrefix   = "http-basic."
	poetryConfigRepoPrefix   = "repositories."
	pyproject                = "pyproject.toml"
)

func GetPypiRepoUrlWithCredentials(serverDetails *config.ServerDetails, repository string) (*url.URL, string, string, error) {
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
	rtUrl.Path += "api/pypi/" + repository + "/simple"
	return rtUrl, username, password, err
}

func GetPypiRemoteRegistryFlag(tool pythonutils.PythonTool) string {
	if tool == pythonutils.Pip {
		return pipRemoteRegistryFlag
	}
	return pipenvRemoteRegistryFlag
}

func GetPypiRepoUrl(serverDetails *config.ServerDetails, repository string) (*url.URL, error) {
	rtUrl, username, password, err := GetPypiRepoUrlWithCredentials(serverDetails, repository)
	if err != nil {
		return nil, err
	}
	if password != "" {
		rtUrl.User = url.UserPassword(username, password)
	}
	return rtUrl, err
}

func ConfigPoetryRepo(url, username, password, configRepoName string) error {
	// Add the poetry repository config
	err := runPoetryConfigCommand([]string{poetryConfigRepoPrefix + configRepoName, url}, false)
	if err != nil {
		return err
	}

	// Set the poetry repository credentials
	err = runPoetryConfigCommand([]string{poetryConfigAuthPrefix + configRepoName, username, password}, true)
	if err != nil {
		return err
	}

	// Add the repository config to the pyproject.toml
	currentDir, err := os.Getwd()
	if err != nil {
		return errorutils.CheckError(err)
	}
	return addRepoToPyprojectFile(filepath.Join(currentDir, pyproject), configRepoName, url)
}

func runPoetryConfigCommand(args []string, maskArgs bool) error {
	logMessage := "config "
	if maskArgs {
		logMessage += "***"
	} else {
		logMessage += strings.Join(args, " ")
	}
	log.Info(fmt.Sprintf("Running Poetry %s", logMessage))
	cmd := buildinfoutils.NewCommand("poetry", "config", args)
	err := gofrogcmd.RunCmd(cmd)
	if err != nil {
		return errorutils.CheckErrorf("Poetry config command failed with: %s", err.Error())
	}
	return nil
}

func addRepoToPyprojectFile(filepath, poetryRepoName, repoUrl string) error {
	viper.SetConfigType("toml")
	viper.SetConfigFile(filepath)
	err := viper.ReadInConfig()
	if err != nil {
		return errorutils.CheckErrorf("Failed to read pyproject.toml: %s", err.Error())
	}
	viper.Set("tool.poetry.source", []map[string]string{{"name": poetryRepoName, "url": repoUrl}})
	err = viper.WriteConfig()
	if err != nil {
		return errorutils.CheckErrorf("Failed to add tool.poetry.source to pyproject.toml: %s", err.Error())
	}
	log.Info(fmt.Sprintf("Added tool.poetry.source name:%q url:%q", poetryRepoName, repoUrl))
	log.Info("Running Poetry update")
	cmd := buildinfoutils.NewCommand("poetry", "update", []string{})
	err = gofrogcmd.RunCmd(cmd)
	if err != nil {
		return errorutils.CheckErrorf("Poetry config command failed with: %s", err.Error())
	}
	return err
}
