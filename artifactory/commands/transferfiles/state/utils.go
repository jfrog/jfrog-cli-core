package state

import (
	"fmt"
	"strconv"
	"time"
)

const (
	secondsInMinute = 60
	secondsInHour   = 60 * secondsInMinute
	secondsInDay    = 24 * secondsInHour
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

// secondsToLiteralTime converts a number of seconds to an easy-to-read string.
// Prefix is not taken into account if the time is less than a minute.
func secondsToLiteralTime(secondsToConvert int64, prefix string) string {
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
