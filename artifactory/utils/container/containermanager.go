package container

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"

	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-cli-core/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-client-go/utils/version"
)

// Search for docker API version format pattern e.g. 1.40
var ApiVersionRegex = regexp.MustCompile(`^(\d+)\.(\d+)$`)

// Docker API version 1.31 is compatible with Docker version 17.07.0, according to https://docs.docker.com/engine/api/#api-version-matrix
const MinSupportedApiVersion string = "1.31"

// Container login error message
const LoginFailureMessage string = "%s login failed for: %s.\n %s image must be in the form: registry-domain/path-in-repository/image-name:version."

func NewContainerManager(containerManagerType ContainerManagerType) ContainerManager {
	return &containerManager{Type: containerManagerType}
}

type ContainerManagerType int

const (
	Docker ContainerManagerType = iota
	Podman
)

func (cmt ContainerManagerType) String() string {
	return [...]string{"docker", "podman"}[cmt]
}

// Container image
type ContainerManager interface {
	Push(image *Image) error
	Id(image *Image) (string, error)
	ParentId(image *Image) (string, error)
	ImageCompatibility(image *Image) (string, string, error)
	Pull(image *Image) error
}

type containerManager struct {
	Type ContainerManagerType
}

type ContainerManagerLoginConfig struct {
	ArtifactoryDetails *config.ArtifactoryDetails
}

// Push image
func (containerManager *containerManager) Push(image *Image) error {
	cmd := &pushCmd{imageTag: image, containerManager: containerManager.Type}
	return gofrogcmd.RunCmd(cmd)
}

// Get image ID
func (containerManager *containerManager) Id(image *Image) (string, error) {
	cmd := &getImageIdCmd{image: image, containerManager: containerManager.Type}
	content, err := gofrogcmd.RunCmdOutput(cmd)
	return content[:strings.Index(content, "\n")], err
}

// Get parent image ID
func (containerManager *containerManager) ParentId(image *Image) (string, error) {
	cmd := &getParentId{image: image, containerManager: containerManager.Type}
	content, err := gofrogcmd.RunCmdOutput(cmd)
	return content[:strings.Index(content, "\n")], err
}

// Pull image
func (containerManager *containerManager) Pull(image *Image) error {
	cmd := &pullCmd{image: image, containerManager: containerManager.Type}
	return gofrogcmd.RunCmd(cmd)
}

// Return the OS and architecture on which the image runs e.g. (linux, amd64, nil).
func (containerManager *containerManager) ImageCompatibility(image *Image) (string, string, error) {
	cmd := &getImageSystemCompatibilityCmd{image: image, containerManager: containerManager.Type}
	content, err := gofrogcmd.RunCmdOutput(cmd)
	if err != nil {
		return "", "", err
	}
	content = strings.Trim(content, "\n")
	firstSeparator := strings.Index(content, ",")
	if firstSeparator == -1 {
		return "", "", errorutils.CheckError(errors.New("couldn't find Image OS and architecture of image:" + image.tag))
	}
	return content[:firstSeparator], content[firstSeparator+1:], err
}

func NewImage(tag string) *Image {
	return &Image{tag: tag}
}

// Image push command
type pushCmd struct {
	imageTag         *Image
	containerManager ContainerManagerType
}

func (pushCmd *pushCmd) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, "push")
	cmd = append(cmd, pushCmd.imageTag.tag)
	return exec.Command(pushCmd.containerManager.String(), cmd[:]...)
}

func (pushCmd *pushCmd) GetEnv() map[string]string {
	return map[string]string{}
}

func (pushCmd *pushCmd) GetStdWriter() io.WriteCloser {
	return nil
}
func (pushCmd *pushCmd) GetErrWriter() io.WriteCloser {
	return nil
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
	cmd = append(cmd, getImageId.image.tag)
	return exec.Command(getImageId.containerManager.String(), cmd[:]...)
}

func (getImageId *getImageIdCmd) GetEnv() map[string]string {
	return map[string]string{}
}

func (getImageId *getImageIdCmd) GetStdWriter() io.WriteCloser {
	return nil
}

func (getImageId *getImageIdCmd) GetErrWriter() io.WriteCloser {
	return nil
}

type FatManifest struct {
	Manifests []ManifestDetails `json:"manifests"`
}

type ManifestDetails struct {
	Digest   string   `json:"digest"`
	Platform Platform `json:"platform"`
}

type Platform struct {
	Architecture string `json:"architecture"`
	Os           string `json:"os"`
}

// Image get parent image id command
type getParentId struct {
	image            *Image
	containerManager ContainerManagerType
}

func (getParentId *getParentId) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, "inspect")
	cmd = append(cmd, "--format", "{{.Parent}}")
	cmd = append(cmd, getParentId.image.tag)
	return exec.Command(getParentId.containerManager.String(), cmd[:]...)
}

func (getParentId *getParentId) GetEnv() map[string]string {
	return map[string]string{}
}

func (getParentId *getParentId) GetStdWriter() io.WriteCloser {
	return nil
}

func (getParentId *getParentId) GetErrWriter() io.WriteCloser {
	return nil
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
	cmd = append(cmd, getImageSystemCompatibilityCmd.image.tag)
	cmd = append(cmd, "--format")
	cmd = append(cmd, "{{ .Os}},{{ .Architecture}}")
	return exec.Command(getImageSystemCompatibilityCmd.containerManager.String(), cmd[:]...)
}

func (getImageSystemCompatibilityCmd *getImageSystemCompatibilityCmd) GetEnv() map[string]string {
	return map[string]string{}
}

func (getImageSystemCompatibilityCmd *getImageSystemCompatibilityCmd) GetStdWriter() io.WriteCloser {
	return nil
}

func (getImageSystemCompatibilityCmd *getImageSystemCompatibilityCmd) GetErrWriter() io.WriteCloser {
	return nil
}

// Get registry from tag
func ResolveRegistryFromTag(imageTag string) (string, error) {
	indexOfFirstSlash := strings.Index(imageTag, "/")
	if indexOfFirstSlash < 0 {
		err := errorutils.CheckError(errors.New("Invalid image tag received for pushing to Artifactory - tag does not include a slash."))
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
	ContainerRegistry string
	Username          string
	Password          string
	containerManager  ContainerManagerType
}

func (loginCmd *LoginCmd) GetCmd() *exec.Cmd {
	if coreutils.IsWindows() {
		return exec.Command("cmd", "/C", "echo", "%CONTAINER_MANAGER_PASS%|", "docker", "login", loginCmd.ContainerRegistry, "--username", loginCmd.Username, "--password-stdin")
	}
	cmd := "echo $CONTAINER_MANAGER_PASS " + fmt.Sprintf(`| `+loginCmd.containerManager.String()+` login %s --username="%s" --password-stdin`, loginCmd.ContainerRegistry, loginCmd.Username)
	return exec.Command("sh", "-c", cmd)
}

func (loginCmd *LoginCmd) GetEnv() map[string]string {
	return map[string]string{"CONTAINER_MANAGER_PASS": loginCmd.Password}
}

func (loginCmd *LoginCmd) GetStdWriter() io.WriteCloser {
	return nil
}

func (loginCmd *LoginCmd) GetErrWriter() io.WriteCloser {
	return nil
}

// Image pull command
type pullCmd struct {
	image            *Image
	containerManager ContainerManagerType
}

func (pullCmd *pullCmd) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, "pull")
	cmd = append(cmd, pullCmd.image.tag)
	return exec.Command(pullCmd.containerManager.String(), cmd[:]...)
}

func (pullCmd *pullCmd) GetEnv() map[string]string {
	return map[string]string{}
}

func (pullCmd *pullCmd) GetStdWriter() io.WriteCloser {
	return nil
}

func (pullCmd *pullCmd) GetErrWriter() io.WriteCloser {
	return nil
}

// First will try to login assuming a proxy-less tag (e.g. "registry-address/docker-repo/image:ver").
// If fails, we will try assuming a reverse proxy tag (e.g. "registry-address-docker-repo/image:ver").
func ContainerManagerLogin(imageTag string, config *ContainerManagerLoginConfig, containerManager ContainerManagerType) error {
	imageRegistry, err := ResolveRegistryFromTag(imageTag)
	if err != nil {
		return err
	}
	username := config.ArtifactoryDetails.User
	password := config.ArtifactoryDetails.Password
	// If access-token exists, perform login with it.
	if config.ArtifactoryDetails.AccessToken != "" {
		log.Debug("Using access-token details in " + containerManager.String() + "-login command.")
		username, err = auth.ExtractUsernameFromAccessToken(config.ArtifactoryDetails.AccessToken)
		if err != nil {
			return err
		}
		password = config.ArtifactoryDetails.AccessToken
	}
	// Perform login.
	cmd := &LoginCmd{ContainerRegistry: imageRegistry, Username: username, Password: password, containerManager: containerManager}
	err = gofrogcmd.RunCmd(cmd)
	if exitCode := coreutils.GetExitCode(err, 0, 0, false); exitCode == coreutils.ExitCodeNoError {
		// Login succeeded
		return nil
	}
	log.Debug(containerManager.String()+" login while assuming proxy-less failed:", err)
	indexOfSlash := strings.Index(imageRegistry, "/")
	if indexOfSlash < 0 {
		return errorutils.CheckError(errors.New(fmt.Sprintf(LoginFailureMessage, containerManager.String(), imageRegistry, containerManager.String())))
	}
	cmd = &LoginCmd{ContainerRegistry: imageRegistry[:indexOfSlash], Username: config.ArtifactoryDetails.User, Password: config.ArtifactoryDetails.Password}
	err = gofrogcmd.RunCmd(cmd)
	if err != nil {
		// Login failed for both attempts
		return errorutils.CheckError(errors.New(fmt.Sprintf(LoginFailureMessage,
			containerManager.String(), fmt.Sprintf("%s, %s", imageRegistry, imageRegistry[:indexOfSlash]), containerManager.String()) + " " + err.Error()))
	}
	// Login succeeded
	return nil
}

// Version command
// Docker only
type VersionCmd struct{}

func (versionCmd *VersionCmd) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, "docker")
	cmd = append(cmd, "version")
	cmd = append(cmd, "--format", "{{.Client.APIVersion}}")
	return exec.Command(cmd[0], cmd[1:]...)
}

func (versionCmd *VersionCmd) GetEnv() map[string]string {
	return map[string]string{}
}

func (versionCmd *VersionCmd) GetStdWriter() io.WriteCloser {
	return nil
}

func (versionCmd *VersionCmd) GetErrWriter() io.WriteCloser {
	return nil
}

func ValidateClientApiVersion() error {
	cmd := &VersionCmd{}
	// 'docker version' may return 1 in case of errors from daemon. We should ignore this kind of errors.
	content, err := gofrogcmd.RunCmdOutput(cmd)
	content = strings.TrimSpace(content)
	if !ApiVersionRegex.Match([]byte(content)) {
		// The Api version is expected to be 'major.minor'. Anything else should return an error.
		return errorutils.CheckError(err)
	}
	if !IsCompatibleApiVersion(content) {
		return errorutils.CheckError(errors.New("This operation requires Docker API version " + MinSupportedApiVersion + " or higher."))
	}
	return nil
}

func IsCompatibleApiVersion(dockerOutput string) bool {
	currentVersion := version.NewVersion(dockerOutput)
	return currentVersion.AtLeast(MinSupportedApiVersion)
}
