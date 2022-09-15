package utils

import (
	"context"
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
			// Response for CalculateStorageInfo
			w.WriteHeader(http.StatusAccepted)
			calculated = true
		}
	}))
	defer testServer.Close()

	// Create storage info manager
	storageInfoManager, err := NewStorageInfoManager(context.Background(), &config.ServerDetails{ArtifactoryUrl: testServer.URL + "/"})
	assert.NoError(t, err)

	// Calculate and assert storage info
	assert.NoError(t, storageInfoManager.CalculateStorageInfo())
	assert.True(t, calculated)
}

func TestGetStorageInfo(t *testing.T) {
	// Prepare mock server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/storageinfo" {
			// Response for CalculateStorageInfo
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
	storageInfoManager, err := NewStorageInfoManager(context.Background(), &config.ServerDetails{ArtifactoryUrl: testServer.URL + "/"})
	assert.NoError(t, err)

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
			// Response for GetStorageInfo
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
	storageInfo, err := NewStorageInfoManager(context.Background(), &config.ServerDetails{ArtifactoryUrl: testServer.URL + "/"})
	assert.NoError(t, err)

	// Get repo summary of repo-1
	repoSummary, err := storageInfo.GetRepoSummary("repo-1")
	assert.NoError(t, err)
	assert.Equal(t, "repo-1", repoSummary.RepoKey)

	// Get repo summary of non-existed repo
	_, err = storageInfo.GetRepoSummary("not-existed")
	assert.ErrorContains(t, err, "could not find repository 'not-existed' in the repositories summary")
}

func TestConvertStorageSizeStringToBytes(t *testing.T) {
	t.Run("bytes", func(t *testing.T) { assertConvertedStorageSize(t, "2.22 bytes", false, 2.22) })
	t.Run("KB", func(t *testing.T) { assertConvertedStorageSize(t, "3.333 KB", false, 3.333*bytesInKB) })
	t.Run("MB", func(t *testing.T) { assertConvertedStorageSize(t, "4.4444 MB", false, 4.4444*bytesInMB) })
	t.Run("GB", func(t *testing.T) { assertConvertedStorageSize(t, "5.55555 GB", false, 5.55555*bytesInGB) })
	t.Run("TB", func(t *testing.T) { assertConvertedStorageSize(t, "6.666666 TB", false, 6.666666*bytesInTB) })
	t.Run("int", func(t *testing.T) { assertConvertedStorageSize(t, "7 KB", false, 7*bytesInKB) })
	t.Run("size missing", func(t *testing.T) { assertConvertedStorageSize(t, "8", true, -1) })
	t.Run("unexpected size", func(t *testing.T) { assertConvertedStorageSize(t, "8 kb", true, -1) })
	t.Run("too many separators", func(t *testing.T) { assertConvertedStorageSize(t, "8 K B", true, -1) })
}

func assertConvertedStorageSize(t *testing.T, size string, errorExpected bool, expectedSizeBeforeConversion float64) {
	converted, err := convertStorageSizeStringToBytes(size)
	if errorExpected {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	assert.Equal(t, int64(expectedSizeBeforeConversion), converted)
}
