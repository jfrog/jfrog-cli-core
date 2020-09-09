package coreutils

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/pkg/errors"
)

// Error modes (how should the application behave when the CheckError function is invoked):
type OnError string

var cliTempDir string

// Name of the user agent, i.e 'jfrog-cli-go/1.38.2'.
var cliUserAgent string

var version string

// Name of the agent, i.e 'jfrog-cli-go'.
var clientAgent string

func init() {
	// Initialize error handling.
	if os.Getenv(ErrorHandling) == string(OnErrorPanic) {
		errorutils.CheckError = PanicOnError
	}

	// Initialize the temp base-dir path of the CLI executions.
	cliTempDir = os.Getenv(TempDir)
	if cliTempDir == "" {
		cliTempDir = os.TempDir()
	}
	fileutils.SetTempDirBase(cliTempDir)
}

// Exit codes:
type ExitCode struct {
	Code int
}

var ExitCodeNoError = ExitCode{0}
var ExitCodeError = ExitCode{1}
var ExitCodeFailNoOp = ExitCode{2}
var ExitCodeVulnerableBuild = ExitCode{3}

type CliError struct {
	ExitCode
	ErrorMsg string
}

func (err CliError) Error() string {
	return err.ErrorMsg
}

func PanicOnError(err error) error {
	if err != nil {
		panic(err)
	}
	return err
}

func GetExitCode(err error, success, failed int, failNoOp bool) ExitCode {
	// Error occurred - Return 1
	if err != nil || failed > 0 {
		return ExitCodeError
	}
	// No errors, but also no files affected - Return 2 if failNoOp
	if success == 0 && failNoOp {
		return ExitCodeFailNoOp
	}
	// Otherwise - Return 0
	return ExitCodeNoError
}

func GetConfigVersion() string {
	return "3"
}

func SumTrueValues(boolArr []bool) int {
	counter := 0
	for _, val := range boolArr {
		counter += utils.Bool2Int(val)
	}
	return counter
}

func SpecVarsStringToMap(rawVars string) map[string]string {
	if len(rawVars) == 0 {
		return nil
	}
	varCandidates := strings.Split(rawVars, ";")
	varsList := []string{}
	for _, v := range varCandidates {
		if len(varsList) > 0 && isEndsWithEscapeChar(varsList[len(varsList)-1]) {
			currentLastVar := varsList[len(varsList)-1]
			varsList[len(varsList)-1] = strings.TrimSuffix(currentLastVar, "\\") + ";" + v
			continue
		}
		varsList = append(varsList, v)
	}
	return varsAsMap(varsList)
}

func isEndsWithEscapeChar(lastVar string) bool {
	return strings.HasSuffix(lastVar, "\\")
}

func varsAsMap(vars []string) map[string]string {
	result := map[string]string{}
	for _, v := range vars {
		keyVal := strings.SplitN(v, "=", 2)
		if len(keyVal) != 2 {
			continue
		}
		result[keyVal[0]] = keyVal[1]
	}
	return result
}

func IsWindows() bool {
	return runtime.GOOS == "windows"
}

func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// Return the path of CLI temp dir.
// This path should be persistent, meaning - should not be cleared at the end of a CLI run.
func GetCliPersistentTempDirPath() string {
	return cliTempDir
}

func GetUserAgent() string {
	return cliUserAgent
}

type Credentials interface {
	SetUser(string)
	SetPassword(string)
	GetUser() string
	GetPassword() string
}

func ReplaceVars(content []byte, specVars map[string]string) []byte {
	log.Debug("Replacing variables in the provided content: \n" + string(content))
	for key, val := range specVars {
		key = "${" + key + "}"
		log.Debug(fmt.Sprintf("Replacing '%s' with '%s'", key, val))
		content = bytes.Replace(content, []byte(key), []byte(val), -1)
	}
	log.Debug("The reformatted content is: \n" + string(content))
	return content
}

func GetJfrogHomeDir() (string, error) {
	// The JfrogHomeEnv environment variable has been deprecated and replaced with HomeDir
	if os.Getenv(HomeDir) != "" {
		return os.Getenv(HomeDir), nil
	} else if os.Getenv(JfrogHomeEnv) != "" {
		return path.Join(os.Getenv(JfrogHomeEnv), ".jfrog"), nil
	}

	userHomeDir := fileutils.GetHomeDir()
	if userHomeDir == "" {
		err := errorutils.CheckError(errors.New("couldn't find home directory. Make sure your HOME environment variable is set"))
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(userHomeDir, ".jfrog"), nil
}

func CreateDirInJfrogHome(dirName string) (string, error) {
	homeDir, err := GetJfrogHomeDir()
	if err != nil {
		return "", err
	}
	folderName := filepath.Join(homeDir, dirName)
	err = fileutils.CreateDirIfNotExist(folderName)
	return folderName, err
}

func GetJfrogSecurityDir() (string, error) {
	homeDir, err := GetJfrogHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, JfrogSecurityDirName), nil
}

func GetJfrogCertsDir() (string, error) {
	securityDir, err := GetJfrogSecurityDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(securityDir, JfrogCertsDirName), nil
}

func GetJfrogSecurityConfFilePath() (string, error) {
	securityDir, err := GetJfrogSecurityDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(securityDir, JfrogSecurityConfFile), nil
}

func GetJfrogBackupDir() (string, error) {
	homeDir, err := GetJfrogHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, JfrogBackupDirName), nil
}

// Ask a yes or no question, with a default answer.
func AskYesNo(promptPrefix string, defaultValue bool) bool {
	defStr := "[n]"
	if defaultValue {
		defStr = "[y]"
	}
	promptPrefix += " (y/n) " + defStr + "? "
	var answer string
	for {
		fmt.Print(promptPrefix)
		_, _ = fmt.Scanln(&answer)
		parsed, valid := parseYesNo(answer, defaultValue)
		if valid {
			return parsed
		}
		fmt.Println("Please enter a valid option.")
	}
}

func parseYesNo(s string, def bool) (ans, valid bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return def, true
	}
	matchedYes, err := regexp.MatchString("^yes$|^y$", strings.ToLower(s))
	if errorutils.CheckError(err) != nil {
		log.Error(err)
		return matchedYes, false
	}
	if matchedYes {
		return true, true
	}

	matchedNo, err := regexp.MatchString("^no$|^n$", strings.ToLower(s))
	if errorutils.CheckError(err) != nil {
		log.Error(err)
		return matchedNo, false
	}
	if matchedNo {
		return false, true
	}
	return false, false
}

func SetCliUserAgent(cliUserAgentToSet string) {
	cliUserAgent = cliUserAgentToSet
}

func GetCliUserAgent() string {
	return cliUserAgent
}

func SetVersion(versionToSet string) {
	version = versionToSet
}

func GetVersion() string {
	return version
}

func SetClientAgent(clientAgentToSet string) {
	clientAgent = clientAgentToSet
}

func GetClientAgent() string {
	return clientAgent
}

func GetCliLogLevel() log.LevelType {
	switch os.Getenv(LogLevel) {
	case "ERROR":
		return log.ERROR
	case "WARN":
		return log.WARN
	case "DEBUG":
		return log.DEBUG
	default:
		return log.INFO
	}
}
