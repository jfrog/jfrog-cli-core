package common

import (
	"sort"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/common/cliutils"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/slices"
)

func GetStringsArrFlagValue(c *components.Context, flagName string) (resultArray []string) {
	if c.IsFlagSet(flagName) {
		resultArray = append(resultArray, strings.Split(c.GetStringFlagValue(flagName), ";")...)
	}
	return
}

// If `fieldName` exist in the cli args, read it to `field` as an array split by `;`.
func OverrideArrayIfSet(field *[]string, c *components.Context, fieldName string) {
	if c.IsFlagSet(fieldName) {
		*field = append([]string{}, strings.Split(c.GetStringFlagValue(fieldName), ";")...)
	}
}

// If `fieldName` exist in the cli args, read it to `field` as a int.
func OverrideIntIfSet(field *int, c *components.Context, fieldName string) {
	if c.IsFlagSet(fieldName) {
		value, err := strconv.ParseInt(c.GetStringFlagValue(fieldName), 0, 64)
		if err != nil {
			return
		}
		*field = int(value)
	}
}

// If `fieldName` exist in the cli args, read it to `field` as a string.
func OverrideStringIfSet(field *string, c *components.Context, fieldName string) {
	if c.IsFlagSet(fieldName) {
		*field = c.GetStringFlagValue(fieldName)
	}
}

// Get a secret value from a flag or from stdin.
func HandleSecretInput(c *components.Context, stringFlag, stdinFlag string) (secret string, err error) {
	return cliutils.HandleSecretInput(stringFlag, c.GetStringFlagValue(stringFlag), stdinFlag, c.GetBoolFlagValue(stdinFlag))
}

func RunCmdWithDeprecationWarning(cmdName, oldSubcommand string, c *components.Context,
	cmd func(c *components.Context) error) error {
	cliutils.LogNonNativeCommandDeprecation(cmdName, oldSubcommand)
	return cmd(c)
}

func GetThreadsCount(c *components.Context) (threads int, err error) {
	return cliutils.GetThreadsCount(c.GetStringFlagValue("threads"))
}

func GetPrintCurrentCmdHelp(c *components.Context) func() error {
	return func() error {
		return c.PrintCommandHelp(c.CommandName)
	}
}

// This function checks whether the command received --help as a single option.
// If it did, the command's help is shown and true is returned.
// This function should be used iff the SkipFlagParsing option is used.
func ShowCmdHelpIfNeeded(c *components.Context, args []string) (bool, error) {
	return cliutils.ShowCmdHelpIfNeeded(args, GetPrintCurrentCmdHelp(c))
}

func PrintHelpAndReturnError(msg string, context *components.Context) error {
	return cliutils.PrintHelpAndReturnError(msg, GetPrintCurrentCmdHelp(context))
}

func WrongNumberOfArgumentsHandler(context *components.Context) error {
	return cliutils.WrongNumberOfArgumentsHandler(len(context.Arguments), GetPrintCurrentCmdHelp(context))
}

func ExtractArguments(context *components.Context) []string {
	return slices.Clone(context.Arguments)
}

// Return a sorted list of a command's flags by a given command key.
func GetCommandFlags(cmdKey string, commandToFlags map[string][]string, flagsMap map[string]components.Flag) []components.Flag {
	flagList, ok := commandToFlags[cmdKey]
	if !ok {
		log.Error("The command \"", cmdKey, "\" is not found in commands flags map.")
		return nil
	}
	return buildAndSortFlags(flagList, flagsMap)
}

func buildAndSortFlags(keys []string, flagsMap map[string]components.Flag) (flags []components.Flag) {
	for _, flag := range keys {
		flags = append(flags, flagsMap[flag])
	}
	sort.Slice(flags, func(i, j int) bool { return flags[i].GetName() < flags[j].GetName() })
	return
}
