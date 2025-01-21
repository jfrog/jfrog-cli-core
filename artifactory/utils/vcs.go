package utils

import (
	"errors"
	buildinfo "github.com/jfrog/build-info-go/entities"
	gofrogcmd "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	utilsconfig "github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	artclientutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io"
	"os"
	"os/exec"
	"strconv"
)

func getLatestVcsRevision(serverDetails *utilsconfig.ServerDetails, buildConfiguration *build.BuildConfiguration, vcsUrl string) (string, error) {
	// Get latest build's build-info from Artifactory
	buildInfo, err := getLatestBuildInfo(serverDetails, buildConfiguration)
	if err != nil {
		return "", err
	}

	// Get previous VCS Revision from BuildInfo.
	lastVcsRevision := ""
	for _, vcs := range buildInfo.VcsList {
		if vcs.Url == vcsUrl {
			lastVcsRevision = vcs.Revision
			break
		}
	}

	return lastVcsRevision, nil
}

// Returns build info, or empty build info struct if not found.
func getLatestBuildInfo(serverDetails *utilsconfig.ServerDetails, buildConfiguration *build.BuildConfiguration) (*buildinfo.BuildInfo, error) {
	// Create services manager to get build-info from Artifactory.
	sm, err := CreateServiceManager(serverDetails, -1, 0, false)
	if err != nil {
		return nil, err
	}

	// Get latest build-info from Artifactory.
	buildName, err := buildConfiguration.GetBuildName()
	if err != nil {
		return nil, err
	}
	buildInfoParams := services.BuildInfoParams{BuildName: buildName, BuildNumber: artclientutils.LatestBuildNumberKey}
	publishedBuildInfo, found, err := sm.GetBuildInfo(buildInfoParams)
	if err != nil {
		return nil, err
	}
	if !found {
		return &buildinfo.BuildInfo{}, nil
	}

	return &publishedBuildInfo.BuildInfo, nil
}

type GitParsingDetails struct {
	DotGitPath   string
	VcsUrl       string
	LogLimit     int
	PrettyFormat string
}

func ParseGitLogsFromLastBuild(serverDetails *utilsconfig.ServerDetails, buildConfiguration *build.BuildConfiguration, gitDetails GitParsingDetails, logRegExp *gofrogcmd.CmdOutputPattern) error {
	// Check that git exists in path.
	_, err := exec.LookPath("git")
	if err != nil {
		return errorutils.CheckError(err)
	}

	// Get latest build's VCS revision from Artifactory.
	lastVcsRevision, err := getLatestVcsRevision(serverDetails, buildConfiguration, gitDetails.VcsUrl)
	if err != nil {
		return err
	}
	return ParseGitLogsFromLastVcsRevision(gitDetails, logRegExp, lastVcsRevision)
}

func ParseGitLogsFromLastVcsRevision(gitDetails GitParsingDetails, logRegExp *gofrogcmd.CmdOutputPattern, lastVcsRevision string) error {
	errRegExp, err := createErrRegExpHandler(lastVcsRevision)
	if err != nil {
		return err
	}

	// Get log with limit, starting from the latest commit.
	logCmd := &LogCmd{logLimit: gitDetails.LogLimit, lastVcsRevision: lastVcsRevision, prettyFormat: gitDetails.PrettyFormat}

	// Change working dir to where .git is.
	wd, err := os.Getwd()
	if errorutils.CheckError(err) != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, errorutils.CheckError(os.Chdir(wd)))
	}()
	err = os.Chdir(gitDetails.DotGitPath)
	if errorutils.CheckError(err) != nil {
		return err
	}

	// Run git command.
	_, _, exitOk, err := gofrogcmd.RunCmdWithOutputParser(logCmd, false, logRegExp, errRegExp)
	if errorutils.CheckError(err) != nil {
		var revisionRangeError RevisionRangeError
		if errors.As(err, &revisionRangeError) {
			// Revision not found in range. Ignore and return.
			log.Info(err.Error())
			return nil
		}
		return err
	}
	if !exitOk {
		// May happen when trying to run git log for non-existing revision.
		err = errorutils.CheckErrorf("failed executing git log command")
	}
	return err
}

// Creates a regexp handler to handle the event of revision missing in the git revision range.
func createErrRegExpHandler(lastVcsRevision string) (*gofrogcmd.CmdOutputPattern, error) {
	// Create regex pattern.
	invalidRangeExp, err := clientutils.GetRegExp(`fatal: Invalid revision range [a-fA-F0-9]+\.\.`)
	if err != nil {
		return nil, err
	}

	// Create handler with exec function.
	errRegExp := gofrogcmd.CmdOutputPattern{
		RegExp: invalidRangeExp,
		ExecFunc: func(pattern *gofrogcmd.CmdOutputPattern) (string, error) {
			// Revision could not be found in the revision range, probably due to a squash / revert. Ignore and return.
			errMsg := "Revision: '" + lastVcsRevision + "' that was fetched from latest build info does not exist in the git revision range."
			return "", RevisionRangeError{ErrorMsg: errMsg}
		},
	}
	return &errRegExp, nil
}

type LogCmd struct {
	logLimit        int
	lastVcsRevision string
	prettyFormat    string
}

func (logCmd *LogCmd) GetCmd() *exec.Cmd {
	var cmd []string
	cmd = append(cmd, "git")
	cmd = append(cmd, "log", "--pretty="+logCmd.prettyFormat, "-"+strconv.Itoa(logCmd.logLimit))
	if logCmd.lastVcsRevision != "" {
		cmd = append(cmd, logCmd.lastVcsRevision+"..")
	}
	return exec.Command(cmd[0], cmd[1:]...)
}

func (logCmd *LogCmd) GetEnv() map[string]string {
	return map[string]string{}
}

func (logCmd *LogCmd) GetStdWriter() io.WriteCloser {
	return nil
}

func (logCmd *LogCmd) GetErrWriter() io.WriteCloser {
	return nil
}

// Error to be thrown when revision could not be found in the git revision range.
type RevisionRangeError struct {
	ErrorMsg string
}

func (err RevisionRangeError) Error() string {
	return err.ErrorMsg
}
