package python

import (
	"bytes"
	"errors"
	"github.com/jfrog/build-info-go/build"
	"github.com/jfrog/build-info-go/entities"
	buildInfoUtils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/python/dependencies"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	buildUtils "github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io"
	"net/url"
	"os"
	"os/exec"
)

const (
	pipenvRemoteRegistryFlag = "--pypi-mirror"
	pipRemoteRegistryFlag    = "-i"
)

type PythonCommand struct {
	serverDetails *config.ServerDetails
	pythonTool    pythonutils.PythonTool
	commandName   string
	args          []string
	repository    string
}

func NewPythonCommand(pythonTool pythonutils.PythonTool) *PythonCommand {
	return &PythonCommand{pythonTool: pythonTool}
}

func (pc *PythonCommand) Run() (err error) {
	log.Info("Running", string(pc.pythonTool), pc.commandName)
	var buildConfiguration *buildUtils.BuildConfiguration
	pc.args, buildConfiguration, err = buildUtils.ExtractBuildDetailsFromArgs(pc.args)
	if err != nil {
		return
	}
	pythonBuildInfo, err := buildUtils.PrepareBuildPrerequisites(buildConfiguration)
	if err != nil {
		return
	}
	defer func() {
		if pythonBuildInfo != nil && err != nil {
			err = errors.Join(err, pythonBuildInfo.Clean())
		}
	}()
	err = pc.SetPypiRepoUrlWithCredentials()
	if err != nil {
		return
	}

	if pythonBuildInfo != nil && pc.commandName == "install" {
		// Need to collect build info
		var pythonModule *build.PythonModule
		pythonModule, err = pythonBuildInfo.AddPythonModule("", pc.pythonTool)
		if err != nil {
			return
		}
		if buildConfiguration.GetModule() != "" {
			pythonModule.SetName(buildConfiguration.GetModule())
		}
		var localDependenciesPath string
		localDependenciesPath, err = config.GetJfrogDependenciesPath()
		if err != nil {
			return
		}
		pythonModule.SetLocalDependenciesPath(localDependenciesPath)
		pythonModule.SetUpdateDepsChecksumInfoFunc(pc.UpdateDepsChecksumInfoFunc)
		err = errorutils.CheckError(pythonModule.RunInstallAndCollectDependencies(pc.args))
	} else {
		// Python native command
		for k, v := range pc.GetEnv() {
			if err := os.Setenv(k, v); err != nil {
				return err
			}
		}

		cmd := pc.GetCmd()
		errBuffer := bytes.NewBuffer([]byte{})
		multiWriter := io.MultiWriter(os.Stderr, errBuffer)
		cmd.Stderr = multiWriter
		cmd.Stdout = os.Stdout

		err = cmd.Run()
		if err != nil {
			if buildInfoUtils.IsForbiddenOutput(buildInfoUtils.Pip, errBuffer.String()) {
				err = errors.Join(err, buildInfoUtils.NewForbiddenError())
			}
		}
	}
	return
}

func (pc *PythonCommand) UpdateDepsChecksumInfoFunc(dependenciesMap map[string]entities.Dependency, srcPath string) error {
	servicesManager, err := utils.CreateServiceManager(pc.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}
	return dependencies.UpdateDepsChecksumInfo(dependenciesMap, srcPath, servicesManager, pc.repository)
}

func (pc *PythonCommand) SetRepo(repo string) *PythonCommand {
	pc.repository = repo
	return pc
}

func (pc *PythonCommand) SetArgs(arguments []string) *PythonCommand {
	pc.args = arguments
	return pc
}

func (pc *PythonCommand) SetCommandName(commandName string) *PythonCommand {
	pc.commandName = commandName
	return pc
}

func (pc *PythonCommand) SetPypiRepoUrlWithCredentials() error {
	rtUrl, err := GetPypiRepoUrl(pc.serverDetails, pc.repository, false)
	if err != nil {
		return err
	}
	pc.args = append(pc.args, GetPypiRemoteRegistryFlag(pc.pythonTool), rtUrl)
	return nil
}

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

// Get the pypi repository embedded credentials URL (https://<user>:<password/token>@<your-artifactory-url>/artifactory/api/pypi/<repo-name>/simple)
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

func runConfigCommand(buildTool project.ProjectType, args []string) error {
	log.Debug("Running", buildTool.String(), "config command...")
	configCmd := gofrogcmd.NewCommand(buildTool.String(), "config", args)
	if err := gofrogcmd.RunCmd(configCmd); err != nil {
		return errorutils.CheckErrorf("%s config command failed with: %q", buildTool.String(), err)
	}
	return nil
}

func (pc *PythonCommand) SetServerDetails(serverDetails *config.ServerDetails) *PythonCommand {
	pc.serverDetails = serverDetails
	return pc
}

func (pc *PythonCommand) ServerDetails() (*config.ServerDetails, error) {
	return pc.serverDetails, nil
}

func (pc *PythonCommand) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, string(pc.pythonTool))
	cmd = append(cmd, pc.commandName)
	cmd = append(cmd, pc.args...)
	return exec.Command(cmd[0], cmd[1:]...)
}

func (pc *PythonCommand) GetEnv() map[string]string {
	return map[string]string{}
}

func (pc *PythonCommand) GetStdWriter() io.WriteCloser {
	return nil
}

func (pc *PythonCommand) GetErrWriter() io.WriteCloser {
	return nil
}
