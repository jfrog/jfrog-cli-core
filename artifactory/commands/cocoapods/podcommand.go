package cocoapods

import (
	"bytes"
	"fmt"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	minSupportedPodVersion = "1.15.2"
	podNetRcfileName       = ".netrc"
	podrcBackupFileName    = ".jfrog.netrc.backup"
)

type PodCommand struct {
	cmdName          string
	repo             string
	podArgs          []string
	serverDetails    *config.ServerDetails
	podVersion       *version.Version
	authArtDetails   auth.ServiceDetails
	restoreNetrcFunc func() error
	workingDirectory string
	executablePath   string
}

func GetPodVersionAndExecPath() (*version.Version, string, error) {
	podExecPath, err := exec.LookPath("pod")
	if err != nil {
		return nil, "", fmt.Errorf("could not find the 'pod' executable in the system PATH", err)
	}
	log.Debug("Using pod executable:", podExecPath)
	versionData, _, err := RunPodCmd(podExecPath, "", []string{"--version"})
	if err != nil {
		return nil, "", err
	}
	return version.NewVersion(strings.TrimSpace(string(versionData))), podExecPath, nil
}

func RunPodCmd(executablePath, srcPath string, podArgs []string) (stdResult, errResult []byte, err error) {
	args := make([]string, 0)
	for i := 0; i < len(podArgs); i++ {
		if strings.TrimSpace(podArgs[i]) != "" {
			args = append(args, podArgs[i])
		}
	}
	log.Debug("Running 'pod " + strings.Join(podArgs, " ") + "' command.")
	command := exec.Command(executablePath, args...)
	command.Dir = srcPath
	outBuffer := bytes.NewBuffer([]byte{})
	command.Stdout = outBuffer
	errBuffer := bytes.NewBuffer([]byte{})
	command.Stderr = errBuffer
	err = command.Run()
	errResult = errBuffer.Bytes()
	stdResult = outBuffer.Bytes()
	if err != nil {
		err = fmt.Errorf("error while running '%s %s': %s\n%s", executablePath, strings.Join(args, " "), err.Error(), strings.TrimSpace(string(errResult)))
		return
	}
	log.Debug("npm '" + strings.Join(args, " ") + "' standard output is:\n" + strings.TrimSpace(string(stdResult)))
	return
}

func (pc *PodCommand) SetServerDetails(serverDetails *config.ServerDetails) *PodCommand {
	pc.serverDetails = serverDetails
	return pc
}

func (pc *PodCommand) RestoreNetrcFunc() func() error {
	return pc.restoreNetrcFunc
}

func (pc *PodCommand) GetData() ([]byte, error) {
	var filteredConf []string
	filteredConf = append(filteredConf, "machine ", pc.serverDetails.Url, "\n")
	filteredConf = append(filteredConf, "login ", pc.serverDetails.User, "\n")
	filteredConf = append(filteredConf, "password ", pc.serverDetails.AccessToken, "\n")

	return []byte(strings.Join(filteredConf, "")), nil
}

func (pc *PodCommand) CreateTempNetrc() error {
	data, err := pc.GetData()
	if err != nil {
		return err
	}
	if err = removeNetrcIfExists(pc.workingDirectory); err != nil {
		return err
	}
	log.Debug("Creating temporary .netrc file.")
	return errorutils.CheckError(os.WriteFile(filepath.Join(pc.workingDirectory, podNetRcfileName), data, 0755))
}

func (pc *PodCommand) setRestoreNetrcFunc() error {
	restoreNetrcFunc, err := ioutils.BackupFile(filepath.Join(pc.workingDirectory, podNetRcfileName), podrcBackupFileName)
	if err != nil {
		return err
	}
	pc.restoreNetrcFunc = func() error {
		return restoreNetrcFunc()
	}
	return nil
}

func (pc *PodCommand) setArtifactoryAuth() error {
	authArtDetails, err := pc.serverDetails.CreateArtAuthConfig()
	if err != nil {
		return err
	}
	if authArtDetails.GetSshAuthHeaders() != nil {
		return errorutils.CheckErrorf("SSH authentication is not supported in this command")
	}
	pc.authArtDetails = authArtDetails
	return nil
}

func NewPodInstallCommand() *PodCommand {
	return &PodCommand{cmdName: "install"}
}

func (pc *PodCommand) PreparePrerequisites() error {
	log.Debug("Preparing prerequisites...")
	var err error
	pc.podVersion, pc.executablePath, err = GetPodVersionAndExecPath()
	if err != nil {
		return err
	}
	if pc.podVersion.Compare(minSupportedPodVersion) > 0 {
		return errorutils.CheckErrorf(
			"JFrog CLI cocoapods %s command requires cocoapods client version %s or higher. The Current version is: %s", pc.cmdName, minSupportedPodVersion, pc.podVersion.GetVersion())
	}

	pc.workingDirectory, err = coreutils.GetWorkingDirectory()
	if err != nil {
		return err
	}
	log.Debug("Working directory set to:", pc.workingDirectory)
	if err = pc.setArtifactoryAuth(); err != nil {
		return err
	}

	return pc.setRestoreNetrcFunc()
}

func removeNetrcIfExists(workingDirectory string) error {
	if _, err := os.Stat(filepath.Join(workingDirectory, podNetRcfileName)); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errorutils.CheckError(err)
	}

	log.Debug("Removing existing .npmrc file")
	return errorutils.CheckError(os.Remove(filepath.Join(workingDirectory, podNetRcfileName)))
}

func SetArtifactoryAsResolutionServer(serverDetails *config.ServerDetails, depsRepo string) (clearResolutionServerFunc func() error, err error) {
	podCmd := NewPodInstallCommand().SetServerDetails(serverDetails)
	if err = podCmd.PreparePrerequisites(); err != nil {
		return
	}
	if err = podCmd.CreateTempNetrc(); err != nil {
		return
	}
	clearResolutionServerFunc = podCmd.RestoreNetrcFunc()
	log.Info(fmt.Sprintf("Resolving dependecies from '%s' from repo '%s'", serverDetails.Url, depsRepo))
	return
}
