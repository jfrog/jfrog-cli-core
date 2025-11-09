package commands

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"

	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type CurlCommand struct {
	arguments      []string
	executablePath string
	serverDetails  *config.ServerDetails
	url            string
}

func NewCurlCommand() *CurlCommand {
	return &CurlCommand{}
}

func (curlCmd *CurlCommand) SetArguments(arguments []string) *CurlCommand {
	curlCmd.arguments = arguments
	return curlCmd
}

func (curlCmd *CurlCommand) SetExecutablePath(executablePath string) *CurlCommand {
	curlCmd.executablePath = executablePath
	return curlCmd
}

func (curlCmd *CurlCommand) SetServerDetails(serverDetails *config.ServerDetails) *CurlCommand {
	curlCmd.serverDetails = serverDetails
	return curlCmd
}

func (curlCmd *CurlCommand) SetUrl(url string) *CurlCommand {
	curlCmd.url = url
	return curlCmd
}

func (curlCmd *CurlCommand) Run() error {
	// Get curl execution path.
	execPath, err := exec.LookPath("curl")
	if err != nil {
		return errorutils.CheckError(err)
	}
	curlCmd.SetExecutablePath(execPath)

	// If the command already includes credentials flag, return an error.
	if curlCmd.isCredentialsFlagExists() {
		return errorutils.CheckErrorf("Curl command must not include credentials flag (-u or --user).")
	}

	// If the command already includes certificates flag, return an error.
	if curlCmd.serverDetails.ClientCertPath != "" && curlCmd.isCertificateFlagExists() {
		return errorutils.CheckErrorf("Curl command must not include certificate flag (--cert or --key).")
	}

	// Get target url for the curl command.
	uriIndex, targetUri, err := curlCmd.buildCommandUrl(curlCmd.url)
	if err != nil {
		return err
	}

	// Replace url argument with complete url.
	curlCmd.arguments[uriIndex] = targetUri

	cmdWithoutCreds := strings.Join(curlCmd.arguments, " ")
	// Add credentials to curl command.
	credentialsMessage := curlCmd.addCommandCredentials()

	// Run curl.
	log.Debug(fmt.Sprintf("Executing curl command: '%s %s'", cmdWithoutCreds, credentialsMessage))
	return gofrogcmd.RunCmd(curlCmd)
}

func (curlCmd *CurlCommand) addCommandCredentials() string {
	certificateHelpPrefix := ""

	if curlCmd.serverDetails.ClientCertPath != "" {
		curlCmd.arguments = append(curlCmd.arguments,
			"--cert", curlCmd.serverDetails.ClientCertPath,
			"--key", curlCmd.serverDetails.ClientCertKeyPath)
		certificateHelpPrefix = "--cert *** --key *** "
	}

	if curlCmd.serverDetails.AccessToken != "" {
		// Add access token header.
		tokenHeader := fmt.Sprintf("Authorization: Bearer %s", curlCmd.serverDetails.AccessToken)
		curlCmd.arguments = append(curlCmd.arguments, "-H", tokenHeader)

		return certificateHelpPrefix + "-H \"Authorization: Bearer ***\""
	}

	// Add credentials flag to Command. In case of flag duplication, the latter is used by Curl.
	credFlag := fmt.Sprintf("-u%s:%s", curlCmd.serverDetails.User, curlCmd.serverDetails.Password)
	curlCmd.arguments = append(curlCmd.arguments, credFlag)

	return certificateHelpPrefix + "-u***:***"
}

func (curlCmd *CurlCommand) buildCommandUrl(url string) (uriIndex int, uriValue string, err error) {
	// Find command's URL argument.
	// Representing the target API for the Curl command.
	uriIndex, uriValue = curlCmd.findUriValueAndIndex()
	if uriIndex == -1 {
		err = errorutils.CheckErrorf("Could not find argument in curl command.")
		return
	}

	// If user provided full-url, throw an error.
	if strings.HasPrefix(uriValue, "http://") || strings.HasPrefix(uriValue, "https://") {
		err = errorutils.CheckErrorf("Curl command must not include full-url, but only the REST API URI (e.g '/api/system/ping').")
		return
	}

	// Trim '/' prefix if exists.
	uriValue = strings.TrimPrefix(uriValue, "/")

	// Attach url to the api.
	uriValue = url + uriValue

	return
}

// Returns server details
func (curlCmd *CurlCommand) GetServerDetails() (*config.ServerDetails, error) {
	// Get --server-id flag value from the command, and remove it.
	flagIndex, valueIndex, serverIdValue, err := coreutils.FindFlag("--server-id", curlCmd.arguments)
	if err != nil {
		return nil, err
	}
	coreutils.RemoveFlagFromCommand(&curlCmd.arguments, flagIndex, valueIndex)
	return config.GetSpecificConfig(serverIdValue, true, true)
}

// Find the URL argument in the Curl Command.
// A command flag is prefixed by '-' or '--'.
// Use this method ONLY after removing all JFrog-CLI flags, i.e. flags in the form: '--my-flag=value' are not allowed.
// An argument is any provided candidate which is not a flag or a flag value.
// The API path is the argument that doesn't start with '-' and isn't a known non-path value.
func (curlCmd *CurlCommand) findUriValueAndIndex() (int, string) {
	// Look for the last non-flag argument, as the URL/path is typically at the end
	lastNonFlagIndex := -1
	lastNonFlagValue := ""

	for index, arg := range curlCmd.arguments {
		// Skip flags (start with -)
		if strings.HasPrefix(arg, "-") {
			continue
		}

		// This is a potential API path
		lastNonFlagIndex = index
		lastNonFlagValue = arg
	}

	// If we found a non-flag argument, check if it might be a flag value
	if lastNonFlagIndex > 0 {
		prevArg := curlCmd.arguments[lastNonFlagIndex-1]

		// Check if the previous argument is a flag that typically takes file/string values
		// Only check the most common ones that could be confused with API paths
		if prevArg == "-o" || prevArg == "--output" || // output file
			prevArg == "-T" || prevArg == "--upload-file" || // upload file
			prevArg == "-d" || prevArg == "--data" || // data
			prevArg == "-u" || prevArg == "--user" { // credentials
			// This might be a flag value, try to find another candidate
			for i := lastNonFlagIndex - 1; i >= 0; i-- {
				if !strings.HasPrefix(curlCmd.arguments[i], "-") &&
					(i == 0 || curlCmd.arguments[i-1] == "-X" ||
						curlCmd.arguments[i-1] == "--request" ||
						!isPotentialValueFlag(curlCmd.arguments[i-1])) {
					return i, curlCmd.arguments[i]
				}
			}
		}
	}

	return lastNonFlagIndex, lastNonFlagValue
}

// Helper to check if a flag commonly takes values that could look like paths
func isPotentialValueFlag(flag string) bool {
	return flag == "-o" || flag == "--output" ||
		flag == "-T" || flag == "--upload-file" ||
		flag == "-d" || flag == "--data" ||
		flag == "-u" || flag == "--user"
}

// Return true if the curl command includes credentials flag.
// The searched flags are not CLI flags.
func (curlCmd *CurlCommand) isCredentialsFlagExists() bool {
	for _, arg := range curlCmd.arguments {
		if strings.HasPrefix(arg, "-u") || arg == "--user" {
			return true
		}
	}

	return false
}

// Return true if the curl command includes certificates flag.
// The searched flags are not CLI flags.
func (curlCmd *CurlCommand) isCertificateFlagExists() bool {
	for _, arg := range curlCmd.arguments {
		if arg == "--cert" || arg == "--key" {
			return true
		}
	}

	return false
}

func (curlCmd *CurlCommand) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, curlCmd.executablePath)
	cmd = append(cmd, curlCmd.arguments...)
	return exec.Command(cmd[0], cmd[1:]...)
}

func (curlCmd *CurlCommand) GetEnv() map[string]string {
	return map[string]string{}
}

func (curlCmd *CurlCommand) GetStdWriter() io.WriteCloser {
	return nil
}

func (curlCmd *CurlCommand) GetErrWriter() io.WriteCloser {
	return nil
}

func (curlCmd *CurlCommand) ServerDetails() (*config.ServerDetails, error) {
	return curlCmd.serverDetails, nil
}
