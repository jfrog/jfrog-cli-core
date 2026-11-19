package dependencies

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	httpClient, httpClientDetails, err := CreateHttpClient(serverDetails)
	assert.NoError(t, err)
	assert.NotNil(t, httpClient)
	assert.NotNil(t, httpClientDetails)

	assert.Equal(t, "elmar", httpClientDetails.User)
	assert.Equal(t, "Egghead", httpClientDetails.Password)
}

func TestCopyFileAtomically(t *testing.T) {
	newSource := func(t *testing.T) string {
		src := filepath.Join(t.TempDir(), "src.jar")
		require.NoError(t, os.WriteFile(src, []byte("new content"), 0755))
		require.NoError(t, os.Chmod(src, 0755))
		return src
	}
	tests := []struct {
		name        string
		existing    bool
		missingSrc  bool
		expectError bool
	}{
		{"new destination", false, false, false},
		{"existing destination is replaced", true, false, false},
		{"missing source", false, true, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			src := newSource(t)
			if test.missingSrc {
				src = filepath.Join(t.TempDir(), "missing.jar")
			}
			dst := filepath.Join(t.TempDir(), "dst.jar")
			if test.existing {
				require.NoError(t, os.WriteFile(dst, []byte("old content"), 0600))
			}

			err := copyFileAtomically(src, dst)

			entries, readErr := os.ReadDir(filepath.Dir(dst))
			require.NoError(t, readErr)
			if test.expectError {
				assert.Error(t, err)
				assert.Empty(t, entries, "no destination and no .part file is left")
				return
			}
			require.NoError(t, err)
			require.Len(t, entries, 1, "no .part file is left")
			content, err := os.ReadFile(dst)
			require.NoError(t, err)
			assert.Equal(t, "new content", string(content))
			if runtime.GOOS != "windows" { // Windows has no Unix permission bits.
				info, err := os.Stat(dst)
				require.NoError(t, err)
				assert.Equal(t, os.FileMode(0755), info.Mode().Perm())
			}
		})
	}
}

func TestDownloadDependencyExplodesAnArchive(t *testing.T) {
	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	file, err := zipWriter.Create("dir/file.txt")
	require.NoError(t, err)
	_, err = file.Write([]byte("zipped content"))
	require.NoError(t, err)
	require.NoError(t, zipWriter.Close())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive.Bytes())
	}))
	t.Cleanup(server.Close)
	t.Setenv(coreutils.HomeDir, t.TempDir())
	targetDir := t.TempDir()

	require.NoError(t, DownloadDependency(&config.ServerDetails{ArtifactoryUrl: server.URL + "/"}, "files.zip", filepath.Join(targetDir, "files.zip"), true))

	content, err := os.ReadFile(filepath.Join(targetDir, "dir", "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "zipped content", string(content))
}
