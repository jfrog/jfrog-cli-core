package coreutils

import (
	"github.com/magiconair/properties/assert"
	"reflect"
	"testing"
)

func TestCountLinesInCell(t *testing.T) {
	tests := []struct {
		content               string
		maxWidth              int
		expectedNumberOfLines int
	}{
		{
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
			0,
			1,
		},
		{
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
			15,
			9,
		},
		{
			"Lorem\n" +
				"ipsum dolor sit amet,\n" +
				"consectetur adipiscing elit,\n" +
				"sed do eiusmod tempor incididunt\n" +
				"ut labore et dolore magna aliqua.",
			15,
			11,
		},
	}
	for _, test := range tests {
		actualNumberOfLines := countLinesInCell(test.content, test.maxWidth)
		assert.Equal(t, test.expectedNumberOfLines, actualNumberOfLines)
	}
}

func TestIsColumnEmpty_NotEmptyScenario(t *testing.T) {
	vulnerabilityRows := []struct {
		Severity         string
		Applicable       string
		SeverityNumValue int
	}{
		{Severity: "Medium", Applicable: "", SeverityNumValue: 2},
		{Severity: "High", Applicable: "Applicable", SeverityNumValue: 3},
		{Severity: "High", Applicable: "", SeverityNumValue: 3},
		{Severity: "Low", Applicable: "", SeverityNumValue: 1},
	}
	rows := reflect.ValueOf(vulnerabilityRows)

	columnEmpty := isColumnEmpty(rows, 1)
	assert.Equal(t, false, columnEmpty)
}

func TestIsColumnEmpty_EmptyScenario(t *testing.T) {
	vulnerabilityRows := []struct {
		Severity         string
		Applicable       string
		SeverityNumValue int
	}{
		{Severity: "Medium", Applicable: "", SeverityNumValue: 2},
		{Severity: "High", Applicable: "", SeverityNumValue: 3},
		{Severity: "High", Applicable: "", SeverityNumValue: 3},
		{Severity: "Low", Applicable: "", SeverityNumValue: 1},
	}
	rows := reflect.ValueOf(vulnerabilityRows)

	columnEmpty := isColumnEmpty(rows, 1)
	assert.Equal(t, true, columnEmpty)
}
