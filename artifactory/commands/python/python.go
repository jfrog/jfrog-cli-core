package python

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/python/dependencies"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type PythonCommand struct {
	serverDetails          *config.ServerDetails
	executable             string
	commandName            string
	args                   []string
	repository             string
	buildConfiguration     *utils.BuildConfiguration
	shouldCollectBuildInfo bool
}

func (pc *PythonCommand) SetServerDetails(serverDetails *config.ServerDetails) *PythonCommand {
	pc.serverDetails = serverDetails
	return pc
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

func (pc *PythonCommand) collectBuildInfo(cacheDirPath, pythonExecutablePath string, allDependencies map[string]*buildinfo.Dependency, dependenciesGraph map[string][]string) error {
	err, packageName := pc.determineModuleName(pythonExecutablePath)
	if err != nil {
		return err
	}
	// Populate dependencies information - checksums, file-name and IDs
	servicesManager, err := utils.CreateServiceManager(pc.serverDetails, -1, 0, false)
	if err != nil {
		return err
	}
	err = dependencies.UpdateDepsChecksumInfo(allDependencies, cacheDirPath, servicesManager, pc.repository)
	if err != nil {
		return err
	}
	dependencies.UpdateDepsIdsAndRequestedBy(allDependencies, dependenciesGraph, packageName, pc.buildConfiguration.GetModule())

	err = dependencies.UpdateDependenciesCache(allDependencies, cacheDirPath)
	if err != nil {
		return err
	}
	return pc.saveBuildInfo(allDependencies)
}

func (pc *PythonCommand) saveBuildInfo(allDependencies map[string]*buildinfo.Dependency) error {
	buildInfo := &buildinfo.BuildInfo{}
	var modules []buildinfo.Module
	var projectDependencies []buildinfo.Dependency

	for _, dep := range allDependencies {
		projectDependencies = append(projectDependencies, *dep)
	}

	// Save build-info.
	module := buildinfo.Module{Id: pc.buildConfiguration.GetModule(), Type: buildinfo.Python, Dependencies: projectDependencies}
	modules = append(modules, module)

	buildInfo.Modules = modules
	buildName, err := pc.buildConfiguration.GetBuildName()
	if err != nil {
		return err
	}
	buildNumber, err := pc.buildConfiguration.GetBuildNumber()
	if err != nil {
		return err
	}
	return utils.SaveBuildInfo(buildName, buildNumber, pc.buildConfiguration.GetProject(), buildInfo)
}

// Determine the module name and return the package name
func (pc *PythonCommand) determineModuleName(pythonExecutablePath string) (error, string) {
	// Get package-name.
	packageName, pkgNameErr := getPackageName(pythonExecutablePath)

	// If module-name was set by the command, don't change it.
	if pc.buildConfiguration.GetModule() != "" {
		if pkgNameErr != nil {
			log.Debug("Couldn't retrieve the package name. Using module name '"+pc.buildConfiguration.GetModule()+"'. Reason: ", pkgNameErr.Error())
		}
		return nil, packageName
	}

	if pkgNameErr != nil {
		return pkgNameErr, ""
	}

	// If the package name is unknown, set the module name to be the build name.
	if packageName == "" {
		buildName, err := pc.buildConfiguration.GetBuildName()
		if err != nil {
			return err, packageName
		}
		packageName = buildName
		log.Debug(fmt.Sprintf("Using build name: %s as module name.", buildName))
	}

	pc.buildConfiguration.SetModule(packageName)
	return nil, packageName
}

func (pc *PythonCommand) prepareBuildPrerequisites() (err error) {
	log.Debug("Preparing build prerequisites...")
	pc.args, pc.buildConfiguration, err = utils.ExtractBuildDetailsFromArgs(pc.args)
	if err != nil {
		return
	}

	// Prepare build-info.
	toCollect, err := pc.buildConfiguration.IsCollectBuildInfo()
	if err != nil {
		return
	}
	if toCollect {
		var buildName, buildNumber string
		buildName, err = pc.buildConfiguration.GetBuildName()
		if err != nil {
			return err
		}
		buildNumber, err = pc.buildConfiguration.GetBuildNumber()
		if err != nil {
			return err
		}
		pc.shouldCollectBuildInfo = true
		if err = utils.SaveBuildGeneralDetails(buildName, buildNumber, pc.buildConfiguration.GetProject()); err != nil {
			return
		}
	}
	return
}

func getExecutablePath(executableName string) (executablePath string, err error) {
	executablePath, err = exec.LookPath(executableName)
	if err != nil {
		return
	}
	if executablePath == "" {
		return "", errorutils.CheckError(errors.New("Could not find the" + executableName + " executable in the system PATH"))
	}

	return executablePath, nil
}

func getPackageName(pythonExecutablePath string) (string, error) {
	filePath, err := getSetupPyFilePath()
	if err != nil || filePath == "" {
		// Error was returned or setup.py does not exist in directory.
		return "", err
	}

	// Extract package name from setup.py.
	packageName, err := ExtractPackageNameFromSetupPy(filePath, pythonExecutablePath)
	if err != nil {
		if err != nil {
			// If setup.py egg_info command failed we use build name as module name and continue to pip-install execution
			log.Info("Couldn't determine module-name after running the 'egg_info' command: " + err.Error())
			return "", nil
		}
	}
	return packageName, err
}

// Look for 'setup.py' file in current work dir.
// If found, return its absolute path.
func getSetupPyFilePath() (string, error) {
	wd, err := os.Getwd()
	if errorutils.CheckError(err) != nil {
		return "", err
	}

	filePath := filepath.Join(wd, "setup.py")
	// Check if setup.py exists.
	validPath, err := fileutils.IsFileExists(filePath, false)
	if err != nil {
		return "", err
	}
	if !validPath {
		log.Debug("Could not find setup.py file in current directory:", wd)
		return "", nil
	}

	return filePath, nil
}

func (pc *PythonCommand) cleanBuildInfoDir() error {
	buildName, err := pc.buildConfiguration.GetBuildName()
	if err != nil {
		return errors.New("Failed cleaning build-info directory while getting build name:" + err.Error())
	}
	buildNumber, err := pc.buildConfiguration.GetBuildNumber()
	if err != nil {
		return errors.New("Failed cleaning build-info directory while getting build name:" + err.Error())
	}
	if err := utils.RemoveBuildDir(buildName, buildNumber, pc.buildConfiguration.GetProject()); err != nil {
		return errors.New("Failed cleaning build-info directory:" + err.Error())
	}
	return nil
}

func (pc *PythonCommand) setPypiRepoUrlWithCredentials(serverDetails *config.ServerDetails, repository string, projectType utils.ProjectType) error {
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

	if projectType == utils.Pip {
		pc.args = append(pc.args, "-i")
	} else if projectType == utils.Pipenv {
		pc.args = append(pc.args, "--pypi-mirror")
	}
	pc.args = append(pc.args, rtUrl.String())
	return nil
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
