package project

import (
	"path/filepath"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

const (
	buildFileName = "build.yml"
)

type ProjectInitCommand struct {
	projectPath string
}

func NewProjectInitCommand(path string) *ProjectInitCommand {
	return &ProjectInitCommand{
		projectPath: path,
	}
}

func (pic *ProjectInitCommand) Run() (err error) {
	technologiesMap, err := coreutils.DetectTechnologies(pic.projectPath)
	if err != nil {
		return err
	}
	// First create repositories for the detected technologies.
	for tech, detected := range technologiesMap {
		if detected {
			// First create repositories for the detected technology.
			err = createDefaultReposIfNeeded(tech)
			if err != nil {
				return err
			}
			err = createDefaultProject(tech)
			if err != nil {
				return err
			}
		}
	}
	// Create build config
	return pic.createBuildConfig()
}

func (pic *ProjectInitCommand) createBuildConfig() error {
	jfrogProjectDir := filepath.Join(pic.projectPath, ".jfrog", "projects")
	if err := fileutils.CreateDirIfNotExist(jfrogProjectDir); err != nil {
		return err
	}
	configFilePath := filepath.Join(jfrogProjectDir, buildFileName)
	return nil
}

func createDefaultReposIfNeeded(tech coreutils.Technology) error {
	return nil
}

func createDefaultProject(tech coreutils.Technology) error {
	return nil
}
