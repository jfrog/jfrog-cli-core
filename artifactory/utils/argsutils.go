package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
)

func ExtractBuildDetailsFromArgs(args []string) (cleanArgs []string, buildConfig *BuildConfiguration, err error) {
	var flagIndex, valueIndex int
	buildConfig = &BuildConfiguration{}
	cleanArgs = append([]string(nil), args...)

	// Extract build-info information from the args.
	flagIndex, valueIndex, buildConfig.BuildName, err = coreutils.FindFlag("--build-name", cleanArgs)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&cleanArgs, flagIndex, valueIndex)

	flagIndex, valueIndex, buildConfig.BuildNumber, err = coreutils.FindFlag("--build-number", cleanArgs)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&cleanArgs, flagIndex, valueIndex)

	flagIndex, valueIndex, buildConfig.Project, err = coreutils.FindFlag("--project", cleanArgs)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&cleanArgs, flagIndex, valueIndex)

	// Retrieve build name and build number from env if both missing
	buildConfig.BuildName, buildConfig.BuildNumber = GetBuildNameAndNumber(buildConfig.BuildName, buildConfig.BuildNumber)
	// Retrieve project from env if missing
	buildConfig.Project = GetBuildProject(buildConfig.Project)

	flagIndex, valueIndex, buildConfig.Module, err = coreutils.FindFlag("--module", cleanArgs)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&cleanArgs, flagIndex, valueIndex)
	err = ValidateBuildAndModuleParams(buildConfig)
	return
}
