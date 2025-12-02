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

// curlBooleanFlags contains curl flags that do NOT take a value.
var curlBooleanFlags = map[string]bool{
	"-#": true, "-0": true, "-1": true, "-2": true, "-3": true, "-4": true, "-6": true,
	"-a": true, "-B": true, "-f": true, "-g": true, "-G": true, "-I": true, "-i": true,
	"-j": true, "-J": true, "-k": true, "-l": true, "-L": true, "-M": true, "-n": true,
	"-N": true, "-O": true, "-p": true, "-q": true, "-R": true, "-s": true, "-S": true,
	"-v": true, "-V": true, "-Z": true,
	"--anyauth": true, "--append": true, "--basic": true, "--ca-native": true,
	"--cert-status": true, "--compressed": true, "--compressed-ssh": true,
	"--create-dirs": true, "--crlf": true, "--digest": true, "--disable": true,
	"--disable-eprt": true, "--disable-epsv": true, "--disallow-username-in-url": true,
	"--doh-cert-status": true, "--doh-insecure": true, "--fail": true,
	"--fail-early": true, "--fail-with-body": true, "--false-start": true,
	"--form-escape": true, "--ftp-create-dirs": true, "--ftp-pasv": true,
	"--ftp-pret": true, "--ftp-skip-pasv-ip": true, "--ftp-ssl-ccc": true,
	"--ftp-ssl-control": true, "--get": true, "--globoff": true,
	"--haproxy-protocol": true, "--head": true, "--http0.9": true, "--http1.0": true,
	"--http1.1": true, "--http2": true, "--http2-prior-knowledge": true,
	"--http3": true, "--http3-only": true, "--ignore-content-length": true,
	"--include": true, "--insecure": true, "--ipv4": true, "--ipv6": true,
	"--junk-session-cookies": true, "--list-only": true, "--location": true,
	"--location-trusted": true, "--mail-rcpt-allowfails": true, "--manual": true,
	"--metalink": true, "--negotiate": true, "--netrc": true, "--netrc-optional": true,
	"--next": true, "--no-alpn": true, "--no-buffer": true, "--no-clobber": true,
	"--no-keepalive": true, "--no-npn": true, "--no-progress-meter": true,
	"--no-sessionid": true, "--ntlm": true, "--ntlm-wb": true, "--parallel": true,
	"--parallel-immediate": true, "--path-as-is": true, "--post301": true,
	"--post302": true, "--post303": true, "--progress-bar": true,
	"--proxy-anyauth": true, "--proxy-basic": true, "--proxy-ca-native": true,
	"--proxy-digest": true, "--proxy-http2": true, "--proxy-insecure": true,
	"--proxy-negotiate": true, "--proxy-ntlm": true, "--proxy-ssl-allow-beast": true,
	"--proxy-ssl-auto-client-cert": true, "--proxy-tlsv1": true, "--proxytunnel": true,
	"--raw": true, "--remote-header-name": true, "--remote-name": true,
	"--remote-name-all": true, "--remote-time": true, "--remove-on-error": true,
	"--retry-all-errors": true, "--retry-connrefused": true, "--sasl-ir": true,
	"--show-error": true, "--silent": true, "--socks5-basic": true,
	"--socks5-gssapi": true, "--socks5-gssapi-nec": true, "--ssl": true,
	"--ssl-allow-beast": true, "--ssl-auto-client-cert": true, "--ssl-no-revoke": true,
	"--ssl-reqd": true, "--ssl-revoke-best-effort": true, "--sslv2": true,
	"--sslv3": true, "--styled-output": true, "--suppress-connect-headers": true,
	"--tcp-fastopen": true, "--tcp-nodelay": true, "--tftp-no-options": true,
	"--tlsv1": true, "--tlsv1.0": true, "--tlsv1.1": true, "--tlsv1.2": true,
	"--tlsv1.3": true, "--tr-encoding": true, "--trace-ids": true,
	"--trace-time": true, "--use-ascii": true, "--verbose": true, "--version": true,
	"--xattr": true,
}

// Find the URL argument in the Curl Command.
// A command flag is prefixed by '-' or '--'.
// Use this method ONLY after removing all JFrog-CLI flags, i.e. flags in the form: '--my-flag=value' are not allowed.
// An argument is any provided candidate which is not a flag or a flag value.
func (curlCmd *CurlCommand) findUriValueAndIndex() (int, string) {
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
			if curlBooleanFlags[arg] {
				continue
			}

			// For short flags
			if !strings.HasPrefix(arg, "--") && len(arg) > 2 {
				// find a flag that takes a value, everything after it is the inline value
				for i := 1; i < len(arg); i++ {
					charFlag := "-" + string(arg[i])
					if !curlBooleanFlags[charFlag] {
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
