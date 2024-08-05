package commandssummaries

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

const (
	platformUrl = "https://myplatform.com/"
	fullPath    = "repo/path/file"
)

func TestGenerateArtifactUrl(t *testing.T) {
	cases := []struct {
		testName     string
		projectKey   string
		majorVersion int
		expected     string
	}{
		{"artifactory 7 without project", "", 7, "https://myplatform.com/ui/repos/tree/General/repo/path/file?clearFilter=true"},
		{"artifactory 7 with project", "proj", 7, "https://myplatform.com/ui/repos/tree/General/repo/path/file?clearFilter=true"},
		{"artifactory 6 without project", "", 6, "https://myplatform.com/artifactory/webapp/#/artifacts/browse/tree/General/repo/path/file"},
	}

	for _, testCase := range cases {
		t.Run(testCase.testName, func(t *testing.T) {
			artifactUrl := generateArtifactUrl(platformUrl, fullPath, testCase.majorVersion)
			assert.Equal(t, testCase.expected, artifactUrl)
		})
	}
}
