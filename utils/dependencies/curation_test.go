package dependencies

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	biutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const curationMessage = "package org.jfrog.buildinfo:build-info-extractor-maven3:2.99.2 download was blocked by jfrog packages curation service " +
	"due to the following policies violated {block-immature-1day,immature-1-day,Package version is 0 days old}."

// The shape of a real Curation response (content type application/problem+json).
var curationBody = fmt.Sprintf(`{"type":"Curation","title":"Artifact blocked by JFrog Curation","status":403,"detail":%q,"errors":[{"status":403,"message":%q}]}`, curationMessage, curationMessage)

func responseError(statusCode int, body string) error {
	return &errorutils.HttpResponseError{StatusCode: statusCode, Status: fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)), Body: []byte(body)}
}

func TestIsBlockedByCuration(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"curation problem", responseError(http.StatusForbidden, curationBody), true},
		{"curation message only", responseError(http.StatusForbidden, `{"errors":[{"status":403,"message":"Package x was blocked by JFrog Packages Curation service"}]}`), true},
		{"curation text, body is not JSON", responseError(http.StatusForbidden, "<html>blocked by JFrog Packages Curation service</html>"), true},
		{"wrapped error", fmt.Errorf("failed: %w", responseError(http.StatusForbidden, curationBody)), true},
		{"missing permission", responseError(http.StatusForbidden, `{"errors":[{"status":403,"message":"Forbidden"}]}`), false},
		{"empty body", responseError(http.StatusForbidden, ""), false},
		{"not a 403", responseError(http.StatusNotFound, curationBody), false},
		{"not a response error", errors.New("blocked by jfrog packages curation service"), false},
		{"no error", nil, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, isBlockedByCuration(test.err))
		})
	}
}

// fakeReleasesServer serves extractor jars. Versions in 'blocked' get a Curation 403, and the version 'forbidden' gets a 403 for a missing permission.
type fakeReleasesServer struct {
	*httptest.Server
	mutex    sync.Mutex
	requests []string
}

// blocked: versions that Curation blocks. forbidden: versions refused for a missing permission.
func newFakeReleasesServer(t *testing.T, blocked, forbidden []string) *fakeReleasesServer {
	server := &fakeReleasesServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.mutex.Lock()
		server.requests = append(server.requests, r.Method+" "+r.URL.Path)
		server.mutex.Unlock()
		switch {
		case isOneOf(r.URL.Path, forbidden):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":[{"status":403,"message":"Forbidden"}]}`))
		case isOneOf(r.URL.Path, blocked):
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(curationBody))
		case strings.HasSuffix(r.URL.Path, ".jar"):
			_, _ = w.Write([]byte("jar content"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func isOneOf(urlPath string, versions []string) bool {
	return slices.ContainsFunc(versions, func(v string) bool { return strings.Contains(urlPath, "/"+v+"/") })
}

func (s *fakeReleasesServer) count(request string) (count int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, r := range s.requests {
		if r == request {
			count++
		}
	}
	return
}

func (s *fakeReleasesServer) serverDetails() *config.ServerDetails {
	return &config.ServerDetails{ArtifactoryUrl: s.URL + "/"}
}

func downloadFakeExtractor(server *fakeReleasesServer, targetDir string) func(version string) error {
	return func(version string) error {
		remotePath := fmt.Sprintf("org/jfrog/buildinfo/build-info-extractor-maven3/%[1]s/build-info-extractor-maven3-%[1]s-uber.jar", version)
		return DownloadDependency(server.serverDetails(), remotePath, filepath.Join(targetDir, version, filepath.Base(remotePath)), false)
	}
}

func TestDownloadDependencyBlockedByCuration(t *testing.T) {
	t.Setenv(coreutils.HomeDir, t.TempDir())
	server := newFakeReleasesServer(t, []string{"2.99.2"}, []string{"2.99.1"})
	download := downloadFakeExtractor(server, t.TempDir())

	err := download("2.99.2")
	assert.ErrorIs(t, err, biutils.ErrBlockedByCuration)
	assert.Contains(t, err.Error(), "blocked by jfrog packages curation service", "the original message is kept")
	// Only the small request that reads the reason of the 403 is sent. The jar itself is never requested.
	assert.Equal(t, 1, server.count("GET /org/jfrog/buildinfo/build-info-extractor-maven3/2.99.2/build-info-extractor-maven3-2.99.2-uber.jar"))

	// A 403 that is not a Curation block is not reported as one.
	err = download("2.99.1")
	require.Error(t, err)
	assert.NotErrorIs(t, err, biutils.ErrBlockedByCuration)
}

// The whole chain: a blocked download in core, the walk of build-info-go.
func TestDownloadNewestExtractorWithCuration(t *testing.T) {
	const jarFormat = "build-info-extractor-maven3-%s-uber.jar"
	t.Setenv(coreutils.HomeDir, t.TempDir())
	versionsOf := func(versions ...string) biutils.ExtractorVersionsFunc {
		return func(string) (biutils.ExtractorVersions, error) {
			return biutils.ExtractorVersions{Versions: versions, PinEnv: "PIN_ENV"}, nil
		}
	}

	t.Run("blocked versions are skipped", func(t *testing.T) {
		targetDir := t.TempDir()
		server := newFakeReleasesServer(t, []string{"2.99.3", "2.99.2", "2.99.1"}, nil)
		chosen, err := biutils.DownloadNewestExtractor(versionsOf("2.99.3", "2.99.2", "2.99.1", "2.99.0"), targetDir, "2.99.3", jarFormat,
			downloadFakeExtractor(server, targetDir), &biutils.NullLog{})
		require.NoError(t, err)
		assert.Equal(t, "2.99.0", chosen)
		jar, err := os.ReadFile(filepath.Join(targetDir, "2.99.0", fmt.Sprintf(jarFormat, "2.99.0")))
		require.NoError(t, err)
		assert.Equal(t, "jar content", string(jar))
		assert.NoFileExists(t, filepath.Join(targetDir, "2.99.3", fmt.Sprintf(jarFormat, "2.99.3")))
	})
	t.Run("every version is blocked", func(t *testing.T) {
		targetDir := t.TempDir()
		server := newFakeReleasesServer(t, []string{"2.99.3", "2.99.2"}, nil)
		_, err := biutils.DownloadNewestExtractor(versionsOf("2.99.3", "2.99.2"), targetDir, "2.99.3", jarFormat, downloadFakeExtractor(server, targetDir), &biutils.NullLog{})
		assert.ErrorIs(t, err, biutils.ErrBlockedByCuration)
	})
	t.Run("a permission error stops the walk at the first version", func(t *testing.T) {
		targetDir := t.TempDir()
		server := newFakeReleasesServer(t, nil, []string{"2.99.3"})
		_, err := biutils.DownloadNewestExtractor(versionsOf("2.99.3", "2.99.2"), targetDir, "2.99.3", jarFormat, downloadFakeExtractor(server, targetDir), &biutils.NullLog{})
		require.Error(t, err)
		assert.NotErrorIs(t, err, biutils.ErrBlockedByCuration)
		assert.Zero(t, server.count("GET /org/jfrog/buildinfo/build-info-extractor-maven3/2.99.2/"+fmt.Sprintf(jarFormat, "2.99.2")))
	})
}

// The jar is copied into place in one step, and no temporary file stays.
func TestDownloadDependencyCopiesTheFileAtomically(t *testing.T) {
	t.Setenv(coreutils.HomeDir, t.TempDir())
	server := newFakeReleasesServer(t, nil, nil)
	targetDir := t.TempDir()

	require.NoError(t, downloadFakeExtractor(server, targetDir)("2.99.0"))

	versionDir := filepath.Join(targetDir, "2.99.0")
	entries, err := os.ReadDir(versionDir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "only the jar is expected")
	assert.Equal(t, "build-info-extractor-maven3-2.99.0-uber.jar", entries[0].Name())
}

func TestRememberBlockedAfterNullFile(t *testing.T) {
	configureTestServer(t, "http://localhost")
	_, cachePath := readBlockedCache()
	require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), 0755))
	// "null" is valid JSON, so it does not count as a damaged file.
	require.NoError(t, os.WriteFile(cachePath, []byte("null"), 0600))

	assert.NotPanics(t, func() { rememberBlocked("http://localhost/jar") })
	blocked, _ := readBlockedCache()
	assert.Contains(t, blocked, "http://localhost/jar")
}

func TestDownloadExtractorRemembersBlockedVersions(t *testing.T) {
	const jarPath = "org/jfrog/buildinfo/build-info-extractor-maven3/%[1]s/build-info-extractor-maven3-%[1]s-uber.jar"
	server := newFakeReleasesServer(t, []string{"2.99.2"}, nil)
	configureTestServer(t, server.URL)
	t.Setenv(coreutils.ReleasesRemoteEnv, "test/my-remote")
	targetDir := t.TempDir()
	download := func(version string) error {
		return DownloadExtractor(filepath.Join(targetDir, version, "extractor.jar"), fmt.Sprintf(jarPath, version))
	}
	blockedRequests := func() int {
		return server.count("HEAD /my-remote/artifactory/oss-release-local/"+fmt.Sprintf(jarPath, "2.99.2")) +
			server.count("GET /my-remote/artifactory/oss-release-local/"+fmt.Sprintf(jarPath, "2.99.2"))
	}

	// The first try asks the server.
	assert.ErrorIs(t, download("2.99.2"), biutils.ErrBlockedByCuration)
	assert.Equal(t, 2, blockedRequests())

	// The next tries do not.
	err := download("2.99.2")
	assert.ErrorIs(t, err, biutils.ErrBlockedByCuration)
	assert.Contains(t, err.Error(), "Delete ")
	assert.Equal(t, 2, blockedRequests(), "a blocked version must not be requested again")

	// An allowed version is not affected, and is not remembered.
	require.NoError(t, download("2.99.0"))
	require.NoError(t, download("2.99.0"))

	// The memory ends after the cache time.
	override(t, &extractorVersionsCacheTTL, 0)
	assert.ErrorIs(t, download("2.99.2"), biutils.ErrBlockedByCuration)
	assert.Equal(t, 4, blockedRequests())
}

// A jar on disk is not downloaded again. The check asks the server if Curation still allows it.
func TestVerifyExtractor(t *testing.T) {
	const jarPath = "org/jfrog/buildinfo/build-info-extractor-maven3/%[1]s/build-info-extractor-maven3-%[1]s-uber.jar"
	setup := func(t *testing.T, blocked, forbidden []string) (server *fakeReleasesServer, requests func(version string) int) {
		server = newFakeReleasesServer(t, blocked, forbidden)
		configureTestServer(t, server.URL)
		t.Setenv(coreutils.ReleasesRemoteEnv, "test/my-remote")
		return server, func(version string) int {
			path := "/my-remote/artifactory/oss-release-local/" + fmt.Sprintf(jarPath, version)
			return server.count("HEAD "+path) + server.count("GET "+path)
		}
	}
	verify := func(version string) error { return verifyExtractor(mavenExtractorArtifact, version) }

	t.Run("allowed: the answer is kept", func(t *testing.T) {
		_, requests := setup(t, nil, nil)
		require.NoError(t, verify("2.99.0"))
		assert.Equal(t, 1, requests("2.99.0"), "one request that does not send the jar")
		require.NoError(t, verify("2.99.0"))
		assert.Equal(t, 1, requests("2.99.0"), "the second check sends no request")
		// The answer ends after the cache time.
		override(t, &extractorVersionsCacheTTL, 0)
		require.NoError(t, verify("2.99.0"))
		assert.Equal(t, 2, requests("2.99.0"))
	})
	t.Run("a download that worked counts as a check", func(t *testing.T) {
		_, requests := setup(t, nil, nil)
		targetPath := filepath.Join(t.TempDir(), "extractor.jar")
		require.NoError(t, DownloadExtractor(targetPath, fmt.Sprintf(jarPath, "2.99.0")))
		before := requests("2.99.0")
		require.NoError(t, verify("2.99.0"))
		assert.Equal(t, before, requests("2.99.0"), "the check sends no request")
	})
	t.Run("blocked: the answer is kept", func(t *testing.T) {
		_, requests := setup(t, []string{"2.99.2"}, nil)
		assert.ErrorIs(t, verify("2.99.2"), biutils.ErrBlockedByCuration)
		assert.Equal(t, 2, requests("2.99.2"), "the request, and the request that reads the reason")
		assert.ErrorIs(t, verify("2.99.2"), biutils.ErrBlockedByCuration)
		assert.Equal(t, 2, requests("2.99.2"), "the second check sends no request")
	})
	t.Run("a missing permission is not a Curation block", func(t *testing.T) {
		_, _ = setup(t, nil, []string{"2.99.1"})
		err := verify("2.99.1")
		require.Error(t, err)
		assert.NotErrorIs(t, err, biutils.ErrBlockedByCuration)
	})
	t.Run("the server is not reachable: an error that is not a block, and nothing is kept", func(t *testing.T) {
		server, _ := setup(t, nil, nil)
		server.Close()
		err := verify("2.99.0")
		require.Error(t, err)
		assert.NotErrorIs(t, err, biutils.ErrBlockedByCuration)
		verified, _ := readURLCache(verifiedCacheFileName)
		assert.Empty(t, verified)
	})
	t.Run("the walk skips a cached jar that Curation blocked later", func(t *testing.T) {
		server, _ := setup(t, []string{"2.99.2"}, nil)
		baseDir := t.TempDir()
		const jarFormat = "build-info-extractor-maven3-%s-uber.jar"
		for _, v := range []string{"2.99.2", "2.99.1"} {
			require.NoError(t, os.MkdirAll(filepath.Join(baseDir, v), 0755))
			require.NoError(t, os.WriteFile(filepath.Join(baseDir, v, fmt.Sprintf(jarFormat, v)), []byte("cached jar"), 0644))
		}
		found := biutils.ExtractorVersions{Versions: []string{"2.99.2", "2.99.1"}, Verify: verify}
		lister := func(string) (biutils.ExtractorVersions, error) { return found, nil }
		chosen, err := biutils.DownloadNewestExtractor(lister, baseDir, "2.99.2", jarFormat, func(v string) error {
			if v == "2.99.2" {
				return errors.New("the blocked version must not be downloaded")
			}
			return nil
		}, &biutils.NullLog{})
		require.NoError(t, err)
		assert.Equal(t, "2.99.1", chosen)
		assert.Zero(t, server.count("GET /my-remote/artifactory/oss-release-local/"+fmt.Sprintf(jarPath, "2.99.1")), "the jar itself is not requested")
	})
}
