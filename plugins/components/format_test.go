package components

import (
	"strings"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/common/format"
	"github.com/stretchr/testify/assert"
)

func TestGetFormatFlagDescription(t *testing.T) {
	allFormatsJoined := strings.Join(format.OutputFormats, ", ")

	tests := []struct {
		name             string
		supportedFormats []format.OutputFormat
		expectedDesc     string
	}{
		{
			name:         "no supportedFormats — all formats listed",
			expectedDesc: "Defines the output format of the command. Acceptable values are: " + allFormatsJoined,
		},
		{
			name:             "subset of formats — only listed formats appear",
			supportedFormats: []format.OutputFormat{format.Json, format.Sarif},
			expectedDesc:     "Defines the output format of the command. Acceptable values are: json, sarif",
		},
		{
			name:             "single supported format",
			supportedFormats: []format.OutputFormat{format.Table},
			expectedDesc:     "Defines the output format of the command. Acceptable values are: table",
		},
		{
			name:             "order of supportedFormats is preserved",
			supportedFormats: []format.OutputFormat{format.Sarif, format.Json, format.Table},
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

func TestGetFormatFlag(t *testing.T) {
	tests := []struct {
		name             string
		defaultFormat    format.OutputFormat
		supportedFormats []format.OutputFormat
	}{
		{
			name:          "no supportedFormats, table default",
			defaultFormat: format.Table,
		},
		{
			name:          "no supportedFormats, json default",
			defaultFormat: format.Json,
		},
		{
			name:             "subset of formats",
			defaultFormat:    format.Json,
			supportedFormats: []format.OutputFormat{format.Json, format.Sarif},
		},
		{
			name:             "single supported format",
			defaultFormat:    format.Sarif,
			supportedFormats: []format.OutputFormat{format.Sarif},
		},
		{
			name:             "default format is None",
			defaultFormat:    format.None,
			supportedFormats: []format.OutputFormat{format.Json, format.Sarif},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flag := GetFormatFlag(test.supportedFormats, test.defaultFormat)

			assert.Equal(t, format.FlagName, flag.GetName())
			if test.defaultFormat != format.None {
				assert.Equal(t, string(test.defaultFormat), flag.GetDefault())
			} else {
				assert.Empty(t, flag.GetDefault())
			}
			assert.Equal(t, GetFormatFlagDescription(test.supportedFormats), flag.GetDescription())
			assert.NotContains(t, flag.GetDescription(), "[Default:", "default value must not appear in the raw description")

			// Description must list exactly the expected formats.
			expectedFormats := test.supportedFormats
			if len(expectedFormats) == 0 {
				for _, f := range format.OutputFormats {
					expectedFormats = append(expectedFormats, format.OutputFormat(f))
				}
			}
			for _, f := range expectedFormats {
				assert.Contains(t, flag.GetDescription(), string(f))
			}
		})
	}
}
