package project

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFromString(t *testing.T) {
	// Test valid conversions
	tests := []struct {
		input    string
		expected ProjectType
	}{
		{"go", Go},
		{"pip", Pip},
		{"npm", Npm},
		{"pnpm", Pnpm},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := FromString(test.input)
			assert.Equal(t, test.expected, result, "For input %s, expected %v but got %v", test.input, test.expected, result)
		})
	}

	// Test invalid conversion
	result := FromString("InvalidProject")
	assert.Equal(t, ProjectType(-1), result, "Expected -1 for invalid project type")
}
