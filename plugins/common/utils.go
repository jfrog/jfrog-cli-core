package common

import (
	"errors"
	commandUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	artifactoryUtils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/commandsummary"
	buildUtils "github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/common/cliutils/summary"
	coreConfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/common/cliutils"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/slices"
)

type DetailedSummaryRecord struct {
	Source string `json:"source,omitempty"`
	Target string `json:"target"`
}

type ExtendedDetailedSummaryRecord struct {
	DetailedSummaryRecord
	Sha256 string `json:"sha256"`
}

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

// This function indicates whether the command should be executed without
// confirmation warning or not.
// If the --quiet option was sent, it is used to determine whether to prompt the confirmation or not.
// If not, the command will prompt the confirmation, unless the CI environment variable was set to true.
func GetQuietValue(c *components.Context) bool {
	if c.IsFlagSet("quiet") {
		return c.GetBoolFlagValue("quiet")
	}

	return getCiValue()
}

// Return true if the CI environment variable was set to true.
func getCiValue() bool {
	var ci bool
	var err error
	if ci, err = clientutils.GetBoolEnvValue(coreutils.CI, false); err != nil {
		return false
	}
	return ci
}

func CreateArtifactoryDetailsByFlags(c *components.Context) (*coreConfig.ServerDetails, error) {
	artDetails, err := CreateServerDetailsWithConfigOffer(c, false, cliutils.Rt)
	if err != nil {
		return nil, err
	}
	if artDetails.ArtifactoryUrl == "" {
		return nil, errors.New("no JFrog Artifactory URL specified, either via the --url flag or as part of the server configuration")
	}
	return artDetails, nil
}

// Returns build configuration struct using the options provided by the user.
// Any empty configuration could be later overridden by environment variables if set.
func CreateBuildConfigurationWithModule(c *components.Context) (buildConfigConfiguration *buildUtils.BuildConfiguration, err error) {
	buildConfigConfiguration = new(buildUtils.BuildConfiguration)
	err = buildConfigConfiguration.SetBuildName(c.GetStringFlagValue("build-name")).SetBuildNumber(c.GetStringFlagValue("build-number")).
		SetProject(c.GetStringFlagValue("project")).SetModule(c.GetStringFlagValue("module")).ValidateBuildAndModuleParams()
	return
}

func ExtractCommand(c *components.Context) []string {
	return slices.Clone(c.Arguments)
}

func IsFailNoOp(context *components.Context) bool {
	if isContextFailNoOp(context) {
		return true
	}
	return isEnvFailNoOp()
}

func isContextFailNoOp(context *components.Context) bool {
	if context == nil {
		return false
	}
	return context.GetBoolFlagValue("fail-no-op")
}

func isEnvFailNoOp() bool {
	return strings.ToLower(os.Getenv(coreutils.FailNoOp)) == "true"
}

// Get project key from flag or environment variable
func GetProject(c *components.Context) string {
	projectKey := c.GetStringFlagValue("project")
	return getOrDefaultEnv(projectKey, coreutils.Project)
}

// Return argument if not empty or retrieve from environment variable
func getOrDefaultEnv(arg, envKey string) string {
	if arg != "" {
		return arg
	}
	return os.Getenv(envKey)
}

func GetBuildName(buildName string) string {
	return getOrDefaultEnv(buildName, coreutils.BuildName)
}

func GetBuildUrl(buildUrl string) string {
	return getOrDefaultEnv(buildUrl, coreutils.BuildUrl)
}

func GetEnvExclude(envExclude string) string {
	return getOrDefaultEnv(envExclude, coreutils.EnvExclude)
}

func GetDocumentationMessage() string {
	return "You can read the documentation at " + coreutils.JFrogHelpUrl + "jfrog-cli"
}

func CleanupResult(result *commandUtils.Result, err *error) {
	if result != nil && result.Reader() != nil {
		*err = errors.Join(*err, result.Reader().Close())
	}
}

func PrintCommandSummary(result *commandUtils.Result, detailedSummary, printDeploymentView, failNoOp bool, originalErr error) (err error) {
	// We would like to print a basic summary of total failures/successes in the case of an error.
	err = originalErr
	if result == nil {
		// We don't have a total of failures/successes artifacts, so we are done.
		return
	}
	defer func() {
		err = GetCliError(err, result.SuccessCount(), result.FailCount(), failNoOp)
	}()
	basicSummary, err := CreateSummaryReportString(result.SuccessCount(), result.FailCount(), failNoOp, err)
	if err != nil {
		// Print the basic summary and return the original error.
		log.Output(basicSummary)
		return
	}
	if detailedSummary {
		err = PrintDetailedSummaryReport(basicSummary, result.Reader(), true, err)
	} else {
		if printDeploymentView {
			err = PrintDeploymentView(result.Reader())
		}
		log.Output(basicSummary)
	}
	return
}

func GetCliError(err error, success, failed int, failNoOp bool) error {
	switch coreutils.GetExitCode(err, success, failed, failNoOp) {
	case coreutils.ExitCodeError:
		{
			var errorMessage string
			if err != nil {
				errorMessage = err.Error()
			}
			return coreutils.CliError{ExitCode: coreutils.ExitCodeError, ErrorMsg: errorMessage}
		}
	case coreutils.ExitCodeFailNoOp:
		return coreutils.CliError{ExitCode: coreutils.ExitCodeFailNoOp, ErrorMsg: "No errors, but also no files affected (fail-no-op flag)."}
	default:
		return nil
	}
}

func CreateSummaryReportString(success, failed int, failNoOp bool, err error) (string, error) {
	summaryReport := summary.GetSummaryReport(success, failed, failNoOp, err)
	summaryContent, mErr := summaryReport.Marshal()
	if errorutils.CheckError(mErr) != nil {
		// Don't swallow the original error. Log the marshal error and return the original error.
		return "", summaryPrintError(mErr, err)
	}
	return clientutils.IndentJson(summaryContent), err
}

// Prints a summary report.
// If a resultReader is provided, we will iterate over the result and print a detailed summary including the affected files.
func PrintDetailedSummaryReport(basicSummary string, reader *content.ContentReader, uploaded bool, originalErr error) error {
	// A reader wasn't provided, prints the basic summary json and return.
	if reader == nil {
		log.Output(basicSummary)
		return nil
	}
	writer, mErr := content.NewContentWriter("files", false, true)
	if mErr != nil {
		log.Output(basicSummary)
		return summaryPrintError(mErr, originalErr)
	}
	// We remove the closing curly bracket in order to append the affected files array using a responseWriter to write directly to stdout.
	basicSummary = strings.TrimSuffix(basicSummary, "\n}") + ","
	log.Output(basicSummary)
	defer log.Output("}")
	readerLength, _ := reader.Length()
	// If the reader is empty we will print an empty array.
	if readerLength == 0 {
		log.Output("  \"files\": []")
	} else {
		for transferDetails := new(clientutils.FileTransferDetails); reader.NextRecord(transferDetails) == nil; transferDetails = new(clientutils.FileTransferDetails) {
			writer.Write(getDetailedSummaryRecord(transferDetails, uploaded))
		}
		reader.Reset()
	}
	mErr = writer.Close()
	if mErr != nil {
		return summaryPrintError(mErr, originalErr)
	}
	rErr := reader.GetError()
	if rErr != nil {
		return summaryPrintError(rErr, originalErr)
	}
	return summaryPrintError(reader.GetError(), originalErr)
}

// Print a file tree based on the items' path in the reader's list.
func PrintDeploymentView(reader *content.ContentReader) error {
	tree := artifactoryUtils.NewFileTree()
	for transferDetails := new(clientutils.FileTransferDetails); reader.NextRecord(transferDetails) == nil; transferDetails = new(clientutils.FileTransferDetails) {
		tree.AddFile(transferDetails.TargetPath, "")
	}
	if err := reader.GetError(); err != nil {
		return err
	}
	reader.Reset()
	output := tree.String()
	if len(output) > 0 {
		log.Info("These files were uploaded:\n\n" + output)
	}
	return nil
}

// Get the detailed summary record.
// For uploads, we need to print the sha256 of the uploaded file along with the source and target, and prefix the target with the Artifactory URL.
func getDetailedSummaryRecord(transferDetails *clientutils.FileTransferDetails, uploaded bool) interface{} {
	record := DetailedSummaryRecord{
		Source: transferDetails.SourcePath,
		Target: transferDetails.TargetPath,
	}
	if uploaded {
		record.Target = transferDetails.RtUrl + record.Target
		extendedRecord := ExtendedDetailedSummaryRecord{
			DetailedSummaryRecord: record,
			Sha256:                transferDetails.Sha256,
		}
		return extendedRecord
	}
	record.Source = transferDetails.RtUrl + record.Source
	return record
}

// Print summary report.
// a given non-nil error will pass through and be returned as is if no other errors are raised.
// In case of a nil error, the current function error will be returned.
func summaryPrintError(summaryError, originalError error) error {
	if originalError != nil {
		if summaryError != nil {
			log.Error(summaryError)
		}
		return originalError
	}
	return summaryError
}

func GetDetailedSummary(c *components.Context) bool {
	return c.GetBoolFlagValue("detailed-summary") || commandsummary.ShouldRecordSummary()
}

func PrintBriefSummaryReport(success, failed int, failNoOp bool, originalErr error) error {
	basicSummary, mErr := CreateSummaryReportString(success, failed, failNoOp, originalErr)
	if mErr == nil {
		log.Output(basicSummary)
	}
	return summaryPrintError(mErr, originalErr)
}
