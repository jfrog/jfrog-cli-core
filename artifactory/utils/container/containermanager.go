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
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/version"
)

// Search for docker API version format pattern e.g. 1.40
var ApiVersionRegex = regexp.MustCompile(`^(\d+)\.(\d+)$`)

// Docker API version 1.31 is compatible with Docker version 17.07.0, according to https://docs.docker.com/engine/api/#api-version-matrix
const MinSupportedApiVersion string = "1.31"

// Docker login error message
const LoginFailureMessage string = "%s login failed for: %s.\n %s image must be in the form: registry-domain/path-in-repository/image-name:version."

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
	Push(image *Image) error
	// Image ID is basically the image's SHA256
	Id(image *Image) (string, error)
	OsCompatibility(image *Image) (string, string, error)
	Pull(image *Image) error
	GetContainerManagerType() ContainerManagerType
}

type containerManager struct {
	Type ContainerManagerType
}

type ContainerManagerLoginConfig struct {
	ServerDetails *config.ServerDetails
}

// Push image
func (containerManager *containerManager) Push(image *Image) error {
	cmd := &pushCmd{imageTag: image, containerManager: containerManager.Type}
	return cmd.RunCmd()
}

// Get image ID
func (containerManager *containerManager) Id(image *Image) (string, error) {
	cmd := &getImageIdCmd{image: image, containerManager: containerManager.Type}
	content, err := cmd.RunCmd()
	return content[:strings.Index(content, "\n")], err
}

// Pull image
func (containerManager *containerManager) Pull(image *Image) error {
	cmd := &pullCmd{image: image, containerManager: containerManager.Type}
	return cmd.RunCmd()
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
		return "", "", errorutils.CheckErrorf("couldn't find OS and architecture of image:" + image.name)
	}
	return content[:firstSeparator], content[firstSeparator+1:], err
}

func (containerManager *containerManager) GetContainerManagerType() ContainerManagerType {
	return containerManager.Type
}

func NewImage(tag string) *Image {
	return &Image{name: tag}
}

// Image push command
type pushCmd struct {
	imageTag         *Image
	containerManager ContainerManagerType
}

func (pushCmd *pushCmd) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, "push")
	cmd = append(cmd, pushCmd.imageTag.name)
	return exec.Command(pushCmd.containerManager.String(), cmd[:]...)
}

func (pushCmd *pushCmd) RunCmd() error {
	command := pushCmd.GetCmd()
	command.Stderr = os.Stderr
	command.Stdout = os.Stderr
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
	return exec.Command(getImageId.containerManager.String(), cmd[:]...)
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
	return exec.Command(getImageSystemCompatibilityCmd.containerManager.String(), cmd[:]...)
}

func (getImageSystemCompatibilityCmd *getImageSystemCompatibilityCmd) RunCmd() (string, error) {
	command := getImageSystemCompatibilityCmd.GetCmd()
	buffer := bytes.NewBuffer([]byte{})
	command.Stderr = buffer
	command.Stdout = buffer
	err := command.Run()
	return buffer.String(), err
}

// Get registry from tag
func ResolveRegistryFromTag(imageTag string) (string, error) {
	indexOfFirstSlash := strings.Index(imageTag, "/")
	if indexOfFirstSlash < 0 {
		err := errorutils.CheckErrorf("Invalid image tag received for pushing to Artifactory - tag does not include a slash.")
		return "", err
	}
	indexOfSecondSlash := strings.Index(imageTag[indexOfFirstSlash+1:], "/")
	// Reverse proxy Artifactory
	if indexOfSecondSlash < 0 {
		return imageTag[:indexOfFirstSlash], nil
	}
	// Can be reverse proxy or proxy-less Artifactory
	indexOfSecondSlash += indexOfFirstSlash + 1
	return imageTag[:indexOfSecondSlash], nil
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

// Image pull command
type pullCmd struct {
	image            *Image
	containerManager ContainerManagerType
}

func (pullCmd *pullCmd) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, "pull")
	cmd = append(cmd, pullCmd.image.name)
	return exec.Command(pullCmd.containerManager.String(), cmd[:]...)
}

func (pullCmd *pullCmd) RunCmd() error {
	command := pullCmd.GetCmd()
	command.Stderr = os.Stderr
	command.Stdout = os.Stderr
	return command.Run()
}

// First we'll try to login assuming a proxy-less tag (e.g. "registry-address/docker-repo/image:ver").
// If fails, we will try assuming a reverse proxy tag (e.g. "registry-address-docker-repo/image:ver").
func ContainerManagerLogin(imageTag string, config *ContainerManagerLoginConfig, containerManager ContainerManagerType) error {
	imageRegistry, err := ResolveRegistryFromTag(imageTag)
	if err != nil {
		return err
	}
	username := config.ServerDetails.User
	password := config.ServerDetails.Password
	// If access-token exists, perform login with it.
	if config.ServerDetails.AccessToken != "" {
		log.Debug("Using access-token details in " + containerManager.String() + "-login command.")
		username, err = auth.ExtractUsernameFromAccessToken(config.ServerDetails.AccessToken)
		if err != nil {
			return err
		}
		password = config.ServerDetails.AccessToken
	}
	// Perform login.
	cmd := &LoginCmd{DockerRegistry: imageRegistry, Username: username, Password: password, containerManager: containerManager}
	err = cmd.RunCmd()
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
	// 'docker version' may return 1 in case of errors from daemon. We should ignore this kind of errors.
	content, err := cmd.RunCmd()
	content = strings.TrimSpace(content)
	if !ApiVersionRegex.Match([]byte(content)) {
		// The Api version is expected to be 'major.minor'. Anything else should return an error.
		return errorutils.CheckError(err)
	}
	if !IsCompatibleApiVersion(content) {
		return errorutils.CheckErrorf("This operation requires Docker API version " + MinSupportedApiVersion + " or higher.")
	}
	return nil
}

func IsCompatibleApiVersion(dockerOutput string) bool {
	currentVersion := version.NewVersion(dockerOutput)
	return currentVersion.AtLeast(MinSupportedApiVersion)
}
