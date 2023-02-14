package golang

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	goutils "github.com/jfrog/jfrog-cli-core/v2/utils/golang"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildPackageVersionRequest(t *testing.T) {
	tests := []struct {
		packageName     string
		branchName      string
		expectedRequest string
	}{
		{"github.com/jfrog/jfrog-cli", "", "github.com/jfrog/jfrog-cli/@v/latest.info"},
		{"github.com/jfrog/jfrog-cli", "dev", "github.com/jfrog/jfrog-cli/@v/dev.info"},
		{"github.com/jfrog/jfrog-cli", "v1.0.7", "github.com/jfrog/jfrog-cli/@v/v1.0.7.info"},
	}
	for _, test := range tests {
		t.Run(test.expectedRequest, func(t *testing.T) {
			versionRequest := buildPackageVersionRequest(test.packageName, test.branchName)
			if versionRequest != test.expectedRequest {
				t.Error("Failed to build package version request. The version request is", versionRequest, " but it is expected to be", test.expectedRequest)
			}
		})
	}
}

func TestGetPackageFilesPath(t *testing.T) {
	packageCachePath, err := goutils.GetGoModCachePath()
	assert.NoError(t, err)
	packageName := "github.com/golang/mock/mockgen"
	version := "v1.4.1"
	expectedPackagePath := filepath.Join(packageCachePath, "github.com/golang/mock@"+version)
	err = os.MkdirAll(expectedPackagePath, os.ModePerm)
	assert.NoError(t, err)
	defer testsutils.RemoveAllAndAssert(t, expectedPackagePath)
	actualPackagePath, err := getFileSystemPackagePath(packageCachePath, packageName, version)
	assert.NoError(t, err)
	assert.Equal(t, expectedPackagePath, actualPackagePath)
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

func TestGetGoRepoUrl(t *testing.T) {
	server := &config.ServerDetails{
		ArtifactoryUrl: "https://server.com/artifactory",
		AccessToken:    "eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJmYWtlXC91c2Vyc1wvdGVzdCJ9.MTIzNDU2Nzg5MA",
	}
	repoName := "test-repo"
	repoUrl, err := GetGoRepoUrl(server, repoName)
	assert.NoError(t, err)
	assert.Equal(t, "https://test:eyJ0eXAiOiJKV1QifQ.eyJzdWIiOiJmYWtlXC91c2Vyc1wvdGVzdCJ9.MTIzNDU2Nzg5MA@server.com/artifactoryapi/go/test-repo", repoUrl)
}
