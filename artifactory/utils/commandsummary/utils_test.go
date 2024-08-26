package commandsummary

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

const (
	testPlatformUrl = "https://myplatform.com/"
	fullPath        = "repo/path/file"
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
	setPlatformUrl(testPlatformUrl)
	for _, testCase := range cases {
		t.Run(testCase.testName, func(t *testing.T) {
			setPlatformMajorVersion(testCase.majorVersion)
			artifactUrl := GenerateArtifactUrl(fullPath)
			assert.Equal(t, testCase.expected, artifactUrl)
		})
	}
}

func TestFileNameToSha1(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"file1", "60b27f004e454aca81b0480209cce5081ec52390"},
		{"file2", "cb99b709a1978bd205ab9dfd4c5aaa1fc91c7523"},
	}

	for _, test := range tests {
		hash := fileNameToSha1(test.input)
		assert.Equal(t, test.expected, hash)
	}
}
