package precheckrunner

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
)

var remoteUrlCheckerTestDir = filepath.Join("testdata", "remoteurlchecker")

func TestRemoteUrlRequest(t *testing.T) {
	// Read mock "GET artifactory/api/repository/nuget-remote" response
	nugetRepo, err := fileutils.ReadFile(filepath.Join(remoteUrlCheckerTestDir, "nuget_repo.json"))
	assert.NoError(t, err)

	// Create RemoteRepositoryCheck test object
	var remoteRepositories []interface{}
	var remoteRepository services.RemoteRepositoryBaseParams
	assert.NoError(t, json.Unmarshal(nugetRepo, &remoteRepository))
	remoteRepositoryCheck := NewRemoteRepositoryCheck(nil, append(remoteRepositories, remoteRepository))

	// Run and verify createRemoteUrlRequest
	remoteUrlRequest, err := remoteRepositoryCheck.createRemoteUrlRequest()
	assert.NoError(t, err)
	assert.Len(t, remoteUrlRequest, 1)
	assert.Equal(t, "nuget-remote", remoteUrlRequest[0].Key)
	assert.Equal(t, "https://www.nuget.org/", remoteUrlRequest[0].Url)
	assert.Equal(t, "nuget", remoteUrlRequest[0].RepoType)
	assert.Equal(t, "admin", remoteUrlRequest[0].Username)
	assert.Equal(t, "password", remoteUrlRequest[0].Password)
}

func TestEmptyRemoteUrlRequest(t *testing.T) {
	// Create RemoteRepositoryCheck test object
	remoteRepositoryCheck := NewRemoteRepositoryCheck(nil, []interface{}{})

	// Run and verify createRemoteUrlRequest
	remoteUrlRequest, err := remoteRepositoryCheck.createRemoteUrlRequest()
	assert.NoError(t, err)
	assert.Empty(t, remoteUrlRequest)
}
