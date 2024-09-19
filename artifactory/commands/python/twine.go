package python

import (
	"errors"
	"fmt"
	"github.com/jfrog/build-info-go/build"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	buildUtils "github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"os/exec"
	"strings"
)

const (
	_configFileOptionKey     = "--config-file"
	_repositoryUrlOptionKey  = "--repository-url"
	_usernameOptionKey       = "--username"
	_passwordOptionKey       = "--password"
	_usernamePrefixOptionKey = "-u"
	_passwordPrefixOptionKey = "-p"
	_repositoryUrlEnvKey     = "TWINE_REPOSITORY_URL"
	_usernameEnvKey          = "TWINE_USERNAME"
	_passwordEnvKey          = "TWINE_PASSWORD"
	// Artifactory endpoint for pypi deployment.
	_apiPypi       = "api/pypi/"
	_twineExecName = "twine"
	_uploadCmdName = "upload"
)

var twineRepoConfigFlags = []string{_configFileOptionKey, _repositoryUrlOptionKey, _usernameOptionKey, _passwordOptionKey, _usernamePrefixOptionKey, _passwordPrefixOptionKey}

type TwineCommand struct {
	serverDetails      *config.ServerDetails
	commandName        string
	args               []string
	targetRepo         string
	buildConfiguration *buildUtils.BuildConfiguration
}

func NewTwineCommand(commandName string) *TwineCommand {
	return &TwineCommand{
		commandName: commandName,
	}
}

func (tc *TwineCommand) CommandName() string {
	return "twine_" + tc.commandName
}

func (tc *TwineCommand) ServerDetails() (*config.ServerDetails, error) {
	return tc.serverDetails, nil
}

func (tc *TwineCommand) SetServerDetails(serverDetails *config.ServerDetails) *TwineCommand {
	tc.serverDetails = serverDetails
	return tc
}

func (tc *TwineCommand) SetTargetRepo(targetRepo string) *TwineCommand {
	tc.targetRepo = targetRepo
	return tc
}

func (tc *TwineCommand) SetArgs(args []string) *TwineCommand {
	tc.args = args
	return tc
}

func (tc *TwineCommand) Run() (err error) {
	// Assert no forbidden flags were provided.
	if tc.isRepoConfigFlagProvided() {
		return errorutils.CheckErrorf(tc.getRepoConfigFlagProvidedErr())
	}
	if err = tc.extractAndFilterArgs(tc.args); err != nil {
		return err
	}
	callbackFunc, err := tc.setAuthEnvVars()
	defer func() {
		err = errors.Join(err, callbackFunc())
	}()

	collectBuild, err := tc.buildConfiguration.IsCollectBuildInfo()
	if err != nil {
		return err
	}
	// If build info is not collected, or this is not an upload command, run the twine command directly.
	if !collectBuild || tc.commandName != _uploadCmdName {
		return tc.runPlainTwineCommand()
	}
	return tc.uploadAndCollectBuildInfo()
}

func (tc *TwineCommand) extractAndFilterArgs(args []string) (err error) {
	cleanArgs := append([]string(nil), args...)
	cleanArgs, tc.buildConfiguration, err = buildUtils.ExtractBuildDetailsFromArgs(cleanArgs)
	if err != nil {
		return
	}
	tc.args = cleanArgs
	return
}

func (tc *TwineCommand) setAuthEnvVars() (callbackFunc func() error, err error) {
	oldRepoUrl := os.Getenv(_repositoryUrlEnvKey)
	oldUsername := os.Getenv(_usernameEnvKey)
	oldPassword := os.Getenv(_passwordEnvKey)
	callbackFunc = func() error {
		return errors.Join(os.Setenv(_repositoryUrlOptionKey, oldRepoUrl), os.Setenv(_usernameEnvKey, oldUsername), os.Setenv(_passwordEnvKey, oldPassword))
	}

	if err = os.Setenv(_repositoryUrlEnvKey, utils.AddTrailingSlashIfNeeded(tc.serverDetails.ArtifactoryUrl)+_apiPypi+tc.targetRepo); err != nil {
		return
	}

	username := tc.serverDetails.User
	password := tc.serverDetails.Password
	// Get credentials from access-token if exists.
	if tc.serverDetails.GetAccessToken() != "" {
		if username == "" {
			username = auth.ExtractUsernameFromAccessToken(tc.serverDetails.GetAccessToken())
		}
		password = tc.serverDetails.GetAccessToken()
	}

	if err = os.Setenv(_usernameEnvKey, username); err != nil {
		return
	}
	err = os.Setenv(_passwordEnvKey, password)
	return
}

func (tc *TwineCommand) runPlainTwineCommand() error {
	log.Debug("Running twine command:", tc.commandName, strings.Join(tc.args, " "))
	args := append([]string{tc.commandName}, tc.args...)
	cmd := exec.Command(_twineExecName, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (tc *TwineCommand) uploadAndCollectBuildInfo() error {
	buildInfo, err := buildUtils.PrepareBuildPrerequisites(tc.buildConfiguration)
	if err != nil {
		return err
	}

	defer func() {
		if buildInfo != nil && err != nil {
			err = errors.Join(err, buildInfo.Clean())
		}
	}()

	var pythonModule *build.PythonModule
	pythonModule, err = buildInfo.AddPythonModule("", pythonutils.Twine)
	if err != nil {
		return err
	}
	if tc.buildConfiguration.GetModule() != "" {
		pythonModule.SetName(tc.buildConfiguration.GetModule())
	}

	artifacts, err := pythonModule.TwineUploadWithLogParsing(tc.args)
	if err != nil {
		return err
	}
	for i := range artifacts {
		artifacts[i].OriginalDeploymentRepo = tc.targetRepo
	}
	if err = pythonModule.AddArtifacts(artifacts); err != nil {
		return err
	}
	log.Debug(fmt.Sprintf("Command finished successfully. %d artifacs were added to build info.", len(artifacts)))
	return nil
}

func (tc *TwineCommand) isRepoConfigFlagProvided() bool {
	for _, arg := range tc.args {
		for _, flag := range twineRepoConfigFlags {
			if strings.HasPrefix(arg, flag) {
				return true
			}
		}
	}
	return false
}

func (tc *TwineCommand) getRepoConfigFlagProvidedErr() string {
	return "twine command must not be executed with the following flags: " + coreutils.ListToText(twineRepoConfigFlags)
}
