package python

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/python"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/python/dependencies"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"os/exec"
	"path/filepath"
)

type PythonCommand struct {
	rtDetails              *config.ServerDetails
	args                   []string
	repository             string
	buildConfiguration     *utils.BuildConfiguration
	shouldCollectBuildInfo bool
}

func (pc *PythonCommand) SetServerDetails(serverDetails *config.ServerDetails) *PythonCommand {
	pc.rtDetails = serverDetails
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

func (pc *PythonCommand) collectBuildInfo(cacheDirPath string, allDependencies map[string]*buildinfo.Dependency, buildInfoType buildinfo.ModuleType) error {
	if err := pc.determineModuleName(); err != nil {
		return err
	}
	// Populate dependencies information - checksums and file-name.
	servicesManager, err := utils.CreateServiceManager(pc.rtDetails, -1, false)
	if err != nil {
		return err
	}
	err = dependencies.UpdateDepsChecksumInfo(allDependencies, cacheDirPath, servicesManager, pc.repository)
	if err != nil {
		return err
	}
	err = dependencies.UpdateDependenciesCache(allDependencies, cacheDirPath)
	if err != nil {
		return err
	}
	return pc.saveBuildInfo(allDependencies, buildInfoType)
}

func (pc *PythonCommand) saveBuildInfo(allDependencies map[string]*buildinfo.Dependency, buildInfoType buildinfo.ModuleType) error {
	buildInfo := &buildinfo.BuildInfo{}
	var modules []buildinfo.Module
	var projectDependencies []buildinfo.Dependency

	for _, dep := range allDependencies {
		projectDependencies = append(projectDependencies, *dep)
	}

	// Save build-info.
	module := buildinfo.Module{Id: pc.buildConfiguration.Module, Type: buildInfoType, Dependencies: projectDependencies}
	modules = append(modules, module)

	buildInfo.Modules = modules
	return utils.SaveBuildInfo(pc.buildConfiguration.BuildName, pc.buildConfiguration.BuildNumber, pc.buildConfiguration.Project, buildInfo)
}

func (pc *PythonCommand) determineModuleName() error {
	pythonExecutablePath, err := GetExecutablePath("python")
	if err != nil {
		return err
	}
	// If module-name was set in command, don't change it.
	if pc.buildConfiguration.Module != "" {
		return nil
	}

	// Get package-name.
	moduleName, err := getPackageName(pythonExecutablePath)
	if err != nil {
		return err
	}

	// If package-name unknown, set module as build-name.
	if moduleName == "" {
		moduleName = pc.buildConfiguration.BuildName
	}

	pc.buildConfiguration.Module = moduleName
	return nil
}

func (pc *PythonCommand) prepareBuildPrerequisites() (err error) {
	log.Debug("Preparing build prerequisites.")
	pc.args, pc.buildConfiguration, err = utils.ExtractBuildDetailsFromArgs(pc.args)
	if err != nil {
		return
	}

	// Prepare build-info.
	if pc.buildConfiguration.BuildName != "" && pc.buildConfiguration.BuildNumber != "" {
		pc.shouldCollectBuildInfo = true
		if err = utils.SaveBuildGeneralDetails(pc.buildConfiguration.BuildName, pc.buildConfiguration.BuildNumber, pc.buildConfiguration.Project); err != nil {
			return
		}
	}
	return
}

func GetExecutablePath(executableName string) (executablePath string, err error) {
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
	// Build uses setup.py file.
	// Setup.py should be in current dir.
	filePath, err := getSetupPyFilePath()
	if err != nil || filePath == "" {
		// Error was returned or setup.py does not exist in directory.
		return "", err
	}

	// Extract package name from setup.py.
	packageName, err := python.ExtractPackageNameFromSetupPy(filePath, pythonExecutablePath)
	if err != nil {
		return "", errors.New("Failed determining module-name from 'setup.py' file: " + err.Error())
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

func (pc *PythonCommand) cleanBuildInfoDir() {
	if err := utils.RemoveBuildDir(pc.buildConfiguration.BuildName, pc.buildConfiguration.BuildNumber, pc.buildConfiguration.Project); err != nil {
		log.Error(fmt.Sprintf("Failed cleaning build-info directory: %s", err.Error()))
	}
}
