package goutils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetArtifactoryRemoteRepoUrl(t *testing.T) {
	server := &config.ServerDetails{
		ArtifactoryUrl: "https://server.com/artifactory",
		AccessToken:    "eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJmYWtlXC91c2Vyc1wvdGVzdCJ9.MTIzNDU2Nzg5MA",
	}
	repoName := "test-repo"
	repoUrl, err := GetArtifactoryRemoteRepoUrl(server, repoName)
	assert.NoError(t, err)
	assert.Equal(t, "https://test:eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJmYWtlXC91c2Vyc1wvdGVzdCJ9.MTIzNDU2Nzg5MA@server.com/artifactoryapi/go/test-repo", repoUrl)
}

func TestGetArtifactoryApiUrl(t *testing.T) {
	details := auth.NewArtifactoryDetails()
	details.SetUrl("https://test.com/artifactory/")

	// Test username and password
	details.SetUser("frog")
	details.SetPassword("passfrog")
	url, err := getArtifactoryApiUrl("test-repo", details)
	assert.NoError(t, err)
	assert.Equal(t, "https://frog:passfrog@test.com/artifactory/api/go/test-repo", url)

	// Test access token
	// Set fake access token with username "test"
	details.SetUser("")
	details.SetAccessToken("eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJmYWtlXC91c2Vyc1wvdGVzdCJ9.MTIzNDU2Nzg5MA")
	url, err = getArtifactoryApiUrl("test-repo", details)
	assert.NoError(t, err)
	assert.Equal(t, "https://test:eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJmYWtlXC91c2Vyc1wvdGVzdCJ9.MTIzNDU2Nzg5MA@test.com/artifactory/api/go/test-repo", url)

	// Test access token with username
	// Set fake access token with username "test"
	// Expect username to be "frog"
	details.SetUser("frog")
	details.SetAccessToken("eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJmYWtlXC91c2Vyc1wvdGVzdCJ9.MTIzNDU2Nzg5MA")
	url, err = getArtifactoryApiUrl("test-repo", details)
	assert.NoError(t, err)
	assert.Equal(t, "https://frog:eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJmYWtlXC91c2Vyc1wvdGVzdCJ9.MTIzNDU2Nzg5MA@test.com/artifactory/api/go/test-repo", url)
}
