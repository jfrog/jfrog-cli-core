package build

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
)

func ExtractBuildDetailsFromArgs(args []string) (cleanArgs []string, buildConfig *BuildConfiguration, err error) {
	var flagIndex, valueIndex int
	var buildName, buildNumber, project, module string
	buildConfig = &BuildConfiguration{}
	cleanArgs = append([]string(nil), args...)

	// Extract build-info information from the args.
	flagIndex, valueIndex, buildName, err = coreutils.FindFlag("--build-name", cleanArgs)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&cleanArgs, flagIndex, valueIndex)

	flagIndex, valueIndex, buildNumber, err = coreutils.FindFlag("--build-number", cleanArgs)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&cleanArgs, flagIndex, valueIndex)

	flagIndex, valueIndex, project, err = coreutils.FindFlag("--project", cleanArgs)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&cleanArgs, flagIndex, valueIndex)

	flagIndex, valueIndex, module, err = coreutils.FindFlag("--module", cleanArgs)
	if err != nil {
		return
	}
	coreutils.RemoveFlagFromCommand(&cleanArgs, flagIndex, valueIndex)
	buildConfig = NewBuildConfiguration(buildName, buildNumber, module, project)
	err = buildConfig.ValidateBuildAndModuleParams()
	return
}
