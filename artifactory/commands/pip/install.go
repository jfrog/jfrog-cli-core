package pip

import (
	"errors"
	"fmt"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	piputils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/pip"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/pip/dependencies"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type PipInstallCommand struct {
	*PipCommand
	buildConfiguration     *utils.BuildConfiguration
	shouldCollectBuildInfo bool
}

func NewPipInstallCommand() *PipInstallCommand {
	return &PipInstallCommand{PipCommand: &PipCommand{}}
}

func (pic *PipInstallCommand) Run() error {
	log.Info("Running pip Install.")

	pythonExecutablePath, err := pic.prepare()
	if err != nil {
		return err
	}

	pipInstaller := &piputils.PipInstaller{CommonExecutor: piputils.CommonExecutor{Args: pic.args, ServerDetails: pic.rtDetails, Repository: pic.repository}, ShouldParseLogs: pic.shouldCollectBuildInfo}
	err = pipInstaller.Install()
	if err != nil {
		pic.cleanBuildInfoDir()
		return err
	}

	if !pic.shouldCollectBuildInfo {
		log.Info("pip install finished successfully.")
		return nil
	}

	// Collect build-info.
	if err := pic.collectBuildInfo(pythonExecutablePath, pipInstaller.DependencyToFileMap); err != nil {
		pic.cleanBuildInfoDir()
		return err
	}

	log.Info("pip install finished successfully.")
	return nil
}

func (pic *PipInstallCommand) collectBuildInfo(pythonExecutablePath string, dependencyToFileMap map[string]string) error {
	if err := pic.determineModuleName(pythonExecutablePath); err != nil {
		return err
	}

	allDependencies := pic.getAllDependencies(dependencyToFileMap)
	dependenciesCache, err := dependencies.GetProjectDependenciesCache()
	if err != nil {
		return err
	}

	// Populate dependencies information - checksums and file-name.
	servicesManager, err := utils.CreateServiceManager(pic.rtDetails, -1, false)
	if err != nil {
		return err
	}
	missingDeps, err := dependencies.AddDepsInfoAndReturnMissingDeps(allDependencies, dependenciesCache, dependencyToFileMap, servicesManager, pic.repository)
	if err != nil {
		return err
	}

	promptMissingDependencies(missingDeps)
	err = dependencies.UpdateDependenciesCache(allDependencies)
	if err != nil {
		return err
	}
	pic.saveBuildInfo(allDependencies)
	return nil
}

// Convert dependencyToFileMap to Dependencies map.
func (pic *PipInstallCommand) getAllDependencies(dependencyToFileMap map[string]string) map[string]*buildinfo.Dependency {
	dependenciesMap := make(map[string]*buildinfo.Dependency, len(dependencyToFileMap))
	for depName := range dependencyToFileMap {
		dependenciesMap[depName] = &buildinfo.Dependency{Id: depName}
	}

	return dependenciesMap
}

func (pic *PipInstallCommand) saveBuildInfo(allDependencies map[string]*buildinfo.Dependency) {
	buildInfo := &buildinfo.BuildInfo{}
	var modules []buildinfo.Module
	var projectDependencies []buildinfo.Dependency

	for _, dep := range allDependencies {
		projectDependencies = append(projectDependencies, *dep)
	}

	// Save build-info.
	module := buildinfo.Module{Id: pic.buildConfiguration.Module, Type: buildinfo.Pip, Dependencies: projectDependencies}
	modules = append(modules, module)

	buildInfo.Modules = modules
	utils.SaveBuildInfo(pic.buildConfiguration.BuildName, pic.buildConfiguration.BuildNumber, pic.buildConfiguration.Project, buildInfo)
}

func (pic *PipInstallCommand) determineModuleName(pythonExecutablePath string) error {
	// If module-name was set in command, don't change it.
	if pic.buildConfiguration.Module != "" {
		return nil
	}

	// Get package-name.
	moduleName, err := getPackageName(pythonExecutablePath, pic.args)
	if err != nil {
		return err
	}

	// If package-name unknown, set module as build-name.
	if moduleName == "" {
		moduleName = pic.buildConfiguration.BuildName
	}

	pic.buildConfiguration.Module = moduleName
	return nil
}

func (pic *PipInstallCommand) prepare() (pythonExecutablePath string, err error) {
	log.Debug("Preparing prerequisites.")

	pythonExecutablePath, err = exec.LookPath("python")
	if err != nil {
		return
	}
	if pythonExecutablePath == "" {
		return "", errorutils.CheckErrorf("Could not find the 'python' executable in the system PATH")
	}
	pic.args, pic.buildConfiguration, err = utils.ExtractBuildDetailsFromArgs(pic.args)
	if err != nil {
		return
	}

	// Prepare build-info.
	if pic.buildConfiguration.BuildName != "" && pic.buildConfiguration.BuildNumber != "" {
		pic.shouldCollectBuildInfo = true
		if err = utils.SaveBuildGeneralDetails(pic.buildConfiguration.BuildName, pic.buildConfiguration.BuildNumber, pic.buildConfiguration.Project); err != nil {
			return
		}
	}

	return
}

func getPackageName(pythonExecutablePath string, pipArgs []string) (string, error) {
	// Build uses setup.py file.
	// Setup.py should be in current dir.
	filePath, err := getSetupPyFilePath()
	if err != nil || filePath == "" {
		// Error was returned or setup.py does not exist in directory.
		return "", err
	}

	// Extract package name from setup.py.
	packageName, err := piputils.ExtractPackageNameFromSetupPy(filePath, pythonExecutablePath)
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

func (pic *PipInstallCommand) cleanBuildInfoDir() {
	if err := utils.RemoveBuildDir(pic.buildConfiguration.BuildName, pic.buildConfiguration.BuildNumber, pic.buildConfiguration.Project); err != nil {
		log.Error(fmt.Sprintf("Failed cleaning build-info directory: %s", err.Error()))
	}
}

func promptMissingDependencies(missingDeps []string) {
	if len(missingDeps) > 0 {
		log.Warn(strings.Join(missingDeps, "\n"))
		log.Warn("The pypi packages above could not be found in Artifactory or were not downloaded in this execution, therefore they are not included in the build-info.\n" +
			"Reinstalling in clean environment or using '--no-cache-dir' and '--force-reinstall' flags (in one execution only), will force downloading and populating Artifactory with these packages, and therefore resolve the issue.")
	}
}

func (pic *PipInstallCommand) CommandName() string {
	return "rt_pip_install"
}

func (pic *PipInstallCommand) ServerDetails() (*config.ServerDetails, error) {
	return pic.rtDetails, nil
}
