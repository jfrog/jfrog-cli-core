package python

import (
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/python/pipenv"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	pipenvutils "github.com/jfrog/jfrog-cli-core/v2/utils/python"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type PipenvInstallCommand struct {
	*PythonCommand
}

func NewPipenvInstallCommand() *PipenvInstallCommand {
	return &PipenvInstallCommand{PythonCommand: &PythonCommand{}}
}

func (pic *PipenvInstallCommand) Run() error {
	log.Info("Running pipenv Install.")

	err := pic.prepareBuildPrerequisites()
	defer func() {
		if err != nil {
			pic.cleanBuildInfoDir()
		}
	}()

	pipenvExecutablePath, err := GetExecutablePath("pipenv")
	pipenvInstaller := &pipenv.PipenvInstaller{Args: pic.args, ServerDetails: pic.rtDetails, Repository: pic.repository, ShouldParseLogs: pic.shouldCollectBuildInfo}
	err = pipenvInstaller.Install(pipenvExecutablePath)
	if err != nil {
		return err
	}

	if !pic.shouldCollectBuildInfo {
		log.Info("pipenv install finished successfully.")
		return nil
	}

	allDepsList, err := pipenvutils.GetPipenvDependenciesList("")
	if err != nil {
		return err
	}

	allDependencies := pic.getAllDependencies(allDepsList, pipenvInstaller.DependencyToFileMap)
	venvDirPath, err := pipenvutils.GetPipenvVenv("")
	if err != nil {
		return err
	}
	// Collect build-info.
	if err := pic.collectBuildInfo(venvDirPath, allDependencies, buildinfo.Pipenv); err != nil {
		return err
	}

	log.Info("pipenv install finished successfully.")
	return nil
}

// Convert dependencyToFileMap to Dependencies map.
func (pc *PipenvInstallCommand) getAllDependencies(allDepsList map[string]bool, dependencyToFileMap map[string]string) map[string]*buildinfo.Dependency {
	dependenciesMap := make(map[string]*buildinfo.Dependency, len(dependencyToFileMap))
	for depName := range allDepsList {
		dependenciesMap[depName] = &buildinfo.Dependency{Id: dependencyToFileMap[depName]}
	}
	return dependenciesMap
}

func (pic *PipenvInstallCommand) CommandName() string {
	return "rt_pipenv_install"
}

func (pic *PipenvInstallCommand) ServerDetails() (*config.ServerDetails, error) {
	return pic.rtDetails, nil
}
