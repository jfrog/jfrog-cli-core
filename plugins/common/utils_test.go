package common

import (
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/common/format"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractOutputFormat(t *testing.T) {
	supported := []format.OutputFormat{format.Table, format.Json, format.Sarif}

	tests := []struct {
		name           string
		flagVal        string
		envVal         string
		expectedFormat format.OutputFormat
		expectError    bool
	}{
		{
			name:           "flag set to supported format",
			flagVal:        "json",
			expectedFormat: format.Json,
		},
		{
			name:           "flag is case-insensitive",
			flagVal:        "SARIF",
			expectedFormat: format.Sarif,
		},
		{
			name:        "flag set to unsupported format",
			flagVal:     "cyclonedx",
			expectError: true,
		},
		{
			name:           "flag empty, env var set to supported format",
			envVal:         "table",
			expectedFormat: format.Table,
		},
		{
			name:        "flag empty, env var set to unsupported format",
			envVal:      "simple-json",
			expectError: true,
		},
		{
			name:        "flag empty, env var empty",
			expectError: true,
		},
		{
			name:           "flag takes precedence over env var",
			flagVal:        "json",
			envVal:         "sarif",
			expectedFormat: format.Json,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.envVal != "" {
				t.Setenv(coreutils.OutputFormat, test.envVal)
			}

			c := &components.Context{}
			if test.flagVal != "" {
				c.AddStringFlag("format", test.flagVal)
			}

			result, err := ExtractOutputFormat(c, supported)
			if test.expectError {
				assert.Error(t, err)
				assert.Equal(t, format.None, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedFormat, result)
			}
		})
	}
}
