package container

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Search for docker API version format pattern e.g. 1.40
var ApiVersionRegex = regexp.MustCompile(`^(\d+)\.(\d+)$`)

// Docker API version 1.31 is compatible with Docker version 17.07.0, according to https://docs.docker.com/engine/api/#api-version-matrix
const MinSupportedApiVersion string = "1.31"

// Docker login error message
const LoginFailureMessage string = "%s login failed for: %s.\n%s image must be in the form: registry-domain/path-in-repository/image-name:version."

func NewManager(containerManagerType ContainerManagerType) ContainerManager {
	return &containerManager{Type: containerManagerType}
}

type ContainerManagerType int

const (
	DockerClient ContainerManagerType = iota
	Podman
)

func (cmt ContainerManagerType) String() string {
	return [...]string{"docker", "podman"}[cmt]
}

// Container image
type ContainerManager interface {
	// Image ID is basically the image's SHA256
	Id(image *Image) (string, error)
	OsCompatibility(image *Image) (string, string, error)
	RunNativeCmd(cmdParams []string) error
	GetContainerManagerType() ContainerManagerType
}

type containerManager struct {
	Type ContainerManagerType
}

type ContainerManagerLoginConfig struct {
	ServerDetails *config.ServerDetails
}

// Run native command of the container buildtool
func (containerManager *containerManager) RunNativeCmd(cmdParams []string) error {
	cmd := &nativeCmd{cmdParams: cmdParams, containerManager: containerManager.Type}
	return cmd.RunCmd()
}

// Get image ID
func (containerManager *containerManager) Id(image *Image) (string, error) {
	cmd := &getImageIdCmd{image: image, containerManager: containerManager.Type}
	content, err := cmd.RunCmd()
	if err != nil {
		return "", err
	}
	return strings.Split(content, "\n")[0], nil
}

// Return the OS and architecture on which the image runs e.g. (linux, amd64, nil).
func (containerManager *containerManager) OsCompatibility(image *Image) (string, string, error) {
	cmd := &getImageSystemCompatibilityCmd{image: image, containerManager: containerManager.Type}
	log.Debug("Running image inspect...")
	content, err := cmd.RunCmd()
	if err != nil {
		return "", "", err
	}
	content = strings.Trim(content, "\n")
	firstSeparator := strings.Index(content, ",")
	if firstSeparator == -1 {
		return "", "", errorutils.CheckErrorf("couldn't find OS and architecture of image: %s", image.name)
	}
	return content[:firstSeparator], content[firstSeparator+1:], err
}

func (containerManager *containerManager) GetContainerManagerType() ContainerManagerType {
	return containerManager.Type
}

// Image push command
type nativeCmd struct {
	cmdParams        []string
	containerManager ContainerManagerType
}

func (nc *nativeCmd) GetCmd() *exec.Cmd {
	return exec.Command(nc.containerManager.String(), nc.cmdParams...)
}

func (nc *nativeCmd) RunCmd() error {
	command := nc.GetCmd()
	command.Stderr = os.Stderr
	command.Stdout = os.Stderr
	if nc.containerManager == DockerClient {
		command.Env = append(os.Environ(), "DOCKER_SCAN_SUGGEST=false")
	}
	return command.Run()
}

// Image get image id command
type getImageIdCmd struct {
	image            *Image
	containerManager ContainerManagerType
}

func (getImageId *getImageIdCmd) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, "images")
	cmd = append(cmd, "--format", "{{.ID}}")
	cmd = append(cmd, "--no-trunc")
	cmd = append(cmd, getImageId.image.name)
	return exec.Command(getImageId.containerManager.String(), cmd...)
}

func (getImageId *getImageIdCmd) RunCmd() (string, error) {
	command := getImageId.GetCmd()
	buffer := bytes.NewBuffer([]byte{})
	command.Stderr = buffer
	command.Stdout = buffer
	err := command.Run()
	return buffer.String(), err
}

// Get image system compatibility details
type getImageSystemCompatibilityCmd struct {
	image            *Image
	containerManager ContainerManagerType
}

func (getImageSystemCompatibilityCmd *getImageSystemCompatibilityCmd) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, "image")
	cmd = append(cmd, "inspect")
	cmd = append(cmd, getImageSystemCompatibilityCmd.image.name)
	cmd = append(cmd, "--format")
	cmd = append(cmd, "{{ .Os}},{{ .Architecture}}")
	return exec.Command(getImageSystemCompatibilityCmd.containerManager.String(), cmd...)
}

func (getImageSystemCompatibilityCmd *getImageSystemCompatibilityCmd) RunCmd() (string, error) {
	command := getImageSystemCompatibilityCmd.GetCmd()
	buffer := bytes.NewBuffer([]byte{})
	command.Stderr = buffer
	command.Stdout = buffer
	err := command.Run()
	return buffer.String(), err
}

// Login command
type LoginCmd struct {
	DockerRegistry   string
	Username         string
	Password         string
	containerManager ContainerManagerType
}

func (loginCmd *LoginCmd) GetCmd() *exec.Cmd {
	if coreutils.IsWindows() {
		return exec.Command("cmd", "/C", "echo", "%CONTAINER_MANAGER_PASS%|", "docker", "login", loginCmd.DockerRegistry, "--username", loginCmd.Username, "--password-stdin")
	}
	cmd := "echo $CONTAINER_MANAGER_PASS " + fmt.Sprintf(`| `+loginCmd.containerManager.String()+` login %s --username="%s" --password-stdin`, loginCmd.DockerRegistry, loginCmd.Username)
	return exec.Command("sh", "-c", cmd)
}

func (loginCmd *LoginCmd) RunCmd() error {
	command := loginCmd.GetCmd()
	command.Stderr = os.Stderr
	command.Stdout = os.Stderr
	command.Env = os.Environ()
	command.Env = append(command.Env, "CONTAINER_MANAGER_PASS="+loginCmd.Password)
	return command.Run()
}

// First we'll try to log in assuming a proxy-less tag (e.g. "registry-address/docker-repo/image:ver").
// If fails, we will try assuming a reverse proxy tag (e.g. "registry-address-docker-repo/image:ver").
func ContainerManagerLogin(imageRegistry string, config *ContainerManagerLoginConfig, containerManager ContainerManagerType) error {
	username := config.ServerDetails.User
	password := config.ServerDetails.Password
	// If access-token exists, perform login with it.
	if config.ServerDetails.AccessToken != "" {
		if username == "" {
			username = auth.ExtractUsernameFromAccessToken(config.ServerDetails.AccessToken)
		}
		password = config.ServerDetails.AccessToken
	}
	// Perform login.
	cmd := &LoginCmd{DockerRegistry: imageRegistry, Username: username, Password: password, containerManager: containerManager}
	err := cmd.RunCmd()
	if exitCode := coreutils.GetExitCode(err, 0, 0, false); exitCode == coreutils.ExitCodeNoError {
		// Login succeeded
		return nil
	}
	log.Debug(containerManager.String()+" login while assuming proxy-less failed:", err)
	indexOfSlash := strings.Index(imageRegistry, "/")
	if indexOfSlash < 0 {
		return errorutils.CheckErrorf(LoginFailureMessage, containerManager.String(), imageRegistry, containerManager.String())
	}
	cmd = &LoginCmd{DockerRegistry: imageRegistry[:indexOfSlash], Username: config.ServerDetails.User, Password: config.ServerDetails.Password}
	err = cmd.RunCmd()
	if err != nil {
		// Login failed for both attempts
		return errorutils.CheckErrorf(LoginFailureMessage,
			containerManager.String(), fmt.Sprintf("%s, %s", imageRegistry, imageRegistry[:indexOfSlash]), containerManager.String()+" "+err.Error())
	}
	// Login succeeded
	return nil
}

// Version command
// Docker-client provides an API for interacting with the Docker daemon. This cmd should be used for docker client only.
type VersionCmd struct{}

func (versionCmd *VersionCmd) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, "docker")
	cmd = append(cmd, "version")
	cmd = append(cmd, "--format", "{{.Client.APIVersion}}")
	return exec.Command(cmd[0], cmd[1:]...)
}

func (versionCmd *VersionCmd) RunCmd() (string, error) {
	command := versionCmd.GetCmd()
	buffer := bytes.NewBuffer([]byte{})
	command.Stderr = buffer
	command.Stdout = buffer
	err := command.Run()
	return buffer.String(), err
}

func ValidateClientApiVersion() error {
	cmd := &VersionCmd{}
	// 'docker version' may return 1 in case of errors from daemon. We should ignore this kind of error.
	content, err := cmd.RunCmd()
	content = strings.TrimSpace(content)
	if !ApiVersionRegex.Match([]byte(content)) {
		// The Api version is expected to be 'major.minor'. Anything else should return an error.
		log.Error("The Docker client Api version is expected to be 'major.minor'. The actual output is:", content)
		return errorutils.CheckError(err)
	}
	return utils.ValidateMinimumVersion(utils.DockerApi, content, MinSupportedApiVersion)
}
