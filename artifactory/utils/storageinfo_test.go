package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	clientUtils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/stretchr/testify/assert"
)

func TestCalculateStorageInfo(t *testing.T) {
	calculated := false
	// Prepare mock server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/storageinfo/calculate" {
			// Reponse for CalculateStorageInfo
			w.WriteHeader(http.StatusAccepted)
			calculated = true
		}
	}))
	defer testServer.Close()

	// Create storage info manager
	storageInfoManager := NewStorageInfoManager(&config.ServerDetails{ArtifactoryUrl: testServer.URL + "/"})

	// Calculate and assert storage info
	err := storageInfoManager.CalculateStorageInfo()
	assert.NoError(t, err)
	assert.True(t, calculated)
}

func TestGetStorageInfo(t *testing.T) {
	// Prepare mock server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/storageinfo" {
			// Reponse for CalculateStorageInfo
			w.WriteHeader(http.StatusOK)
			response := &clientUtils.StorageInfo{RepositoriesSummaryList: []clientUtils.RepositorySummary{{RepoKey: "repo-1"}}}
			body, err := json.Marshal(response)
			assert.NoError(t, err)
			_, err = w.Write(body)
			assert.NoError(t, err)
		}
	}))
	defer testServer.Close()

	// Create storage info manager
	storageInfoManager := NewStorageInfoManager(&config.ServerDetails{ArtifactoryUrl: testServer.URL + "/"})

	// Get and assert storage info
	storageInfo, err := storageInfoManager.GetStorageInfo()
	assert.NoError(t, err)
	assert.NotNil(t, storageInfo)
	assert.Equal(t, "repo-1", storageInfo.RepositoriesSummaryList[0].RepoKey)
}

func TestGetSourceRepoSummary(t *testing.T) {
	getRepoSummaryPollingInterval = 10 * time.Millisecond
	getRepoSummaryPollingTimeout = 30 * time.Millisecond

	// Prepare mock server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/storageinfo" {
			// Reponse for GetStorageInfo
			w.WriteHeader(http.StatusOK)
			response := &clientUtils.StorageInfo{RepositoriesSummaryList: []clientUtils.RepositorySummary{{RepoKey: "repo-1"}, {RepoKey: "repo-2"}}}
			bytes, err := json.Marshal(response)
			assert.NoError(t, err)
			_, err = w.Write(bytes)
			assert.NoError(t, err)
		}
	}))
	defer testServer.Close()

	// Create storage info manager
	storageInfo := NewStorageInfoManager(&config.ServerDetails{ArtifactoryUrl: testServer.URL + "/"})

	// Get repo summary of repo-1
	repoSummary, err := storageInfo.GetRepoSummary("repo-1")
	assert.NoError(t, err)
	assert.Equal(t, "repo-1", repoSummary.RepoKey)

	// Get repo summary of non-existed repo
	_, err = storageInfo.GetRepoSummary("not-existed")
	assert.ErrorContains(t, err, "could not find repository 'not-existed' in the repositories summary")
}
