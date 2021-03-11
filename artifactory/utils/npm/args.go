package npm

import (
	"github.com/jfrog/jfrog-cli-core/artifactory/utils"
	commonutils "github.com/jfrog/jfrog-cli-core/common/utils"
	"strconv"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

func ExtractNpmOptionsFromArgs(args []string) (threads int, jsonOutput bool, cleanArgs []string, buildConfig *utils.BuildConfiguration, err error) {
	threads = 3
	// Extract threads information from the args.
	flagIndex, valueIndex, numOfThreads, err := commonutils.FindFlag("--threads", args)
	if err != nil {
		return
	}
	commonutils.RemoveFlagFromCommand(&args, flagIndex, valueIndex)
	if numOfThreads != "" {
		threads, err = strconv.Atoi(numOfThreads)
		if err != nil {
			err = errorutils.CheckError(err)
			return
		}
	}

	// Since we use --json flag for retrieving the npm config for writing the temp .npmrc, json=true is written to the config list.
	// We don't want to force the json output for all users, so we check whether the json output was explicitly required.
	flagIndex, jsonOutput, err = commonutils.FindBooleanFlag("--json", args)
	if err != nil {
		return
	}
	// Since boolean flag might appear as --flag or --flag=value, the value index is the same as the flag index.
	commonutils.RemoveFlagFromCommand(&args, flagIndex, flagIndex)

	cleanArgs, buildConfig, err = commonutils.ExtractBuildDetailsFromArgs(args)
	return
}
