package state

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

const (
	secondsInMinute = 60
	secondsInHour   = 60 * secondsInMinute
	secondsInDay    = 24 * secondsInHour

	oldTransferDirectoryStructureErrorFormat = `unsupported transfer directory structure found.
This structure was created by a previous run of the transfer-files command, but is no longer supported by this JFrog CLI version.
You may either downgrade JFrog CLI to the version that was used before, or remove the transfer directory which is located under the JFrog CLI home directory (%s).

Note: Deleting the transfer directory will remove all your transfer history, which means the transfer will start from scratch`
)

func ConvertTimeToRFC3339(timeToConvert time.Time) string {
	return timeToConvert.Format(time.RFC3339)
}

func ConvertRFC3339ToTime(timeToConvert string) (time.Time, error) {
	return time.Parse(time.RFC3339, timeToConvert)
}

func ConvertTimeToEpochMilliseconds(timeToConvert time.Time) string {
	return strconv.FormatInt(timeToConvert.UnixMilli(), 10)
}

// SecondsToLiteralTime converts a number of seconds to an easy-to-read string.
// Prefix is not taken into account if the time is less than a minute.
func SecondsToLiteralTime(secondsToConvert int64, prefix string) string {
	daysTime := secondsToConvert / secondsInDay
	daysTimeInSecs := daysTime * secondsInDay
	hoursTime := (secondsToConvert - daysTimeInSecs) / secondsInHour
	if daysTime >= 1 {
		return getTimeAmountWithRemainder(daysTime, hoursTime, day, hour, prefix)
	}

	hoursTimeInSecs := hoursTime * secondsInHour
	minutesTime := (secondsToConvert - hoursTimeInSecs) / secondsInMinute
	if hoursTime >= 1 {
		return getTimeAmountWithRemainder(hoursTime, minutesTime, hour, minute, prefix)
	}

	if minutesTime >= 1 {
		return getTimeAmountWithRemainder(minutesTime, 0, minute, "", prefix)
	}
	return "Less than a minute"
}

// Get the time amount as string, with the remainder added only if it is non-zero.
// For example "About 2 hours and 1 minute"
func getTimeAmountWithRemainder(mainAmount, remainderAmount int64, mainType, remainderType timeTypeSingular, prefix string) string {
	timeStr := prefix + getTimeSingularOrPlural(mainAmount, mainType)
	if remainderAmount > 0 {
		timeStr += " and " + getTimeSingularOrPlural(remainderAmount, remainderType)
	}
	return timeStr
}

// Returns the time amount followed by its type, with 's' for plural if needed.
// For example '1 hour' or '2 hours'.
func getTimeSingularOrPlural(timeAmount int64, timeType timeTypeSingular) string {
	result := fmt.Sprintf("%d %s", timeAmount, timeType)
	if timeAmount > 1 {
		result += "s"
	}
	return result
}

func GetRepositoryTransferDir(repoKey string) (string, error) {
	reposDir, err := coreutils.GetJfrogTransferRepositoriesDir()
	if err != nil {
		return "", err
	}
	repoHash, err := getRepositoryHash(repoKey)
	if err != nil {
		return "", err
	}

	repoDir := filepath.Join(reposDir, repoHash)
	err = fileutils.CreateDirIfNotExist(repoDir)
	if err != nil {
		return "", err
	}

	return repoDir, nil
}

func getRepositoryHash(repoKey string) (string, error) {
	checksumInfo, err := utils.CalcChecksums(strings.NewReader(repoKey), utils.SHA1)
	if err = errorutils.CheckError(err); err != nil {
		return "", err
	}
	return checksumInfo[utils.SHA1], nil
}

func GetJfrogTransferRepoSubDir(repoKey, subDirName string) (string, error) {
	transferDir, err := GetRepositoryTransferDir(repoKey)
	if err != nil {
		return "", err
	}
	return filepath.Join(transferDir, subDirName), nil
}

func GetOldTransferDirectoryStructureError() error {
	transferDir, err := coreutils.GetJfrogTransferDir()
	if err != nil {
		return err
	}
	return errorutils.CheckErrorf(oldTransferDirectoryStructureErrorFormat, transferDir)
}
