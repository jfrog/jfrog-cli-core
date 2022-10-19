package state

import (
	"strconv"
	"time"
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

func getRepoMissingErrorMsg(repoKey string) string {
	return "Could not find repository '" + repoKey + "' in state file. Aborting."
}
