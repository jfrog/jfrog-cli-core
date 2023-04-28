package utils

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSplitRepoAndServerId(t *testing.T) {
	// Test cases
	tests := []struct {
		serverAndRepo string
		remoteEnv     string
		serverID      string
		repoName      string
		err           error
	}{
		{
			serverAndRepo: "myServer/myRepo",
			remoteEnv:     releasesRemoteEnv,
			serverID:      "myServer",
			repoName:      "myRepo",
			err:           nil,
		},
		{
			serverAndRepo: "/myRepo",
			remoteEnv:     ExtractorsRemoteEnv,
			serverID:      "",
			repoName:      "",
			err:           fmt.Errorf("'%s' environment variable is '/myRepo' but should be '<server ID>/<repo name>'", ExtractorsRemoteEnv),
		},
		{
			serverAndRepo: "myServer/",
			remoteEnv:     releasesRemoteEnv,
			serverID:      "",
			repoName:      "",
			err:           fmt.Errorf("'%s' environment variable is 'myServer/' but should be '<server ID>/<repo name>'", releasesRemoteEnv),
		},
		{
			serverAndRepo: "",
			remoteEnv:     releasesRemoteEnv,
			serverID:      "",
			repoName:      "",
			err:           fmt.Errorf("'%s' environment variable is '' but should be '<server ID>/<repo name>'", releasesRemoteEnv),
		},
	}
	for _, test := range tests {
		serverID, repoName, err := splitRepoAndServerId(test.serverAndRepo, test.remoteEnv)
		if err != nil {
			assert.Equal(t, test.err.Error(), err.Error())
		}
		// Assert the results
		assert.Equal(t, test.serverID, serverID)
		assert.Equal(t, test.repoName, repoName)
	}
}

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
			remoteEnv:    ExtractorsRemoteEnv,
			downloadPath: "path/to/file",
			expectedPath: "my-repo/path/to/file",
		},
		{
			repoName:     "my-repo",
			remoteEnv:    releasesRemoteEnv,
			downloadPath: "path/to/file",
			expectedPath: "my-repo/artifactory/oss-release-local/path/to/file",
		},
	}

	// Execute the tests
	for _, test := range tests {
		actualPath := getFullRemoteRepoPath(test.repoName, test.remoteEnv, test.downloadPath)
		assert.Equal(t, test.expectedPath, actualPath)
	}
}
