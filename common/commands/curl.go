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

	// Build the full URL from the API path (first non-flag argument).
	// Remove the API path and append the full URL at the end.
	if err := curlCmd.buildAndAppendUrl(); err != nil {
		return err
	}

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

// buildAndAppendUrl finds the first non-flag argument (the API path), removes it,
// builds the full URL, and appends it at the end. This allows curl flags to appear in any order.
func (curlCmd *CurlCommand) buildAndAppendUrl() error {
	// Common curl flags that take a value in the next argument
	flagsWithValues := map[string]bool{
		"-X": true, "-H": true, "-d": true, "-o": true, "-A": true, "-e": true,
		"-T": true, "-b": true, "-c": true, "-F": true, "-m": true, "-w": true,
		"-x": true, "-y": true, "-z": true, "-C": true, "-K": true, "-E": true,
		"--request": true, "--header": true, "--data": true, "--output": true,
		"--user-agent": true, "--referer": true, "--upload-file": true,
		"--cookie": true, "--cookie-jar": true, "--form": true, "--max-time": true,
		"--write-out": true, "--proxy": true, "--cert": true, "--key": true,
		"--cacert": true, "--capath": true, "--connect-timeout": true,
		"--retry": true, "--retry-delay": true, "--retry-max-time": true,
		"--speed-limit": true, "--speed-time": true, "--limit-rate": true,
		"--max-filesize": true, "--max-redirs": true, "--data-binary": true,
		"--data-urlencode": true, "--data-raw": true, "--data-ascii": true,
	}

	// Find the first non-flag argument (the API path)
	// Skip arguments that are values for flags
	apiPathIndex := -1
	skipNext := false

	for i, arg := range curlCmd.arguments {
		// Skip if this is a flag value
		if skipNext {
			skipNext = false
			continue
		}

		// Check if this is a flag
		if strings.HasPrefix(arg, "-") {
			// Check if it's a flag that takes a value (and value is not inline)
			if flagsWithValues[arg] {
				skipNext = true
			}
			// Check for long flags with inline values like --header=value
			if strings.Contains(arg, "=") {
				skipNext = false
			}
			// For short flags, check if value is inline like -XGET
			if len(arg) > 2 && !strings.HasPrefix(arg, "--") {
				skipNext = false
			}
			continue
		}

		// Found a non-flag argument that's not a flag value - this is the API path
		apiPathIndex = i
		break
	}

	if apiPathIndex == -1 {
		return errorutils.CheckErrorf("Could not find API path argument in curl command.")
	}

	apiPath := curlCmd.arguments[apiPathIndex]

	// If user provided full-url, throw an error.
	if strings.HasPrefix(apiPath, "http://") || strings.HasPrefix(apiPath, "https://") {
		return errorutils.CheckErrorf("Curl command must not include full-url, but only the REST API URI (e.g '/api/system/ping').")
	}

	// Remove the API path from its current position
	curlCmd.arguments = append(curlCmd.arguments[:apiPathIndex], curlCmd.arguments[apiPathIndex+1:]...)

	// Trim '/' prefix if exists.
	apiPath = strings.TrimPrefix(apiPath, "/")

	// Build full URL and append at the end
	fullUrl := curlCmd.url + apiPath
	curlCmd.arguments = append(curlCmd.arguments, fullUrl)

	return nil
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
