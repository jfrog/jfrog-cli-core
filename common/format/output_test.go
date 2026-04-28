package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseOutputFormat(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		supportedFormats []OutputFormat
		expectedFormat   OutputFormat
		expectError      bool
	}{
		{
			name:             "exact match — json",
			input:            "json",
			supportedFormats: []OutputFormat{Json, Sarif},
			expectedFormat:   Json,
		},
		{
			name:             "case-insensitive match",
			input:            "JSON",
			supportedFormats: []OutputFormat{Json, Sarif},
			expectedFormat:   Json,
		},
		{
			name:             "mixed case",
			input:            "SaRiF",
			supportedFormats: []OutputFormat{Json, Sarif},
			expectedFormat:   Sarif,
		},
		{
			name:             "unsupported format returns error",
			input:            "table",
			supportedFormats: []OutputFormat{Json, Sarif},
			expectedFormat:   None,
			expectError:      true,
		},
		{
			name:             "empty input returns error",
			input:            "",
			supportedFormats: []OutputFormat{Json, Sarif},
			expectedFormat:   None,
			expectError:      true,
		},
		{
			name:             "unknown value returns error",
			input:            "xml",
			supportedFormats: []OutputFormat{Table, Json, SimpleJson, Sarif, CycloneDx},
			expectedFormat:   None,
			expectError:      true,
		},
		{
			name:             "all formats supported — cyclonedx matches",
			input:            "cyclonedx",
			supportedFormats: []OutputFormat{Table, Json, SimpleJson, Sarif, CycloneDx},
			expectedFormat:   CycloneDx,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := ParseOutputFormat(test.input, test.supportedFormats)
			assert.Equal(t, test.expectedFormat, result)
			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
