package coreutils

import (
	"errors"
	"os"
)

type BuildConfiguration struct {
	BuildName   string
	BuildNumber string
	Module      string
	Project     string
}

func ValidateBuildAndModuleParams(buildConfig *BuildConfiguration) error {
	if (buildConfig.BuildName == "" && buildConfig.BuildNumber != "") || (buildConfig.BuildName != "" && buildConfig.BuildNumber == "") {
		return errors.New("the build-name and build-number options cannot be provided separately")
	}
	if buildConfig.Module != "" && buildConfig.BuildName == "" && buildConfig.BuildNumber == "" {
		return errors.New("the build-name and build-number options are mandatory when the module option is provided")
	}
	return nil
}

// Get build name and number from env, only if both were not provided
func GetBuildNameAndNumber(buildName, buildNumber string) (string, string) {
	if buildName != "" || buildNumber != "" {
		return buildName, buildNumber
	}
	return os.Getenv(BuildName), os.Getenv(BuildNumber)
}

// Get build project from env, if not provided
func GetBuildProject(buildProject string) string {
	if buildProject != "" {
		return buildProject
	}
	return os.Getenv(Project)
}
