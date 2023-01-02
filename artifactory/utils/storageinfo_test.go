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
	testServer, storageInfoManager := mockGetStorageInfoAndInitManager(t, []clientUtils.RepositorySummary{{RepoKey: "repo-1"}})
	defer testServer.Close()

	// Get and assert storage info
	storageInfo, err := storageInfoManager.GetStorageInfo()
	assert.NoError(t, err)
	assert.NotNil(t, storageInfo)
	assert.Equal(t, "repo-1", storageInfo.RepositoriesSummaryList[0].RepoKey)
}

func TestGetSourceRepoSummary(t *testing.T) {
	// Prepare mock server
	testServer, storageInfoManager := mockGetStorageInfoAndInitManager(t, []clientUtils.RepositorySummary{{RepoKey: "repo-1"}, {RepoKey: "repo-2"}})
	defer testServer.Close()

	// Get repo summary of repo-1
	repoSummary, err := storageInfoManager.GetRepoSummary("repo-1")
	assert.NoError(t, err)
	assert.Equal(t, "repo-1", repoSummary.RepoKey)

	// Get repo summary of non-existed repo
	_, err = storageInfoManager.GetRepoSummary("not-existed")
	assert.ErrorContains(t, err, "could not find repository 'not-existed' in the repositories summary")
}

func TestConvertStorageSizeStringToBytes(t *testing.T) {
	convertStorageSizeStringToBytesCases := []struct {
		name                         string
		size                         string
		errorExpected                bool
		expectedSizeBeforeConversion float64
	}{
		{"bytes", "2.22 bytes", false, 2.22},
		{"KB", "3.333 KB", false, 3.333 * float64(bytesInKB)},
		{"KB with comma", "1,004.64 KB", false, 1004.64 * float64(bytesInKB)},
		{"MB", "4.4444 MB", false, 4.4444 * float64(bytesInMB)},
		{"GB", "5.55555 GB", false, 5.55555 * float64(bytesInGB)},
		{"TB", "6.666666 TB", false, 6.666666 * float64(bytesInTB)},
		{"int", "7 KB", false, 7 * float64(bytesInKB)},
		{"size missing", "8", true, -1},
		{"unexpected size", "8 kb", true, -1},
		{"too many separators", "8 K B", true, -1},
	}
	for _, testCase := range convertStorageSizeStringToBytesCases {
		t.Run(testCase.name, func(t *testing.T) {
			assertConvertedStorageSize(t, testCase.size, testCase.errorExpected, testCase.expectedSizeBeforeConversion)
		})
	}
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

func TestGetReposTotalSize(t *testing.T) {
	getRepoSummaryPollingInterval = 10 * time.Millisecond
	getRepoSummaryPollingTimeout = 30 * time.Millisecond

	repositoriesSummaryList := []clientUtils.RepositorySummary{
		{RepoKey: "repo-1", UsedSpaceInBytes: "12345", FilesCount: "3"},
		{RepoKey: "repo-2", UsedSpace: "678 bytes", FilesCount: "4"},
	}
	// Prepare mock server.
	firstRequest := true
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// In order to test the polling, on the first request we return only one repo, and on the second both of them.
		// If the polling does not work properly, we should see a wrong total returned.
		if firstRequest {
			firstRequest = false
			getStorageInfoResponse(t, w, r, repositoriesSummaryList[0:1])
		} else {
			getStorageInfoResponse(t, w, r, repositoriesSummaryList)
		}
	}))
	defer testServer.Close()

	// Create storage info manager
	storageInfoManager, err := NewStorageInfoManager(context.Background(), &config.ServerDetails{ArtifactoryUrl: testServer.URL + "/"})
	assert.NoError(t, err)

	// Get the total size of the two repos.
	totalSize, totalFiles, err := storageInfoManager.GetReposTotalSizeAndFiles("repo-1", "repo-2")
	assert.NoError(t, err)
	assert.Equal(t, int64(13023), totalSize)
	assert.Equal(t, int64(7), totalFiles)

	// Assert error is returned due to the missing repository.
	_, _, err = storageInfoManager.GetReposTotalSizeAndFiles("repo-1", "repo-2", "repo-3")
	assert.EqualError(t, err, storageInfoRepoMissingError)
}

func mockGetStorageInfoAndInitManager(t *testing.T, repositoriesSummaryList []clientUtils.RepositorySummary) (*httptest.Server, *StorageInfoManager) {
	getRepoSummaryPollingInterval = 10 * time.Millisecond
	getRepoSummaryPollingTimeout = 30 * time.Millisecond

	// Prepare mock server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		getStorageInfoResponse(t, w, r, repositoriesSummaryList)
	}))

	// Create storage info manager
	storageInfoManager, err := NewStorageInfoManager(context.Background(), &config.ServerDetails{ArtifactoryUrl: testServer.URL + "/"})
	assert.NoError(t, err)
	return testServer, storageInfoManager
}

func getStorageInfoResponse(t *testing.T, w http.ResponseWriter, r *http.Request, repositoriesSummaryList []clientUtils.RepositorySummary) {
	if r.RequestURI == "/api/storageinfo" {
		// Response for CalculateStorageInfo
		w.WriteHeader(http.StatusOK)
		response := &clientUtils.StorageInfo{RepositoriesSummaryList: repositoriesSummaryList}
		body, err := json.Marshal(response)
		assert.NoError(t, err)
		_, err = w.Write(body)
		assert.NoError(t, err)
	}
}

func TestConvertIntToStorageSizeString(t *testing.T) {
	tests := []struct {
		num    int
		output string
	}{
		{12546, "12.3KB"},
		{148576, "145.1KB"},
		{2587985, "2.5MB"},
		{12896547, "12.3MB"},
		{12896547785, "12.0GB"},
		{5248965785422365, "4773.9TB"},
	}

	for _, test := range tests {
		assert.Equal(t, test.output, ConvertIntToStorageSizeString(int64(test.num)))
	}
}
