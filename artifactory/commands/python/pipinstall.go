package python

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/python/pip"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
)

type PipInstallCommand struct {
	*PythonCommand
}

func NewPipInstallCommand() *PipInstallCommand {
	return &PipInstallCommand{PythonCommand: &PythonCommand{}}
}

func (pic *PipInstallCommand) Run() error {
	log.Info("Running pip Install.")

	err := pic.prepareBuildPrerequisites()
	defer func() {
		if err != nil {
			pic.cleanBuildInfoDir()
		}
	}()

	pipenvExecutablePath, err := GetExecutablePath("pip")
	if err != nil {
		return err
	}
	pipInstaller := &pip.PipInstaller{Args: pic.args, ServerDetails: pic.rtDetails, Repository: pic.repository, ShouldParseLogs: pic.shouldCollectBuildInfo}
	err = pipInstaller.Install(pipenvExecutablePath)
	if err != nil {
		return err
	}

	if !pic.shouldCollectBuildInfo {
		log.Info("pip install finished successfully.")
		return nil
	}

	// Collect build-info.
	projectsDirPath, err := os.Getwd()
	if err != nil {
		return err
	}
	allDependencies := pic.getAllDependencies(pipInstaller.DependencyToFileMap)
	if err := pic.collectBuildInfo(projectsDirPath, allDependencies, buildinfo.Pip); err != nil {
		return err
	}

	log.Info("pip install finished successfully.")
	return nil
}

// Convert dependencyToFileMap to Dependencies map.
func (pc *PipInstallCommand) getAllDependencies(dependencyToFileMap map[string]string) map[string]*buildinfo.Dependency {
	dependenciesMap := make(map[string]*buildinfo.Dependency, len(dependencyToFileMap))
	for depName := range dependencyToFileMap {
		dependenciesMap[depName] = &buildinfo.Dependency{Id: depName}
	}

	return dependenciesMap
}

func (pic *PipInstallCommand) CommandName() string {
	return "rt_pip_install"
}

func (pic *PipInstallCommand) ServerDetails() (*config.ServerDetails, error) {
	return pic.rtDetails, nil
}
