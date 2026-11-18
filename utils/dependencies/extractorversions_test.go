package dependencies

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	biutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testMetadata = `<?xml version="1.0" encoding="UTF-8"?>
<metadata modelVersion="1.1.0">
  <versioning>
    <versions>
      <version>2.43.6</version>
      <version>2.43.9</version>
      <version>2.44.0</version>
      <version>2.44.2</version>
      <version>2.44.2</version>
      <version>3.0.0</version>
      <version>2.45.0-SNAPSHOT</version>
    </versions>
  </versioning>
</metadata>`

// metadataServer serves a maven-metadata.xml on one path only, and records the paths requested.
func metadataServer(t *testing.T, servedPath string, delay time.Duration) (server *httptest.Server, requestedPaths func() []string) {
	var mutex sync.Mutex
	var paths []string
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		paths = append(paths, r.URL.Path)
		mutex.Unlock()
		time.Sleep(delay)
		if r.URL.Path != servedPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(testMetadata))
	}))
	t.Cleanup(server.Close)
	return server, func() []string {
		mutex.Lock()
		defer mutex.Unlock()
		return append([]string(nil), paths...)
	}
}

// clearExtractorEnv makes the test independent of the variables of the developer's shell.
func clearExtractorEnv(t *testing.T) {
	for _, env := range []string{coreutils.MavenExtractorVersionEnv, coreutils.GradleExtractorVersionEnv, coreutils.ReleasesRemoteEnv, coreutils.DeprecatedExtractorsRemoteEnv} {
		t.Setenv(env, "")
	}
}

func configureTestServer(t *testing.T, serverUrl string) {
	clearExtractorEnv(t)
	t.Setenv(coreutils.HomeDir, t.TempDir())
	t.Setenv(coreutils.DependenciesDir, t.TempDir())
	require.NoError(t, config.SaveServersConf([]*config.ServerDetails{{ServerId: "test", ArtifactoryUrl: serverUrl + "/", Url: serverUrl + "/"}}))
}

func TestListExtractorVersionsFromRemoteRepo(t *testing.T) {
	const metadataFile = "org/jfrog/buildinfo/build-info-extractor-maven3/maven-metadata.xml"
	tests := []struct {
		name         string
		env          string
		expectedPath string
	}{
		{"releases repo", coreutils.ReleasesRemoteEnv, "/my-remote/artifactory/oss-release-local/" + metadataFile},
		// A customer that still uses the deprecated variable must keep working.
		{"deprecated extractors remote", coreutils.DeprecatedExtractorsRemoteEnv, "/my-remote/" + metadataFile},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, requestedPaths := metadataServer(t, test.expectedPath, 0)
			configureTestServer(t, server.URL)
			t.Setenv(test.env, "test/my-remote")

			found, err := ListMavenExtractorVersions("2")

			require.NoError(t, err)
			// Newest first, no duplicates, no snapshot and no other major line.
			assert.Equal(t, []string{"2.44.2", "2.44.0", "2.43.9", "2.43.6"}, found.Versions)
			assert.False(t, found.Pinned)
			assert.Equal(t, coreutils.MavenExtractorVersionEnv, found.PinEnv)
			assert.Equal(t, []string{test.expectedPath}, requestedPaths())
		})
	}
}

func TestListExtractorVersionsPinned(t *testing.T) {
	t.Run("pinned version, no request", func(t *testing.T) {
		server, requestedPaths := metadataServer(t, "/", 0)
		configureTestServer(t, server.URL)
		t.Setenv(coreutils.ReleasesRemoteEnv, "test/my-remote")
		t.Setenv(coreutils.MavenExtractorVersionEnv, " 2.43.9 ")

		// The pin ignores the major line.
		found, err := ListMavenExtractorVersions("5")

		require.NoError(t, err)
		assert.Equal(t, biutils.ExtractorVersions{Versions: []string{"2.43.9"}, Pinned: true, PinEnv: coreutils.MavenExtractorVersionEnv}, found)
		assert.Empty(t, requestedPaths())
	})
	t.Run("each tool has its own variable", func(t *testing.T) {
		clearExtractorEnv(t)
		t.Setenv(coreutils.MavenExtractorVersionEnv, "2.43.9")
		t.Setenv(coreutils.GradleExtractorVersionEnv, "5.2.5")
		maven, err := ListMavenExtractorVersions("2")
		require.NoError(t, err)
		gradle, err := ListGradleExtractorVersions("5")
		require.NoError(t, err)
		assert.Equal(t, []string{"2.43.9"}, maven.Versions)
		assert.Equal(t, []string{"5.2.5"}, gradle.Versions)
	})
	t.Run("Gradle pin must fit the major line of the Gradle version", func(t *testing.T) {
		clearExtractorEnv(t)
		t.Setenv(coreutils.GradleExtractorVersionEnv, "5.2.5")
		// Gradle older than 6.8.1 needs the 4.x line.
		_, err := ListGradleExtractorVersions("4")
		assert.ErrorIs(t, err, biutils.ErrInvalidExtractorVersion)
		assert.ErrorContains(t, err, "4.x")
		// A Maven pin has no such rule.
		t.Setenv(coreutils.MavenExtractorVersionEnv, "3.0.0")
		_, err = ListMavenExtractorVersions("2")
		assert.NoError(t, err)
	})
	t.Run("invalid pin", func(t *testing.T) {
		clearExtractorEnv(t)
		t.Setenv(coreutils.GradleExtractorVersionEnv, "latest")
		_, err := ListGradleExtractorVersions("5")
		assert.ErrorIs(t, err, biutils.ErrInvalidExtractorVersion)
		assert.ErrorContains(t, err, coreutils.GradleExtractorVersionEnv)
	})
}

func TestListExtractorVersionsFailures(t *testing.T) {
	const metadataFile = "/my-remote/artifactory/oss-release-local/org/jfrog/buildinfo/build-info-extractor-maven3/maven-metadata.xml"

	t.Run("file not found", func(t *testing.T) {
		server, _ := metadataServer(t, "/other", 0)
		configureTestServer(t, server.URL)
		t.Setenv(coreutils.ReleasesRemoteEnv, "test/my-remote")
		_, err := ListMavenExtractorVersions("2")
		assert.ErrorContains(t, err, "404")
	})
	t.Run("slow server", func(t *testing.T) {
		server, _ := metadataServer(t, metadataFile, 2*time.Second)
		configureTestServer(t, server.URL)
		t.Setenv(coreutils.ReleasesRemoteEnv, "test/my-remote")
		override(t, &extractorVersionsTimeout, 100*time.Millisecond)

		start := time.Now()
		_, err := ListMavenExtractorVersions("2")

		assert.Error(t, err)
		assert.Less(t, time.Since(start), time.Second)
	})
}

func TestListExtractorVersionsCache(t *testing.T) {
	const metadataPath = "/my-remote/artifactory/oss-release-local/org/jfrog/buildinfo/build-info-extractor-maven3/maven-metadata.xml"
	setup := func(t *testing.T) (requests func() int) {
		server, requestedPaths := metadataServer(t, metadataPath, 0)
		configureTestServer(t, server.URL)
		t.Setenv(coreutils.ReleasesRemoteEnv, "test/my-remote")
		return func() int { return len(requestedPaths()) }
	}
	cacheFile := func(t *testing.T) string {
		path, err := versionsCachePath(mavenExtractorArtifact)
		require.NoError(t, err)
		return path
	}

	t.Run("the saved list is used for the cache time", func(t *testing.T) {
		requests := setup(t)
		first, err := ListMavenExtractorVersions("2")
		require.NoError(t, err)
		second, err := ListMavenExtractorVersions("2")
		require.NoError(t, err)
		assert.Equal(t, first.Versions, second.Versions)
		assert.Equal(t, 1, requests(), "the second call must not send a request")
		// The list of all the lines is saved, so the Gradle-style filter works on it too.
		other, err := ListMavenExtractorVersions("3")
		require.NoError(t, err)
		assert.Equal(t, []string{"3.0.0"}, other.Versions)
		assert.Equal(t, 1, requests())
		info, err := os.Stat(cacheFile(t))
		require.NoError(t, err)
		if runtime.GOOS != "windows" { // Windows has no Unix permission bits.
			assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
			dirInfo, err := os.Stat(filepath.Dir(cacheFile(t)))
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0700), dirInfo.Mode().Perm(), "only the user can change what the cache says")
		}
	})
	t.Run("an old list is read again", func(t *testing.T) {
		requests := setup(t)
		override(t, &extractorVersionsCacheTTL, 0)
		_, err := ListMavenExtractorVersions("2")
		require.NoError(t, err)
		_, err = ListMavenExtractorVersions("2")
		require.NoError(t, err)
		assert.Equal(t, 2, requests())
	})
	t.Run("lookup fails: the old list is used", func(t *testing.T) {
		server, _ := metadataServer(t, metadataPath, 0)
		configureTestServer(t, server.URL)
		t.Setenv(coreutils.ReleasesRemoteEnv, "test/my-remote")
		override(t, &extractorVersionsCacheTTL, 0)
		first, err := ListMavenExtractorVersions("2")
		require.NoError(t, err)

		server.Close() // The network is gone.
		second, err := ListMavenExtractorVersions("2")

		require.NoError(t, err)
		assert.Equal(t, first.Versions, second.Versions)
		assert.NotNil(t, first.Verify)
		assert.Nil(t, second.Verify, "the server is not asked about the jars on disk when the lookup failed")
	})
	t.Run("lookup fails and no saved list: the error", func(t *testing.T) {
		server, _ := metadataServer(t, metadataPath, 0)
		configureTestServer(t, server.URL)
		t.Setenv(coreutils.ReleasesRemoteEnv, "test/my-remote")
		server.Close()
		_, err := ListMavenExtractorVersions("2")
		assert.Error(t, err)
	})
	t.Run("a list of another repository is not used", func(t *testing.T) {
		requests := setup(t)
		_, err := ListMavenExtractorVersions("2")
		require.NoError(t, err)
		// Another repository: the path of the metadata file changes, and the saved list does not match.
		t.Setenv(coreutils.ReleasesRemoteEnv, "test/other-remote")
		_, err = ListMavenExtractorVersions("2")
		assert.Error(t, err, "the other repository has no such file")
		assert.Equal(t, 2, requests())
	})
	t.Run("a damaged saved list is ignored", func(t *testing.T) {
		requests := setup(t)
		path := cacheFile(t)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte("{not json"), 0600))
		found, err := ListMavenExtractorVersions("2")
		require.NoError(t, err)
		assert.NotEmpty(t, found.Versions)
		assert.Equal(t, 1, requests())
	})
	t.Run("a failed lookup is not repeated at once", func(t *testing.T) {
		for _, test := range []struct {
			name      string
			saveFirst bool
		}{{"no saved list", false}, {"old saved list", true}} {
			t.Run(test.name, func(t *testing.T) {
				server, _ := metadataServer(t, metadataPath, 0)
				configureTestServer(t, server.URL)
				t.Setenv(coreutils.ReleasesRemoteEnv, "test/my-remote")
				if test.saveFirst {
					override(t, &extractorVersionsCacheTTL, 0)
					_, err := ListMavenExtractorVersions("2")
					require.NoError(t, err)
				} else {
					// The server has no such file.
					t.Setenv(coreutils.ReleasesRemoteEnv, "test/other-remote")
				}
				// From now on the server fails, and counts the requests.
				var failedRequests atomic.Int32
				server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					failedRequests.Add(1)
					w.WriteHeader(http.StatusInternalServerError)
				})

				for i := 0; i < 3; i++ {
					found, err := ListMavenExtractorVersions("2")
					assert.Equal(t, test.saveFirst, err == nil)
					assert.Equal(t, test.saveFirst, len(found.Versions) > 0)
				}
				// The first call sends the request that fails, the others do not.
				assert.EqualValues(t, 1, failedRequests.Load())
			})
		}
	})
	t.Run("a failed lookup is tried again after the failure time", func(t *testing.T) {
		server, requestedPaths := metadataServer(t, "/other", 0)
		configureTestServer(t, server.URL)
		t.Setenv(coreutils.ReleasesRemoteEnv, "test/my-remote")
		override(t, &extractorVersionsFailureTTL, 0)
		for i := 0; i < 2; i++ {
			_, err := ListMavenExtractorVersions("2")
			assert.Error(t, err)
		}
		assert.Len(t, requestedPaths(), 2)
	})
	t.Run("a saved version that is not a version is dropped", func(t *testing.T) {
		requests := setup(t)
		path := cacheFile(t)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		content, err := json.Marshal(versionsCache{
			Source:    metadataSource(t),
			FetchedAt: time.Now(),
			Versions:  []string{"2.0/../../../../tmp/evil"},
		})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, content, 0600))
		found, err := ListMavenExtractorVersions("2")
		require.NoError(t, err)
		assert.Equal(t, []string{"2.44.2", "2.44.0", "2.43.9", "2.43.6"}, found.Versions)
		assert.Equal(t, 1, requests())
	})
}

// metadataSource is the URL of the metadata file, as the saved list records it as its source.
func metadataSource(t *testing.T) string {
	server, remotePath, err := getExtractorsRemoteDetails("org/jfrog/buildinfo/"+mavenExtractorArtifact+"/maven-metadata.xml", false)
	require.NoError(t, err)
	return server.ArtifactoryUrl + remotePath
}

// The announcement is printed once, although several versions can be tried in one command.
func TestExtractorDownloadNoticeIsPrintedOnce(t *testing.T) {
	clearExtractorEnv(t)
	var output bytes.Buffer
	log.SetLogger(log.NewLogger(log.INFO, &output))
	t.Cleanup(func() { log.SetLogger(log.NewLogger(log.INFO, os.Stderr)) })
	directDownloadNotice = sync.Once{}

	for i := 0; i < 3; i++ {
		_, _, err := GetExtractorsRemoteDetails("org/jfrog/buildinfo/x")
		require.NoError(t, err)
	}
	assert.Equal(t, 1, bytes.Count(output.Bytes(), []byte("not cached locally")))
}

// override sets a package variable for the test, and restores it at the end.
func override[T any](t *testing.T, variable *T, value T) {
	t.Helper()
	previous := *variable
	*variable = value
	t.Cleanup(func() { *variable = previous })
}
