package python

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestGetPypiRepoUrlWithCredentials(t *testing.T) {
	tests := []struct {
		name        string
		curationCmd bool
	}{
		{
			name:        "test curation command true",
			curationCmd: true,
		},
		{
			name:        "test curation command false",
			curationCmd: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, _, _, err := GetPypiRepoUrlWithCredentials(&config.ServerDetails{}, "test", tt.curationCmd)
			require.NoError(t, err)
			assert.Equal(t, tt.curationCmd, strings.Contains(url.Path, coreutils.CurationPassThroughApi))
		})
	}
}
