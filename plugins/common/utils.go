package common

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/common/cliutils"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
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
	secret = c.GetStringFlagValue(stringFlag)
	isStdinSecret := c.GetBoolFlagValue(stdinFlag)
	if isStdinSecret && secret != "" {
		err = errorutils.CheckErrorf("providing both %s and %s flags is not supported", stringFlag, stdinFlag)
		return
	}

	if isStdinSecret {
		var stat os.FileInfo
		stat, err = os.Stdin.Stat()
		if err != nil {
			return
		}
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			var rawSecret []byte
			rawSecret, err = io.ReadAll(os.Stdin)
			if err != nil {
				return
			}
			secret = strings.TrimSpace(string(rawSecret))
			if secret != "" {
				log.Debug("Using", stringFlag, "provided via Stdin")
				return
			}
		}
		err = errorutils.CheckErrorf("no %s provided via Stdin", stringFlag)
	}
	return
}

func FixWinPathBySource(path string, fromSpec bool) string {
	if strings.Count(path, "/") > 0 {
		// Assuming forward slashes - not doubling backslash to allow regexp escaping
		return ioutils.UnixToWinPathSeparator(path)
	}
	if fromSpec {
		// Doubling backslash only for paths from spec files (that aren't forward slashed)
		return ioutils.DoubleWinPathSeparator(path)
	}
	return path
}

func RunCmdWithDeprecationWarning(cmdName, oldSubcommand string, c *components.Context,
	cmd func(c *components.Context) error) error {
	LogNonNativeCommandDeprecation(cmdName, oldSubcommand)
	return cmd(c)
}

func LogNonNativeCommandDeprecation(cmdName, oldSubcommand string) {
	if shouldLogWarning() {
		log.Warn(
			`You are using a deprecated syntax of the command.
	Instead of:
	$ ` + coreutils.GetCliExecutableName() + ` ` + oldSubcommand + ` ` + cmdName + ` ...
	Use:
	$ ` + coreutils.GetCliExecutableName() + ` ` + cmdName + ` ...`)
	}
}

func LogNonGenericAuditCommandDeprecation(cmdName string) {
	if shouldLogWarning() {
		log.Warn(
			`You are using a deprecated syntax of the command.
	Instead of:
	$ ` + coreutils.GetCliExecutableName() + ` ` + cmdName + ` ...
	Use:
	$ ` + coreutils.GetCliExecutableName() + ` audit ...`)
	}
}

func shouldLogWarning() bool {
	return strings.ToLower(os.Getenv(cliutils.JfrogCliAvoidDeprecationWarnings)) != "true"
}

func GetCLIDocumentationMessage() string {
	return "You can read the documentation at " + coreutils.JFrogHelpUrl + "jfrog-cli"
}

func GetThreadsCount(c *components.Context) (threads int, err error) {
	threads = cliutils.Threads
	if c.GetStringFlagValue("threads") != "" {
		threads, err = strconv.Atoi(c.GetStringFlagValue("threads"))
		if err != nil || threads < 1 {
			err = errors.New("the '--threads' option should have a numeric positive value")
			return 0, err
		}
	}
	return threads, nil
}

// This function checks whether the command received --help as a single option.
// If it did, the command's help is shown and true is returned.
// This function should be used iff the SkipFlagParsing option is used.
func ShowCmdHelpIfNeeded(c *components.Context, args []string) (bool, error) {
	if len(args) != 1 {
		return false, nil
	}
	if args[0] == "--help" || args[0] == "-h" {
		err := c.PrintCommandHelp(c.CommandName)
		return true, err
	}
	return false, nil
}

func PrintHelpAndReturnError(msg string, context *components.Context) error {
	log.Error(msg + " " + GetCLIDocumentationMessage())
	err := context.PrintCommandHelp(context.CommandName)
	if err != nil {
		msg = msg + ". " + err.Error()
	}
	return errors.New(msg)
}

func WrongNumberOfArgumentsHandler(context *components.Context) error {
	return PrintHelpAndReturnError(fmt.Sprintf("Wrong number of arguments (%d).", len(context.Arguments)), context)
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
