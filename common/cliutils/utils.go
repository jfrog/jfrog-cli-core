package cliutils

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

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

func LogNonNativeCommandDeprecation(cmdName, oldSubcommand string) {
	if ShouldLogWarning() {
		log.Warn(
			`You are using a deprecated syntax of the command.
	Instead of:
	$ ` + coreutils.GetCliExecutableName() + ` ` + oldSubcommand + ` ` + cmdName + ` ...
	Use:
	$ ` + coreutils.GetCliExecutableName() + ` ` + cmdName + ` ...`)
	}
}

func LogNonGenericAuditCommandDeprecation(cmdName string) {
	if ShouldLogWarning() {
		log.Warn(
			`You are using a deprecated syntax of the command.
	Instead of:
	$ ` + coreutils.GetCliExecutableName() + ` ` + cmdName + ` ...
	Use:
	$ ` + coreutils.GetCliExecutableName() + ` audit ...`)
	}
}

func ShouldLogWarning() bool {
	return strings.ToLower(os.Getenv(JfrogCliAvoidDeprecationWarnings)) != "true"
}

func GetCLIDocumentationMessage() string {
	return "You can read the documentation at " + coreutils.JFrogHelpUrl + "jfrog-cli"
}

func PrintHelpAndReturnError(msg string, printHelp func() error) error {
	log.Error(msg + " " + GetCLIDocumentationMessage())
	err := printHelp()
	if err != nil {
		msg = msg + ". " + err.Error()
	}
	return errors.New(msg)
}

// This function checks whether the command received --help as a single option.
// This function should be used iff the SkipFlagParsing option is used.
// Generic commands such as docker, don't have dedicated subcommands. As a workaround, printing the help of their subcommands,
// we use a dummy command with no logic but the help message. to trigger the print of those dummy commands,
// each generic command must decide what cmdName it needs to pass to this function.
// For example, 'jf docker scan --help' passes cmdName='dockerscanhelp' to print our help and not the origin from docker client/cli.
func ShowGenericCmdHelpIfNeeded(args []string, printHelp func() error) (bool, error) {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			err := printHelp()
			return true, err
		}
	}
	return false, nil
}

// This function checks whether the command received --help as a single option.
// If it did, the command's help is shown and true is returned.
// This function should be used iff the SkipFlagParsing option is used.
func ShowCmdHelpIfNeeded(args []string, printHelp func() error) (bool, error) {
	if len(args) != 1 {
		return false, nil
	}
	if args[0] == "--help" || args[0] == "-h" {
		err := printHelp()
		return true, err
	}
	return false, nil
}

func WrongNumberOfArgumentsHandler(argCount int, printHelp func() error) error {
	return PrintHelpAndReturnError(fmt.Sprintf("Wrong number of arguments (%d).", argCount), printHelp)
}

func GetThreadsCount(threadCountStrVal string) (threads int, err error) {
	threads = Threads
	if threadCountStrVal != "" {
		threads, err = strconv.Atoi(threadCountStrVal)
		if err != nil || threads < 1 {
			err = errors.New("the '--threads' option should have a numeric positive value")
			return 0, err
		}
	}
	return threads, nil
}

// Get a secret value from a flag or from stdin.
func HandleSecretInput(stringFlag, secretRaw, stdinFlag string, isStdin bool) (secret string, err error) {
	secret = secretRaw
	isStdinSecret := isStdin
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

func OfferConfig(createServerDetails func() (*config.ServerDetails, error)) (*config.ServerDetails, error) {
	confirmed, err := ShouldOfferConfig()
	if !confirmed || err != nil {
		return nil, err
	}
	details, err := createServerDetails()
	if err != nil {
		return nil, err
	}
	configCmd := commands.NewConfigCommand(commands.AddOrEdit, details.ServerId).SetDefaultDetails(details).SetInteractive(true).SetEncPassword(true)
	err = configCmd.Run()
	if err != nil {
		return nil, err
	}

	return configCmd.ServerDetails()
}

func ShouldOfferConfig() (bool, error) {
	exists, err := config.IsServerConfExists()
	if err != nil || exists {
		return false, err
	}
	var ci bool
	if ci, err = clientUtils.GetBoolEnvValue(coreutils.CI, false); err != nil {
		return false, err
	}
	if ci {
		return false, nil
	}

	msg := fmt.Sprintf("To avoid this message in the future, set the %s environment variable to true.\n"+
		"The CLI commands require the URL and authentication details\n"+
		"Configuring JFrog CLI with these parameters now will save you having to include them as command options.\n"+
		"You can also configure these parameters later using the 'jfrog c' command.\n"+
		"Configure now?", coreutils.CI)
	confirmed := coreutils.AskYesNo(msg, false)
	if !confirmed {
		return false, nil
	}
	return true, nil
}

// Exclude refreshable tokens parameter should be true when working with external tools (build tools, curl, etc)
// or when sending requests not via ArtifactoryHttpClient.
func CreateServerDetailsWithConfigOffer(createServerDetails func() (*config.ServerDetails, error), excludeRefreshableTokens bool) (*config.ServerDetails, error) {
	createdDetails, err := OfferConfig(createServerDetails)
	if err != nil {
		return nil, err
	}
	if createdDetails != nil {
		return createdDetails, err
	}

	details, err := createServerDetails()
	if err != nil {
		return nil, err
	}
	// If urls or credentials were passed as options, use options as they are.
	// For security reasons, we'd like to avoid using part of the connection details from command options and the rest from the config.
	// Either use command options only or config only.
	if credentialsChanged(details) {
		return details, nil
	}

	// Else, use details from config for requested serverId, or for default server if empty.
	confDetails, err := commands.GetConfig(details.ServerId, excludeRefreshableTokens)
	if err != nil {
		return nil, err
	}

	// Take insecureTls value from options since it is not saved in config.
	confDetails.InsecureTls = details.InsecureTls
	confDetails.Url = clientUtils.AddTrailingSlashIfNeeded(confDetails.Url)
	confDetails.DistributionUrl = clientUtils.AddTrailingSlashIfNeeded(confDetails.DistributionUrl)

	// Create initial access token if needed.
	if !excludeRefreshableTokens {
		err = config.CreateInitialRefreshableTokensIfNeeded(confDetails)
		if err != nil {
			return nil, err
		}
	}

	return confDetails, nil
}

func credentialsChanged(details *config.ServerDetails) bool {
	return details.Url != "" || details.ArtifactoryUrl != "" || details.DistributionUrl != "" || details.XrayUrl != "" ||
		details.User != "" || details.Password != "" || details.SshKeyPath != "" || details.SshPassphrase != "" || details.AccessToken != "" ||
		details.ClientCertKeyPath != "" || details.ClientCertPath != ""
}

func FixWinPathsForFileSystemSourcedCmds(uploadSpec *spec.SpecFiles, specFlag, exclusionsFlag bool) {
	if coreutils.IsWindows() {
		for i, file := range uploadSpec.Files {
			uploadSpec.Files[i].Pattern = FixWinPathBySource(file.Pattern, specFlag)
			for j, exclusion := range uploadSpec.Files[i].Exclusions {
				// If exclusions are set, they override the spec value
				uploadSpec.Files[i].Exclusions[j] = FixWinPathBySource(exclusion, specFlag && !exclusionsFlag)
			}
		}
	}
}
