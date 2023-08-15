package coreutils

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	GettingStartedGuideUrl = "https://github.com/jfrog/jfrog-cli/blob/v2/guides/getting-started-with-jfrog-using-the-cli.md"
	JFrogComUrl            = "https://jfrog.com/"
	JFrogHelpUrl           = JFrogComUrl + "help/r/"
)

type MinVersionProduct string

const (
	Artifactory  MinVersionProduct = "JFrog Artifactory"
	Xray         MinVersionProduct = "JFrog Xray"
	DataTransfer MinVersionProduct = "Data Transfer"
	DockerApi    MinVersionProduct = "Docker API"
	Projects     MinVersionProduct = "JFrog Projects"
)

const (
	// ReleasesRemoteEnv should be used for downloading the CLI dependencies (extractor jars, analyzerManager etc.) through an Artifactory remote
	// repository, instead of downloading directly from releases.jfrog.io. The remote repository should be
	// configured to proxy releases.jfrog.io.
	// This env var should store a server ID and a remote repository in form of '<ServerID>/<RemoteRepo>'
	ReleasesRemoteEnv = "JFROG_CLI_RELEASES_REPO"
	// DeprecatedExtractorsRemoteEnv is deprecated, it is replaced with ReleasesRemoteEnv.
	// Its functionality was similar to ReleasesRemoteEnv, but it proxies releases.jfrog.io/artifactory/oss-release-local instead.
	DeprecatedExtractorsRemoteEnv = "JFROG_CLI_EXTRACTORS_REMOTE"
	// JFrog releases URL
	JfrogReleasesUrl  = "https://releases.jfrog.io/artifactory/"
	MinimumVersionMsg = "You are using %s version %s, while this operation requires version %s or higher."
)

// Error modes (how should the application behave when the CheckError function is invoked):
type OnError string

var cliTempDir string

// User agent - the user of the program that uses this library (usually another program, or the same as the client agent), i.e 'jfrog-pipelines'
var cliUserAgentName string
var cliUserAgentVersion string

// Client agent - the program that uses this library, i.e 'jfrog-cli-go'
var clientAgentName string
var clientAgentVersion string

var cliExecutableName string

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

func SetIfEmpty(str *string, defaultStr string) bool {
	if *str == "" {
		*str = defaultStr
		return true
	}
	return false
}

func IsAnyEmpty(strings ...string) bool {
	for _, str := range strings {
		if str == "" {
			return true
		}
	}
	return false
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

func ExitOnErr(err error) {
	if err, ok := err.(CliError); ok {
		traceExit(err.ExitCode, err)
	}
	if exitCode := GetExitCode(err, 0, 0, false); exitCode != ExitCodeNoError {
		traceExit(exitCode, err)
	}
}

func traceExit(exitCode ExitCode, err error) {
	if err != nil && len(err.Error()) > 0 {
		log.Error(err)
	}
	os.Exit(exitCode.Code)
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

// When running a command in an external process, if the command fails to run or doesn't complete successfully ExitError is returned.
// We would like to return a regular error instead of ExitError,
// because some frameworks (such as codegangsta used by JFrog CLI) automatically exit when this error is returned.
func ConvertExitCodeError(err error) error {
	if _, ok := err.(*exec.ExitError); ok {
		err = errors.New(err.Error())
	}
	return err
}

// GetCliConfigVersion returns the latest version of the config.yml file on the file system at '.jfrog'.
func GetCliConfigVersion() int {
	return 6
}

// GetPluginsConfigVersion returns the latest plugins layout version on the file system (at '.jfrog/plugins').
func GetPluginsConfigVersion() int {
	return 1
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

func IsMac() bool {
	return runtime.GOOS == "darwin"
}

func GetOSAndArc() (string, error) {
	arch := runtime.GOARCH
	// Windows
	if IsWindows() {
		return "windows-amd64", nil
	}
	// Mac
	if IsMac() {
		if arch == "arm64" {
			return "mac-arm64", nil
		} else {
			return "mac-amd64", nil
		}
	}
	// Linux
	if IsLinux() {
		switch arch {
		case "i386", "i486", "i586", "i686", "i786", "x86":
			return "linux-386", nil
		case "amd64", "x86_64", "x64":
			return "linux-amd64", nil
		case "arm", "armv7l":
			return "linux-arm", nil
		case "arm64", "aarch64":
			return "linux-arm64", nil
		case "ppc64", "ppc64le":
			return "linux-" + arch, nil
		}
	}
	return "", errorutils.CheckErrorf("unsupported OS: %s-%s", runtime.GOOS, arch)
}

// Return the path of CLI temp dir.
// This path should be persistent, meaning - should not be cleared at the end of a CLI run.
func GetCliPersistentTempDirPath() string {
	return cliTempDir
}

func GetWorkingDirectory() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	if currentDir, err = filepath.Abs(currentDir); err != nil {
		return "", errorutils.CheckError(err)
	}

	return currentDir, nil
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
		content = bytes.ReplaceAll(content, []byte(key), []byte(val))
	}
	log.Debug("The reformatted content is: \n" + string(content))
	return content
}

func GetJfrogHomeDir() (string, error) {
	if os.Getenv(HomeDir) != "" {
		return os.Getenv(HomeDir), nil
	}

	userHomeDir := fileutils.GetHomeDir()
	if userHomeDir == "" {
		err := errorutils.CheckErrorf("couldn't find home directory. Make sure your HOME environment variable is set")
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

func GetJfrogPluginsDir() (string, error) {
	homeDir, err := GetJfrogHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, JfrogPluginsDirName), nil
}

func GetJfrogPluginsResourcesDir(pluginsName string) (string, error) {
	pluginsDir, err := GetJfrogPluginsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(pluginsDir, pluginsName, PluginsResourcesDirName), nil
}

func GetPluginsDirContent() ([]os.DirEntry, error) {
	pluginsDir, err := GetJfrogPluginsDir()
	if err != nil {
		return nil, err
	}
	exists, err := fileutils.IsDirExists(pluginsDir, false)
	if err != nil || !exists {
		return nil, err
	}
	content, err := os.ReadDir(pluginsDir)
	return content, errorutils.CheckError(err)
}

func ChmodPluginsDirectoryContent() error {
	plugins, err := GetPluginsDirContent()
	if err != nil || plugins == nil {
		return err
	}
	pluginsDir, err := GetJfrogPluginsDir()
	if err != nil {
		return err
	}
	for _, p := range plugins {
		err = os.Chmod(filepath.Join(pluginsDir, p.Name()), 0777)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetJfrogLocksDir() (string, error) {
	homeDir, err := GetJfrogHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, JfrogLocksDirName), nil
}

func GetJfrogConfigLockDir() (string, error) {
	configLockDirName := "config"
	locksDirPath, err := GetJfrogLocksDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(locksDirPath, configLockDirName), nil
}

func GetJfrogPluginsLockDir() (string, error) {
	pluginsLockDirName := "plugins"
	locksDirPath, err := GetJfrogLocksDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(locksDirPath, pluginsLockDirName), nil
}

func GetJfrogTransferLockDir() (string, error) {
	transferLockDirName := "transfer"
	locksDirPath, err := GetJfrogLocksDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(locksDirPath, transferLockDirName), nil
}

func GetJfrogTransferRunStatusFilePath() (string, error) {
	transferDir, err := GetJfrogTransferDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(transferDir, JfrogTransferRunStatusFileName), nil
}

func GetJfrogTransferRepositoriesDir() (string, error) {
	transferDir, err := GetJfrogTransferDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(transferDir, JfrogTransferRepositoriesDirName), nil
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
		log.Output("Please enter a valid option.")
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

func GetCliUserAgent() string {
	if cliUserAgentVersion == "" {
		return cliUserAgentName
	}
	return fmt.Sprintf("%s/%s", cliUserAgentName, cliUserAgentVersion)
}

func SetCliUserAgentName(cliUserAgentNameToSet string) {
	cliUserAgentName = cliUserAgentNameToSet
}

func GetCliUserAgentName() string {
	return cliUserAgentName
}

func SetCliUserAgentVersion(versionToSet string) {
	cliUserAgentVersion = versionToSet
}

func GetCliUserAgentVersion() string {
	return cliUserAgentVersion
}

func SetClientAgentName(clientAgentToSet string) {
	clientAgentName = clientAgentToSet
}

func GetClientAgentName() string {
	return clientAgentName
}

func SetClientAgentVersion(versionToSet string) {
	clientAgentVersion = versionToSet
}

func GetClientAgentVersion() string {
	return clientAgentVersion
}

func SetCliExecutableName(executableName string) {
	cliExecutableName = executableName
}

func GetCliExecutableName() string {
	return cliExecutableName
}

// Turn a list of strings into a sentence.
// For example, turn ["one", "two", "three"] into "one, two and three".
// For a single element: "one".
func ListToText(list []string) string {
	if len(list) == 1 {
		return list[0]
	}
	return strings.Join(list[0:len(list)-1], ", ") + " and " + list[len(list)-1]
}

func RemoveAllWhiteSpaces(input string) string {
	return strings.Join(strings.Fields(input), "")
}

func GetJfrogTransferDir() (string, error) {
	homeDir, err := GetJfrogHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, JfrogTransferDirName), nil
}

func ValidateMinimumVersion(product MinVersionProduct, currentVersion, minimumVersion string) error {
	if !version.NewVersion(currentVersion).AtLeast(minimumVersion) {
		return errorutils.CheckErrorf(MinimumVersionMsg, product, currentVersion, minimumVersion)
	}
	return nil
}

func GetServerIdAndRepo(remoteEnv string) (serverID string, repoName string, err error) {
	serverAndRepo := os.Getenv(remoteEnv)
	if serverAndRepo == "" {
		log.Debug(remoteEnv, "is not set")
		return
	}
	// The serverAndRepo is in the form of '<ServerID>/<RemoteRepo>'
	serverID, repoName, seperatorExists := strings.Cut(serverAndRepo, "/")
	// Check that the format is valid
	if !seperatorExists || repoName == "" || serverID == "" {
		err = errorutils.CheckErrorf("'%s' environment variable is '%s' but should be '<server ID>/<repo name>'", remoteEnv, serverAndRepo)
	}
	return
}
