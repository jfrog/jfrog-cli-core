package python

import (
	"errors"
	"fmt"
	"github.com/jfrog/build-info-go/build"
	"github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/build-info-go/utils/pythonutils"
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/python/dependencies"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io"
	"net/url"
	"os/exec"
)

type PythonCommand struct {
	serverDetails *config.ServerDetails
	projectType   utils.ProjectType
	executable    string
	commandName   string
	args          []string
	repository    string
}

func NewPythonCommand(projectType utils.ProjectType) *PythonCommand {
	return &PythonCommand{projectType: projectType}
}

func (pc *PythonCommand) Run() (err error) {
	log.Info(fmt.Sprintf("Running %s %s.", utils.ProjectTypes[pc.projectType], pc.commandName))
	var buildConfiguration *utils.BuildConfiguration
	pc.args, buildConfiguration, err = utils.ExtractBuildDetailsFromArgs(pc.args)
	if err != nil {
		return err
	}
	pythonBuildInfo, err := utils.PrepareBuildPrerequisites(buildConfiguration)
	if err != nil {
		return
	}
	defer func() {
		if pythonBuildInfo != nil && err != nil {
			e := pythonBuildInfo.Clean()
			if e != nil {
				err = errors.New(err.Error() + "\n" + e.Error())
			}
		}
	}()
	err = pc.setPypiRepoUrlWithCredentials()
	if err != nil {
		return nil
	}

	if pythonBuildInfo != nil && pc.commandName == "install" {
		// Need to collect build info
		var pythonTool pythonutils.PythonTool
		switch pc.projectType {
		case utils.Pip:
			pythonTool = pythonutils.Pip
		case utils.Pipenv:
			pythonTool = pythonutils.Pipenv
		default:
			return errors.New(fmt.Sprintf("Build info dependencies collection for %s commands is not supported.", utils.ProjectTypes[pc.projectType]))
		}
		if err != nil {
			err = errorutils.CheckError(err)
			return
		}
		var pythonModule *build.PythonModule
		pythonModule, err = pythonBuildInfo.AddPythonModule("", pythonTool)
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
		if err != nil {
			return
		}
	} else {
		// Python native command
		err = pc.SetExecutablePath()
		if err != nil {
			return nil
		}
		err = gofrogcmd.RunCmd(pc)
		if err != nil {
			return
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

func (pc *PythonCommand) SetExecutablePath() error {

	executablePath, err := exec.LookPath(utils.ProjectTypes[pc.projectType])
	if err != nil {
		return err
	}
	if executablePath == "" {
		return errorutils.CheckError(errors.New("Could not find the" + utils.ProjectTypes[pc.projectType] + " executable in the system PATH"))
	}
	pc.executable = executablePath
	return nil
}

func (pc *PythonCommand) setPypiRepoUrlWithCredentials() error {
	rtUrl, err := url.Parse(pc.serverDetails.GetArtifactoryUrl())
	if err != nil {
		return errorutils.CheckError(err)
	}

	username := pc.serverDetails.GetUser()
	password := pc.serverDetails.GetPassword()

	// Get credentials from access-token if exists.
	if pc.serverDetails.GetAccessToken() != "" {
		username, err = auth.ExtractUsernameFromAccessToken(pc.serverDetails.GetAccessToken())
		if err != nil {
			return err
		}
		password = pc.serverDetails.GetAccessToken()
	}

	if username != "" && password != "" {
		rtUrl.User = url.UserPassword(username, password)
	}
	rtUrl.Path += "api/pypi/" + pc.repository + "/simple"

	if pc.projectType == utils.Pip {
		pc.args = append(pc.args, "-i")
	} else if pc.projectType == utils.Pipenv {
		pc.args = append(pc.args, "--pypi-mirror")
	}
	pc.args = append(pc.args, rtUrl.String())
	return nil
}

func (pc *PythonCommand) CommandName() string {
	return "rt_python_command"
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
	cmd = append(cmd, pc.executable)
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
