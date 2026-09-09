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
		{"conan", Conan},
		{"uv", UV},
		{"cargo", Cargo},
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

// TestCargoStringRoundTrip guards the enum/slice alignment: Cargo must stringify to "cargo"
// and round-trip through FromString. A misaligned append would shift indices and break this.
func TestCargoStringRoundTrip(t *testing.T) {
	assert.Equal(t, "cargo", Cargo.String())
	assert.Equal(t, Cargo, FromString(Cargo.String()))
}
