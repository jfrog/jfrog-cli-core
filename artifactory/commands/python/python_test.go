package python

import (
	"github.com/jfrog/build-info-go/utils/pythonutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetPypiRepoUrl(t *testing.T) {
	server := &config.ServerDetails{
		ArtifactoryUrl: "https://server.com/artifactory",
		AccessToken:    "eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJmYWtlXC91c2Vyc1wvdGVzdCJ9.MTIzNDU2Nzg5MA",
	}
	repository := "test-repo"
	url, err := GetPypiRepoUrl(server, repository)
	assert.NoError(t, err)
	assert.Equal(t, "https://test:eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJmYWtlXC91c2Vyc1wvdGVzdCJ9.MTIzNDU2Nzg5MA@server.com/artifactoryapi/pypi/test-repo/simple", url.String())
	server.AccessToken = ""
	server.User = "user"
	server.Password = "password"
	url, err = GetPypiRepoUrl(server, repository)
	assert.NoError(t, err)
	assert.Equal(t, "https://user:password@server.com/artifactoryapi/pypi/test-repo/simple", url.String())
}

func TestGetPypiRemoteRegistryFlag(t *testing.T) {
	assert.Equal(t, "-i", GetPypiRemoteRegistryFlag(pythonutils.Pip))
	assert.Equal(t, "--pypi-mirror", GetPypiRemoteRegistryFlag(pythonutils.Pipenv))
}
