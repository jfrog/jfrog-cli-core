package goutils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/url"
	"testing"
)

func TestGetArtifactoryRemoteRepoUrl(t *testing.T) {
	server := &config.ServerDetails{
		ArtifactoryUrl: "https://server.com/artifactory",
		AccessToken:    "eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJmYWtlXC91c2Vyc1wvdGVzdCJ9.MTIzNDU2Nzg5MA",
	}
	repoName := "test-repo"
	repoUrl, err := GetArtifactoryRemoteRepoUrl(server, repoName, GoProxyUrlParams{})
	assert.NoError(t, err)
	assert.Equal(t, "https://test:eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJmYWtlXC91c2Vyc1wvdGVzdCJ9.MTIzNDU2Nzg5MA@server.com/artifactory/api/go/test-repo", repoUrl)
}

func TestGetArtifactoryApiUrl(t *testing.T) {
	details := auth.NewArtifactoryDetails()
	details.SetUrl("https://test.com/artifactory/")

	// Test username and password
	details.SetUser("frog")
	details.SetPassword("passfrog")
	url, err := getArtifactoryApiUrl("test-repo", details, GoProxyUrlParams{})
	assert.NoError(t, err)
	assert.Equal(t, "https://frog:passfrog@test.com/artifactory/api/go/test-repo", url)

	// Test username and password with EndpointPrefix and direct
	details.SetUser("frog")
	details.SetPassword("passfrog")
	url, err = getArtifactoryApiUrl("test-repo", details, GoProxyUrlParams{EndpointPrefix: "test", Direct: true})
	assert.NoError(t, err)
	assert.Equal(t, "https://frog:passfrog@test.com/artifactory/test/api/go/test-repo|direct", url)

	// Test access token
	// Set fake access token with username "test"
	details.SetUser("")
	details.SetAccessToken("eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJmYWtlXC91c2Vyc1wvdGVzdCJ9.MTIzNDU2Nzg5MA")
	url, err = getArtifactoryApiUrl("test-repo", details, GoProxyUrlParams{})
	assert.NoError(t, err)
	assert.Equal(t, "https://test:eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJmYWtlXC91c2Vyc1wvdGVzdCJ9.MTIzNDU2Nzg5MA@test.com/artifactory/api/go/test-repo", url)

	// Test access token with username
	// Set fake access token with username "test"
	// Expect username to be "frog"
	details.SetUser("frog")
	details.SetAccessToken("eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJmYWtlXC91c2Vyc1wvdGVzdCJ9.MTIzNDU2Nzg5MA")
	url, err = getArtifactoryApiUrl("test-repo", details, GoProxyUrlParams{})
	assert.NoError(t, err)
	assert.Equal(t, "https://frog:eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJmYWtlXC91c2Vyc1wvdGVzdCJ9.MTIzNDU2Nzg5MA@test.com/artifactory/api/go/test-repo", url)
}

func TestGoProxyUrlParams_BuildUrl(t *testing.T) {
	tests := []struct {
		name           string
		RepoName       string
		Direct         bool
		EndpointPrefix string
		ExpectedUrl    string
	}{
		{
			name:        "Url Without direct or Prefix",
			RepoName:    "go",
			ExpectedUrl: "https://test/api/go/go",
		},
		{
			name:        "Url With direct",
			RepoName:    "go",
			Direct:      true,
			ExpectedUrl: "https://test/api/go/go|direct",
		},
		{
			name:           "Url With Prefix",
			RepoName:       "go",
			EndpointPrefix: "prefix",
			ExpectedUrl:    "https://test/prefix/api/go/go",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remoteUrl, err := url.Parse("https://test")
			require.NoError(t, err)
			gdu := &GoProxyUrlParams{
				Direct:         tt.Direct,
				EndpointPrefix: tt.EndpointPrefix,
			}
			assert.Equalf(t, tt.ExpectedUrl, gdu.BuildUrl(remoteUrl, tt.RepoName), "BuildUrl(%v, %v)", remoteUrl, tt.RepoName)
		})
	}
}
