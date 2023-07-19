package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecondsToLiteralTime(t *testing.T) {
	testCases := []struct {
		name          string
		expected      string
		secsToConvert int64
		prefix        string
	}{
		{"plural days and plural hours", "About 11 days and 13 hours", getTimeInSecs(11, 13, 3, 7), "About "},
		{"plural days and singular hour", "5 days and 1 hour", getTimeInSecs(5, 1, 2, 0), ""},
		{"plural days", " 3 days", getTimeInSecs(3, 0, 4, 0), " "},
		{"singular day and plural hours", "About 1 day and 2 hours", getTimeInSecs(1, 2, 6, 6), "About "},
		{"singular day and singular hour", "About 1 day and 1 hour", getTimeInSecs(1, 1, 6, 6), "About "},
		{"singular day", "About 1 day", getTimeInSecs(1, 0, 4, 0), "About "},
		{"plural hours and plural minutes", "About 11 hours and 13 minutes", getTimeInSecs(0, 11, 13, 0), "About "},
		{"plural hours and singular minute", "About 5 hours and 1 minute", getTimeInSecs(0, 5, 1, 6), "About "},
		{"plural hours", "About 3 hours", getTimeInSecs(0, 3, 0, 3), "About "},
		{"singular hours and plural minutes", "About 1 hour and 13 minutes", getTimeInSecs(0, 1, 13, 0), "About "},
		{"singular hours and singular minute", "About 1 hour and 1 minute", getTimeInSecs(0, 1, 1, 6), "About "},
		{"singular hour", "About 1 hour", getTimeInSecs(0, 1, 0, 3), "About "},
		{"plural minutes", "About 10 minutes", getTimeInSecs(0, 0, 10, 3), "About "},
		{"singular minute", "About 1 minute", getTimeInSecs(0, 0, 1, 3), "About "},
		{"seconds", "Less than a minute", getTimeInSecs(0, 0, 0, 3), "About "},
		{"seconds no prefix", "Less than a minute", getTimeInSecs(0, 0, 0, 4), ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, SecondsToLiteralTime(testCase.secsToConvert, testCase.prefix))
		})
	}
}

func getTimeInSecs(days, hours, minutes, seconds int64) int64 {
	return 86400*days + 3600*hours + 60*minutes + seconds
}
