package commandsummary

import (
	"testing"

	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
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
	StaticMarkdownConfig.setPlatformUrl(testPlatformUrl)
	for _, testCase := range cases {
		t.Run(testCase.testName, func(t *testing.T) {
			StaticMarkdownConfig.setPlatformMajorVersion(testCase.majorVersion)
			artifactUrl, err := GenerateArtifactUrl(fullPath, "")
			assert.NoError(t, err)
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

func TestAddGitHubTrackingToUrl(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		section        string
		envValue       string
		expectedResult string
		expectsError   bool
	}{
		{
			"No GITHUB_WORKFLOW set",
			"https://example.com/path",
			"build",
			"",
			"https://example.com/path",
			false,
		},
		{
			"GITHUB_WORKFLOW set",
			"https://example.com/path",
			"build",
			"workflow123",
			"https://example.com/path?gh_job_id=workflow123&gh_section=build",
			false,
		},
		{
			"Invalid URL",
			":invalid-url",
			"build",
			"workflow123",
			"",
			true,
		},
		{
			"URL with existing query parameters",
			"https://example.com/path?existing_param=value",
			"deploy",
			"workflow123",
			"https://example.com/path?existing_param=value&gh_job_id=workflow123&gh_section=deploy",
			false,
		},
		{
			"GITHUB_WORKFLOW with special characters",
			"https://example.com/path",
			"test",
			"workflow with spaces & special?characters",
			"https://example.com/path?gh_job_id=workflow+with+spaces+%26+special%3Fcharacters&gh_section=test",
			false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Set up the environment variable
			cleanup := testsutils.SetEnvWithCallbackAndAssert(t, "GITHUB_WORKFLOW", test.envValue)
			defer cleanup()

			// Call the function
			result, err := addGitHubTrackingToUrl(test.url, test.section)

			if test.expectsError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedResult, result)
			}
		})
	}
}
