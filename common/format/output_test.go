package format

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFormatFlagDescription(t *testing.T) {
	allFormatsJoined := strings.Join(OutputFormats, ", ")

	tests := []struct {
		name             string
		supportedFormats []OutputFormat
		expectedDesc     string
	}{
		{
			name:         "no supportedFormats — all formats listed",
			expectedDesc: "Defines the output format of the command. Acceptable values are: " + allFormatsJoined,
		},
		{
			name:             "subset of formats — only listed formats appear",
			supportedFormats: []OutputFormat{Json, Sarif},
			expectedDesc:     "Defines the output format of the command. Acceptable values are: json, sarif",
		},
		{
			name:             "single supported format",
			supportedFormats: []OutputFormat{Table},
			expectedDesc:     "Defines the output format of the command. Acceptable values are: table",
		},
		{
			name:             "order of supportedFormats is preserved",
			supportedFormats: []OutputFormat{Sarif, Json, Table},
			expectedDesc:     "Defines the output format of the command. Acceptable values are: sarif, json, table",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := GetFormatFlagDescription(test.supportedFormats)
			assert.Equal(t, test.expectedDesc, result)
			assert.NotContains(t, result, "[Default:", "description must not contain a default value prefix")
		})
	}
}

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

func TestGetFormatFlag(t *testing.T) {
	tests := []struct {
		name             string
		defaultFormat    OutputFormat
		supportedFormats []OutputFormat
	}{
		{
			name:          "no supportedFormats, table default",
			defaultFormat: Table,
		},
		{
			name:          "no supportedFormats, json default",
			defaultFormat: Json,
		},
		{
			name:             "subset of formats",
			defaultFormat:    Json,
			supportedFormats: []OutputFormat{Json, Sarif},
		},
		{
			name:             "single supported format",
			defaultFormat:    Sarif,
			supportedFormats: []OutputFormat{Sarif},
		},
		{
			name:             "default format is None",
			defaultFormat:    None,
			supportedFormats: []OutputFormat{Json, Sarif},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flag := GetFormatFlag(test.supportedFormats, test.defaultFormat)

			assert.Equal(t, FlagName, flag.GetName())
			if test.defaultFormat != None {
				assert.Equal(t, string(test.defaultFormat), flag.GetDefault())
			} else {
				assert.Empty(t, flag.GetDefault())
			}
			assert.Equal(t, GetFormatFlagDescription(test.supportedFormats), flag.GetDescription())
			assert.NotContains(t, flag.GetDescription(), "[Default:", "default value must not appear in the raw description")

			// Description must list exactly the expected formats.
			expectedFormats := test.supportedFormats
			if len(expectedFormats) == 0 {
				for _, f := range OutputFormats {
					expectedFormats = append(expectedFormats, OutputFormat(f))
				}
			}
			for _, f := range expectedFormats {
				assert.Contains(t, flag.GetDescription(), string(f))
			}
		})
	}
}
