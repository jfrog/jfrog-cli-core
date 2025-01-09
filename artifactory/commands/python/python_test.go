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
	testCases := []struct {
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

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			url, _, _, err := GetPypiRepoUrlWithCredentials(&config.ServerDetails{}, "test", testCase.curationCmd)
			require.NoError(t, err)
			assert.Equal(t, testCase.curationCmd, strings.Contains(url.Path, coreutils.CurationPassThroughApi))
		})
	}
}
