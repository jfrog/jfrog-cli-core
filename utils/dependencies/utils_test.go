package dependencies

import (
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
)

func TestGetFullRemoteRepoPath(t *testing.T) {
	// Test cases
	tests := []struct {
		repoName     string
		remoteEnv    string
		downloadPath string
		expectedPath string
	}{
		{
			repoName:     "my-repo",
			remoteEnv:    coreutils.DeprecatedExtractorsRemoteEnv,
			downloadPath: "path/to/file",
			expectedPath: "my-repo/path/to/file",
		},
		{
			repoName:     "my-repo",
			remoteEnv:    coreutils.ReleasesRemoteEnv,
			downloadPath: "path/to/file",
			expectedPath: "my-repo/artifactory/oss-release-local/path/to/file",
		},
	}

	// Execute the tests
	for _, test := range tests {
		actualPath := getFullExtractorsPathInArtifactory(test.repoName, test.remoteEnv, test.downloadPath)
		assert.Equal(t, test.expectedPath, actualPath)
	}
}

func TestCreateHttpClient(t *testing.T) {
	serverDetails := &config.ServerDetails{
		Url:      "https://acme.jfrog.io",
		User:     "elmar",
		Password: "Egghead",
	}
	httpClient, httpClientDetails, err := createHttpClient(serverDetails)
	assert.NoError(t, err)
	assert.NotNil(t, httpClient)
	assert.NotNil(t, httpClientDetails)

	assert.Equal(t, "elmar", httpClientDetails.User)
	assert.Equal(t, "Egghead", httpClientDetails.Password)
}
