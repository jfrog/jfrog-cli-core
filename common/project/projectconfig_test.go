package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromString(t *testing.T) {
	// Test valid conversions
	testCases := []struct {
		input    string
		expected ProjectType
	}{
		{"go", Go},
		{"pip", Pip},
		{"npm", Npm},
		{"pnpm", Pnpm},
		{"ruby", Ruby},
	}

	for _, testCase := range testCases {
		t.Run(testCase.input, func(t *testing.T) {
			result := FromString(testCase.input)
			assert.Equal(t, testCase.expected, result)
		})
	}

	// Test invalid conversion
	result := FromString("InvalidProject")
	assert.Equal(t, ProjectType(-1), result)
}
