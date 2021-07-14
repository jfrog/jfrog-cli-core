package coreutils

import (
	"github.com/magiconair/properties/assert"
	"testing"
)

func TestCountLinesInCell(t *testing.T) {
	tests := []struct {
		content                string
		maxWidth               int
		expenctedNumberOfLines int
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
		assert.Equal(t, test.expenctedNumberOfLines, actualNumberOfLines)
	}
}
