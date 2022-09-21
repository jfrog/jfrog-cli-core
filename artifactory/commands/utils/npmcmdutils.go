package utils

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/jfrog/gofrog/version"

	xrutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"

	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const minSupportedArtifactoryVersionForNpmCmds = "5.5.2"

func GetArtifactoryNpmRepoDetails(repo string, authArtDetails *auth.ServiceDetails) (npmAuth, registry string, err error) {
	npmAuth, err = getNpmAuth(authArtDetails)
	if err != nil {
		return "", "", err
	}

	if err = utils.ValidateRepoExists(repo, *authArtDetails); err != nil {
		return "", "", err
	}

	registry = getNpmRepositoryUrl(repo, (*authArtDetails).GetUrl())
	return
}

func getNpmAuth(authArtDetails *auth.ServiceDetails) (npmAuth string, err error) {
	// Check Artifactory version
	err = validateArtifactoryVersionForNpmCmds(authArtDetails)
	if err != nil {
		return
	}

	// Get npm token from Artifactory
	return getNpmAuthFromArtifactory(authArtDetails)
}

func validateArtifactoryVersionForNpmCmds(artDetails *auth.ServiceDetails) error {
	// Get Artifactory version.
	versionStr, err := (*artDetails).GetVersion()
	if err != nil {
		return err
	}

	// Validate version.
	rtVersion := version.NewVersion(versionStr)
	if !rtVersion.AtLeast(minSupportedArtifactoryVersionForNpmCmds) {
		return errorutils.CheckErrorf("this operation requires Artifactory version " + minSupportedArtifactoryVersionForNpmCmds + " or higher")
	}

	return nil
}

func getNpmAuthFromArtifactory(artDetails *auth.ServiceDetails) (npmAuth string, err error) {
	authApiUrl := (*artDetails).GetUrl() + "api/npm/auth"
	log.Debug("Sending npm auth request")

	// Get npm token from Artifactory.
	client, err := httpclient.ClientBuilder().SetRetries(3).Build()
	if err != nil {
		return "", err
	}
	resp, body, _, err := client.SendGet(authApiUrl, true, (*artDetails).CreateHttpClientDetails(), "")
	if err != nil {
		return "", err
	}
	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return "", err
	}

	return string(body), nil
}

func getNpmRepositoryUrl(repo, url string) string {
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	url += "api/npm/" + repo
	return url
}

// Remove all the none npm CLI flags from args.
func ExtractNpmOptionsFromArgs(args []string) (detailedSummary, xrayScan bool, scanOutputFormat xrutils.OutputFormat, cleanArgs []string, buildConfig *utils.BuildConfiguration, err error) {
	cleanArgs = append([]string(nil), args...)
	cleanArgs, detailedSummary, err = coreutils.ExtractDetailedSummaryFromArgs(cleanArgs)
	if err != nil {
		return
	}

	cleanArgs, xrayScan, err = coreutils.ExtractXrayScanFromArgs(cleanArgs)
	if err != nil {
		return
	}

	cleanArgs, format, err := coreutils.ExtractXrayOutputFormatFromArgs(cleanArgs)
	if err != nil {
		return
	}
	scanOutputFormat, err = GetXrayOutputFormat(format)
	if err != nil {
		return
	}
	cleanArgs, buildConfig, err = utils.ExtractBuildDetailsFromArgs(cleanArgs)
	return
}

// BackupFile creates a backup of the file in filePath. The backup will be found at backupPath.
// The returned restore function can be called to restore the file's state - the file in filePath will be replaced by the backup in backupPath.
// If there is no file at filePath, a backup file won't be created, and the restore function will delete the file at filePath.
func BackupFile(filePath, backupPath string) (restore func() error, err error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return createRestoreFileFunc(filePath, backupPath), nil
		}
		return nil, errorutils.CheckError(err)
	}

	fileMode := fileInfo.Mode()
	if err = ioutils.CopyFile(filePath, backupPath, fileMode); err != nil {
		return nil, err
	}
	log.Debug("The file", filePath, "was backed up successfully to", backupPath)
	return createRestoreFileFunc(filePath, backupPath), nil
}

// createRestoreFileFunc creates a function for restoring a file from its backup.
// The returned function replaces the file in filePath with the backup in backupPath.
// If there is no file at backupPath (which means there was no file at filePath when BackupFile() was called), then the function deletes the file at filePath.
func createRestoreFileFunc(filePath, backupPath string) func() error {
	return func() error {
		if _, err := os.Stat(backupPath); err != nil {
			if os.IsNotExist(err) {
				err = os.Remove(filePath)
				return errorutils.CheckError(err)
			}
			return errorutils.CheckErrorf(createRestoreErrorPrefix(filePath, backupPath) + err.Error())
		}

		if err := fileutils.MoveFile(backupPath, filePath); err != nil {
			return errorutils.CheckError(err)
		}
		log.Debug("Restored the file", filePath, "successfully")

		return nil
	}
}

func createRestoreErrorPrefix(filePath, backupPath string) string {
	return fmt.Sprintf("An error occurred while restoring the file: %s\n"+
		"To restore the file manually: delete %s and rename the backup file at %s (if exists) to '%s'.\n"+
		"Failure cause: ",
		filePath, filePath, backupPath, filePath)
}
