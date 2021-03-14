package utils

import (
	commonutils "github.com/jfrog/jfrog-cli-core/common/utils"
	"strconv"
	"strings"

	"github.com/mattn/go-shellwords"
)

// Boolean flag can be provided in one of the following forms:
// 1. --flag=value, where value can be true/false
// 2. --flag, here the value is true
// Return values:
// flagIndex - index of flagName in args.
// flagValue - value of flagName.
// err - error if flag exists, but we failed to extract its value.
// If flag does not exist flagIndex = -1 with false value and nil error.
func FindBooleanFlag(flagName string, args []string) (flagIndex int, flagValue bool, err error) {
	var arg string
	for flagIndex, arg = range args {
		if strings.HasPrefix(arg, flagName) {
			value := strings.TrimPrefix(arg, flagName)
			if len(value) == 0 {
				flagValue = true
			} else if strings.HasPrefix(value, "=") {
				flagValue, err = strconv.ParseBool(value[1:])
			} else {
				continue
			}
			return
		}
	}
	return -1, false, nil
}

// Find the first match of any of the provided flags in args.
// Return same values as FindFlag.
func FindFlagFirstMatch(flags, args []string) (flagIndex, flagValueIndex int, flagValue string, err error) {
	// Look for provided flags.
	for _, flag := range flags {
		flagIndex, flagValueIndex, flagValue, err = commonutils.FindFlag(flag, args)
		if err != nil {
			return
		}
		if flagIndex != -1 {
			// Found value for flag.
			return
		}
	}
	return
}

func ExtractBuildDetailsFromArgs(args []string) (cleanArgs []string, buildConfig *BuildConfiguration, err error) {
	var flagIndex, valueIndex int
	buildConfig = &BuildConfiguration{}
	cleanArgs = append([]string(nil), args...)

	// Extract build-info information from the args.
	flagIndex, valueIndex, buildConfig.BuildName, err = commonutils.FindFlag("--build-name", cleanArgs)
	if err != nil {
		return
	}
	commonutils.RemoveFlagFromCommand(&cleanArgs, flagIndex, valueIndex)

	flagIndex, valueIndex, buildConfig.BuildNumber, err = commonutils.FindFlag("--build-number", cleanArgs)
	if err != nil {
		return
	}
	commonutils.RemoveFlagFromCommand(&cleanArgs, flagIndex, valueIndex)

	flagIndex, valueIndex, buildConfig.Project, err = commonutils.FindFlag("--project", cleanArgs)
	if err != nil {
		return
	}
	commonutils.RemoveFlagFromCommand(&cleanArgs, flagIndex, valueIndex)

	// Retrieve build name and build number from env if both missing
	buildConfig.BuildName, buildConfig.BuildNumber = GetBuildNameAndNumber(buildConfig.BuildName, buildConfig.BuildNumber)
	// Retrieve project from env if missing
	buildConfig.Project = GetBuildProject(buildConfig.Project)

	flagIndex, valueIndex, buildConfig.Module, err = commonutils.FindFlag("--module", cleanArgs)
	if err != nil {
		return
	}
	commonutils.RemoveFlagFromCommand(&cleanArgs, flagIndex, valueIndex)
	err = ValidateBuildAndModuleParams(buildConfig)
	return
}

func ExtractInsecureTlsFromArgs(args []string) (cleanArgs []string, insecureTls bool, err error) {
	cleanArgs = append([]string(nil), args...)

	flagIndex, insecureTls, err := FindBooleanFlag("--insecure-tls", args)
	if err != nil {
		return
	}
	commonutils.RemoveFlagFromCommand(&cleanArgs, flagIndex, flagIndex)
	return
}

// Iterate over each argument, if env variable is found (e.g $HOME) replace it with env value.
func ParseArgs(args []string) ([]string, error) {
	// Escape backslash & space
	for i := 0; i < len(args); i++ {
		args[i] = strings.ReplaceAll(args[i], `\`, `\\`)
		if strings.Index(args[i], ` `) != -1 && !isQuote(args[i]) {
			args[i] = strings.ReplaceAll(args[i], `"`, ``)
			args[i] = strings.ReplaceAll(args[i], `'`, ``)
			args[i] = `"` + args[i] + `"`
		}
	}
	parser := shellwords.NewParser()
	parser.ParseEnv = true
	return parser.Parse(strings.Join(args, " "))
}

func isQuote(s string) bool {
	return len(s) > 0 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\''))
}
