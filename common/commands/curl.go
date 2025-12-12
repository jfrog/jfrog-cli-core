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
func (curlCmd *CurlCommand) findUriValueAndIndex() (int, string) {
	// curlBooleanFlags is a set of curl flags that do NOT take a value.
	curlBooleanFlags := map[string]struct{}{
		"-#": {}, "-:": {}, "-0": {}, "-1": {}, "-2": {}, "-3": {}, "-4": {}, "-6": {},
		"-a": {}, "-B": {}, "-f": {}, "-g": {}, "-G": {}, "-I": {}, "-i": {},
		"-j": {}, "-J": {}, "-k": {}, "-l": {}, "-L": {}, "-M": {}, "-n": {},
		"-N": {}, "-O": {}, "-p": {}, "-q": {}, "-R": {}, "-s": {}, "-S": {},
		"-v": {}, "-V": {}, "-Z": {},
		"--anyauth": {}, "--append": {}, "--basic": {}, "--ca-native": {},
		"--cert-status": {}, "--compressed": {}, "--compressed-ssh": {},
		"--create-dirs": {}, "--crlf": {}, "--digest": {}, "--disable": {},
		"--disable-eprt": {}, "--disable-epsv": {}, "--disallow-username-in-url": {},
		"--doh-cert-status": {}, "--doh-insecure": {}, "--fail": {},
		"--fail-early": {}, "--fail-with-body": {}, "--false-start": {},
		"--form-escape": {}, "--ftp-create-dirs": {}, "--ftp-pasv": {},
		"--ftp-pret": {}, "--ftp-skip-pasv-ip": {}, "--ftp-ssl-ccc": {},
		"--ftp-ssl-control": {}, "--get": {}, "--globoff": {},
		"--haproxy-protocol": {}, "--head": {}, "--http0.9": {}, "--http1.0": {},
		"--http1.1": {}, "--http2": {}, "--http2-prior-knowledge": {},
		"--http3": {}, "--http3-only": {}, "--ignore-content-length": {},
		"--include": {}, "--insecure": {}, "--ipv4": {}, "--ipv6": {},
		"--junk-session-cookies": {}, "--list-only": {}, "--location": {},
		"--location-trusted": {}, "--mail-rcpt-allowfails": {}, "--manual": {},
		"--metalink": {}, "--negotiate": {}, "--netrc": {}, "--netrc-optional": {},
		"--next": {}, "--no-alpn": {}, "--no-buffer": {}, "--no-clobber": {},
		"--no-keepalive": {}, "--no-npn": {}, "--no-progress-meter": {},
		"--no-sessionid": {}, "--ntlm": {}, "--ntlm-wb": {}, "--parallel": {},
		"--parallel-immediate": {}, "--path-as-is": {}, "--post301": {},
		"--post302": {}, "--post303": {}, "--progress-bar": {},
		"--proxy-anyauth": {}, "--proxy-basic": {}, "--proxy-ca-native": {},
		"--proxy-digest": {}, "--proxy-http2": {}, "--proxy-insecure": {},
		"--proxy-negotiate": {}, "--proxy-ntlm": {}, "--proxy-ssl-allow-beast": {},
		"--proxy-ssl-auto-client-cert": {}, "--proxy-tlsv1": {}, "--proxytunnel": {},
		"--raw": {}, "--remote-header-name": {}, "--remote-name": {},
		"--remote-name-all": {}, "--remote-time": {}, "--remove-on-error": {},
		"--retry-all-errors": {}, "--retry-connrefused": {}, "--sasl-ir": {},
		"--show-error": {}, "--silent": {}, "--socks5-basic": {},
		"--socks5-gssapi": {}, "--socks5-gssapi-nec": {}, "--ssl": {},
		"--ssl-allow-beast": {}, "--ssl-auto-client-cert": {}, "--ssl-no-revoke": {},
		"--ssl-reqd": {}, "--ssl-revoke-best-effort": {}, "--sslv2": {},
		"--sslv3": {}, "--styled-output": {}, "--suppress-connect-headers": {},
		"--tcp-fastopen": {}, "--tcp-nodelay": {}, "--tftp-no-options": {},
		"--tlsv1": {}, "--tlsv1.0": {}, "--tlsv1.1": {}, "--tlsv1.2": {},
		"--tlsv1.3": {}, "--tr-encoding": {}, "--trace-ids": {},
		"--trace-time": {}, "--use-ascii": {}, "--verbose": {}, "--version": {},
		"--xattr": {},
	}

	// isBooleanFlag checks if a flag is in the boolean flags
	isBooleanFlag := func(flag string) bool {
		_, exists := curlBooleanFlags[flag]
		return exists
	}

	skipNextArg := false
	for index, arg := range curlCmd.arguments {
		// Check if this arg should be skipped (it's a value for the previous flag)
		if skipNextArg {
			skipNextArg = false
			continue
		}

		// Check if this is a flag
		if strings.HasPrefix(arg, "-") {
			// Check for flags with inline values like --header=value
			if strings.HasPrefix(arg, "--") && strings.Contains(arg, "=") {
				continue
			}

			// Check if it a boolean flag
			if isBooleanFlag(arg) {
				continue
			}

			// For short flags
			if !strings.HasPrefix(arg, "--") && len(arg) > 2 {
				// find a flag that takes a value, everything after it is the inline value
				for i := 1; i < len(arg); i++ {
					charFlag := "-" + string(arg[i])
					if !isBooleanFlag(charFlag) {
						// Found a flag that takes a value
						if i < len(arg)-1 {
							// Inline value exists (e.g., -XGET, -Lotest.txt)
							break
						}
						// No inline value (e.g., -Lo, -sX), next arg is the value
						skipNextArg = true
						break
					}
				}
				continue
			}

			// Flag takes a value in the next argument
			skipNextArg = true
			continue
		}

		// Found a non-flag argument - this is the URL/API path
		return index, arg
	}

	return -1, ""
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
