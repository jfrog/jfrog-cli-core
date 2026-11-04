package transferfiles

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testRootPath   = "/api/storage/" + testSourceRepo
	testSourceRepo = "generic-local"
	testSourcePath = "folder"
	testSourceName = "file.txt"
)

func testSourceFile() api.FileRepresentation {
	return api.FileRepresentation{
		Repo: testSourceRepo,
		Path: testSourcePath,
		Name: testSourceName,
		Size: 11,
	}
}

func testSourceRelativePath() string {
	return testSourceRepo + "/" + testSourcePath + "/" + testSourceName
}

// fastNotFoundRecheck shrinks the backoff between ambiguous-404 re-checks so tests stay fast.
func fastNotFoundRecheck(t *testing.T) {
	t.Helper()
	old := notFoundRecheckBackoff
	notFoundRecheckBackoff = time.Millisecond
	t.Cleanup(func() { notFoundRecheckBackoff = old })
}

func TestIsSourceItemGone(t *testing.T) {
	assert.True(t, IsSourceItemGone(ErrSourceItemGone))
	assert.False(t, IsSourceItemGone(nil))
	assert.False(t, IsSourceItemGone(assert.AnError))
}

func TestGetFileMetadata_success(t *testing.T) {
	fileContent := "hello world"
	storagePath := "/api/storage/" + testSourceRelativePath()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			response := map[string]any{
				"repo":         testSourceRepo,
				"path":         testSourcePath + "/" + testSourceName,
				"created":      "2020-01-01T00:00:00.000Z",
				"createdBy":    "admin",
				"lastModified": "2020-01-02T00:00:00.000Z",
				"modifiedBy":   "deployer",
				"size":         strconv.Itoa(len(fileContent)),
				"checksums": map[string]string{
					"sha1":   "sha1-value",
					"sha256": "sha256-value",
					"md5":    "md5-value",
				},
			}
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(response))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			response := map[string]any{
				"properties": map[string][]string{
					"build.name": {"app"},
					"env":        {"prod", "release"},
				},
			}
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(response))
		case r.URL.Path == storagePath && r.URL.RawQuery == "stats":
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"downloadCount":    0,
				"lastDownloaded":   0,
				"lastDownloadedBy": "",
			}))
		default:
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	metadata, err := client.GetFileMetadata(context.Background(), testSourceFile())
	require.NoError(t, err)
	require.NotNil(t, metadata)

	assert.Equal(t, testSourceRepo, metadata.Repo)
	assert.Equal(t, testSourcePath, metadata.Path)
	assert.Equal(t, testSourceName, metadata.Name)
	assert.Equal(t, int64(len(fileContent)), metadata.Size)
	assert.Equal(t, "sha1-value", metadata.Sha1)
	assert.Equal(t, "sha256-value", metadata.Sha256)
	assert.Equal(t, "md5-value", metadata.Md5)
	assert.Equal(t, "2020-01-01T00:00:00.000Z", metadata.Created)
	assert.Equal(t, "admin", metadata.CreatedBy)
	assert.Equal(t, "2020-01-02T00:00:00.000Z", metadata.LastModified)
	assert.Equal(t, "deployer", metadata.ModifiedBy)
	assert.Equal(t, map[string][]string{
		"build.name": {"app"},
		"env":        {"prod", "release"},
	}, metadata.Properties)
	assert.Zero(t, metadata.DownloadCount)
	assert.Zero(t, metadata.LastDownloaded)
	assert.Empty(t, metadata.LastDownloadedBy)
}

func TestGetFileMetadata_includesDownloadStats(t *testing.T) {
	storagePath := "/api/storage/" + testSourceRelativePath()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"size": "11",
				"checksums": map[string]string{
					"sha1": "sha1-value",
				},
			}))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"No properties could be found."}]}`))
		case r.URL.Path == storagePath && r.URL.RawQuery == "stats":
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"downloadCount":    5,
				"lastDownloaded":   1788945284685,
				"lastDownloadedBy": "admin",
			}))
		default:
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	metadata, err := client.GetFileMetadata(context.Background(), testSourceFile())
	require.NoError(t, err)
	assert.Equal(t, int64(5), metadata.DownloadCount)
	assert.Equal(t, int64(1788945284685), metadata.LastDownloaded)
	assert.Equal(t, "admin", metadata.LastDownloadedBy)
}

func TestGetFileMetadata_statsNotFound_continuesWithoutStats(t *testing.T) {
	storagePath := "/api/storage/" + testSourceRelativePath()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"size": "11",
				"checksums": map[string]string{
					"sha1": "sha1-value",
				},
			}))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"No properties could be found."}]}`))
		case r.URL.Path == storagePath && r.URL.RawQuery == "stats":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"Unable to find item"}]}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	metadata, err := client.GetFileMetadata(context.Background(), testSourceFile())
	require.NoError(t, err)
	assert.Zero(t, metadata.DownloadCount)
	assert.Empty(t, metadata.LastDownloadedBy)
}

func TestGetFileMetadata_statsForbidden_continuesWithoutStats(t *testing.T) {
	storagePath := "/api/storage/" + testSourceRelativePath()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"size": "11",
				"checksums": map[string]string{
					"sha1": "sha1-value",
				},
			}))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"No properties could be found."}]}`))
		case r.URL.Path == storagePath && r.URL.RawQuery == "stats":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":[{"message":"Forbidden"}]}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	metadata, err := client.GetFileMetadata(context.Background(), testSourceFile())
	require.NoError(t, err)
	assert.Equal(t, "sha1-value", metadata.Sha1)
	assert.Zero(t, metadata.DownloadCount)
	assert.Empty(t, metadata.LastDownloadedBy)
}

func TestGetFileMetadata_statsContextCancel_returnsError(t *testing.T) {
	storagePath := "/api/storage/" + testSourceRelativePath()
	statsReceived := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"size": "11",
				"checksums": map[string]string{
					"sha1": "sha1-value",
				},
			}))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"No properties could be found."}]}`))
		case r.URL.Path == storagePath && r.URL.RawQuery == "stats":
			close(statsReceived)
			<-r.Context().Done()
		default:
			t.Errorf("unexpected request: %s", r.URL.String())
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct {
		metadata *SourceFileMetadata
		err      error
	}, 1)
	go func() {
		metadata, err := client.GetFileMetadata(ctx, testSourceFile())
		done <- struct {
			metadata *SourceFileMetadata
			err      error
		}{metadata, err}
	}()

	select {
	case <-statsReceived:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not receive stats request in time")
	}
	cancel()

	select {
	case result := <-done:
		assert.Nil(t, result.metadata)
		assert.ErrorIs(t, result.err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("GetFileMetadata did not return promptly after stats cancellation")
	}
}

// TestGetFileMetadata_propertiesNotFoundOtherBody_itemStillExists_failsNotSilentlyDropped covers
// B-14: an ambiguous properties-404 (a body that isn't the pinned "No properties could be found"
// string) on an item that still exists must NOT be classified as a deletion (B-13/B-20), and must
// NOT silently yield empty properties either (that transfers the file and drops its properties
// with no warning). The properties GET is retried (bounded) and, if it stays ambiguous, the item
// fails so it lands in the errors CSV and is retried on the next run.
func TestGetFileMetadata_propertiesNotFoundOtherBody_itemStillExists_failsNotSilentlyDropped(t *testing.T) {
	fastNotFoundRecheck(t)
	storagePath := "/api/storage/" + testSourceRelativePath()
	var storageInfoRequests, propertiesRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			storageInfoRequests++
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"size": "11",
				"checksums": map[string]string{
					"sha1": "sha1-value",
				},
			}))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			propertiesRequests++
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"Not found"}]}`))
		case r.URL.Path == storagePath && r.URL.RawQuery == "stats":
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == testRootPath:
			_, _ = w.Write([]byte(testRootBody))
		default:
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	metadata, err := client.GetFileMetadata(context.Background(), testSourceFile())
	assert.Nil(t, metadata)
	require.Error(t, err)
	assert.False(t, IsSourceItemGone(err), "an existing item must not be classified as deleted")
	assert.Equal(t, notFoundRecheckAttempts, propertiesRequests, "properties GET must be retried a bounded number of times")
	// Once for the initial fetch, once to corroborate that the item still exists.
	assert.Equal(t, 2, storageInfoRequests)
}

// TestGetFileMetadata_propertiesNotFoundGermanJSONBody_treatedAsNoProperties is the B-14 lab
// repro after the structural classification: a properties 404 that is a genuine Artifactory JSON
// error ({"errors":[{"status":404,...}]}) means "no properties" whatever language the message is
// in, so the file is transferred (no failure, no retries) and the unknown message is logged.
func TestGetFileMetadata_propertiesNotFoundGermanJSONBody_treatedAsNoProperties(t *testing.T) {
	fastNotFoundRecheck(t)
	propertiesNotFoundWarned.Store(false)
	_, stderrBuffer, previousLog := tests.RedirectLogOutputToBuffer()
	defer log.SetLogger(previousLog)
	storagePath := "/api/storage/" + testSourceRelativePath()
	var propertiesRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"size": "11"}))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			propertiesRequests++
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"status":404,"message":"Keine Eigenschaften gefunden"}]}`))
		case r.URL.Path == storagePath && r.URL.RawQuery == "stats":
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == testRootPath:
			_, _ = w.Write([]byte(testRootBody))
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	metadata, err := client.GetFileMetadata(context.Background(), testSourceFile())
	require.NoError(t, err)
	require.NotNil(t, metadata)
	assert.Nil(t, metadata.Properties)
	assert.Equal(t, 1, propertiesRequests, "a structural Artifactory 404 must not be retried")
	assert.Contains(t, stderrBuffer.String(), "non-English/unknown 404 message")
	assert.Contains(t, stderrBuffer.String(), testSourceRelativePath())
}

// TestGetFileMetadata_propertiesNotFoundGermanJSONBody_warnsOnce: a server answering every
// properties 404 in another locale must not produce one Warn per file; the first occurrence warns,
// the rest are Debug.
func TestGetFileMetadata_propertiesNotFoundGermanJSONBody_warnsOnce(t *testing.T) {
	propertiesNotFoundWarned.Store(false)
	_, stderrBuffer, previousLog := tests.RedirectLogOutputToBuffer()
	defer log.SetLogger(previousLog)
	storagePath := "/api/storage/" + testSourceRelativePath()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"size": "11"}))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"status":404,"message":"Keine Eigenschaften gefunden"}]}`))
		case r.URL.Path == storagePath && r.URL.RawQuery == "stats":
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == testRootPath:
			_, _ = w.Write([]byte(testRootBody))
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_, err = client.GetFileMetadata(context.Background(), testSourceFile())
		require.NoError(t, err)
	}
	warnCount := 0
	for _, line := range strings.Split(stderrBuffer.String(), "\n") {
		// The debug-level repeats carry the same text, so count only the Warn lines (CI runs with JFROG_CLI_LOG_LEVEL=DEBUG).
		if strings.Contains(line, "[Warn]") && strings.Contains(line, "non-English/unknown 404 message") {
			warnCount++
		}
	}
	assert.Equal(t, 1, warnCount, "only the first occurrence may be logged at Warn level")
}

// TestGetFileMetadata_propertiesNotFoundGermanJSONBody_itemGone: the item vanished between the
// storage-info and properties requests; a structural 404 must not mask the deletion.
func TestGetFileMetadata_propertiesNotFoundGermanJSONBody_itemGone(t *testing.T) {
	fastNotFoundRecheck(t)
	storagePath := "/api/storage/" + testSourceRelativePath()
	var storageInfoRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			storageInfoRequests++
			if storageInfoRequests == 1 {
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"size": "11"}))
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"status":404,"message":"Unable to find item"}]}`))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"status":404,"message":"Element nicht gefunden"}]}`))
		case r.URL.Path == testRootPath:
			_, _ = w.Write([]byte(testRootBody))
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	metadata, err := client.GetFileMetadata(context.Background(), testSourceFile())
	assert.Nil(t, metadata)
	assert.True(t, IsSourceItemGone(err))
}

// TestGetFileMetadata_propertiesNotFoundNonArtifactoryBody_itemStillExists_fails: a 404 that is
// not an Artifactory JSON error (proxy HTML page, plain text, empty body, JSON without a 404
// status) could be anything, so it is retried (bounded) and then fails the item rather than
// silently dropping the properties.
func TestGetFileMetadata_propertiesNotFoundNonArtifactoryBody_itemStillExists_fails(t *testing.T) {
	bodies := map[string]string{
		"nginx html":       "<html><head><title>404 Not Found</title></head><body><center><h1>404 Not Found</h1></center><hr><center>nginx</center></body></html>",
		"plain text":       "404 page not found",
		"empty":            "",
		"json no status":   `{"errors":[{"message":"Keine Eigenschaften gefunden"}]}`,
		"json other shape": `{"message":"Keine Eigenschaften gefunden"}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			fastNotFoundRecheck(t)
			storagePath := "/api/storage/" + testSourceRelativePath()
			var propertiesRequests int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == storagePath && r.URL.RawQuery == "":
					w.WriteHeader(http.StatusOK)
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"size": "11"}))
				case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
					propertiesRequests++
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(body))
				case r.URL.Path == storagePath && r.URL.RawQuery == "stats":
					w.WriteHeader(http.StatusNotFound)
				case r.URL.Path == testRootPath:
					_, _ = w.Write([]byte(testRootBody))
				default:
					http.Error(w, "unexpected request", http.StatusInternalServerError)
				}
			}))
			defer server.Close()

			client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
			require.NoError(t, err)

			metadata, err := client.GetFileMetadata(context.Background(), testSourceFile())
			assert.Nil(t, metadata, "properties must not be silently dropped")
			require.Error(t, err)
			assert.False(t, IsSourceItemGone(err))
			assert.Equal(t, notFoundRecheckAttempts, propertiesRequests)
		})
	}
}

// TestGetFileMetadata_propertiesNotFoundOtherBody_recoversOnRetry verifies that a transient
// ambiguous properties-404 followed by a proper answer yields the real properties.
func TestGetFileMetadata_propertiesNotFoundOtherBody_recoversOnRetry(t *testing.T) {
	fastNotFoundRecheck(t)
	storagePath := "/api/storage/" + testSourceRelativePath()
	var propertiesRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"size": "11"}))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			propertiesRequests++
			if propertiesRequests == 1 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`404 page not found`))
				return
			}
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"properties": map[string][]string{"k": {"v"}},
			}))
		case r.URL.Path == storagePath && r.URL.RawQuery == "stats":
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == testRootPath:
			_, _ = w.Write([]byte(testRootBody))
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	metadata, err := client.GetFileMetadata(context.Background(), testSourceFile())
	require.NoError(t, err)
	assert.Equal(t, map[string][]string{"k": {"v"}}, metadata.Properties)
	assert.Equal(t, 2, propertiesRequests)
}

// TestGetFileMetadata_propertiesNotFoundProxyThenArtifactoryJSON404: a transient proxy 404 on the
// first properties request followed by Artifactory's own (non-English) JSON 404 resolves to
// "no properties" instead of exhausting the retries.
func TestGetFileMetadata_propertiesNotFoundProxyThenArtifactoryJSON404(t *testing.T) {
	fastNotFoundRecheck(t)
	storagePath := "/api/storage/" + testSourceRelativePath()
	var propertiesRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"size": "11"}))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			propertiesRequests++
			w.WriteHeader(http.StatusNotFound)
			if propertiesRequests == 1 {
				_, _ = w.Write([]byte(`404 page not found`))
				return
			}
			_, _ = w.Write([]byte(`{"errors":[{"status":404,"message":"Keine Eigenschaften gefunden"}]}`))
		case r.URL.Path == storagePath && r.URL.RawQuery == "stats":
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == testRootPath:
			_, _ = w.Write([]byte(testRootBody))
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	metadata, err := client.GetFileMetadata(context.Background(), testSourceFile())
	require.NoError(t, err)
	assert.Nil(t, metadata.Properties)
	assert.Equal(t, 2, propertiesRequests)
}

// TestGetFileMetadata_propertiesNotFoundEnglishBody_noRetryNoCorroboration pins the existing
// behavior: the recognized English "No properties could be found" body means an empty
// property set, immediately, without extra requests.
func TestGetFileMetadata_propertiesNotFoundEnglishBody_noRetryNoCorroboration(t *testing.T) {
	fastNotFoundRecheck(t)
	storagePath := "/api/storage/" + testSourceRelativePath()
	var storageInfoRequests, propertiesRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			storageInfoRequests++
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"size": "11"}))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			propertiesRequests++
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"No properties could be found."}]}`))
		case r.URL.Path == storagePath && r.URL.RawQuery == "stats":
			w.WriteHeader(http.StatusNotFound)
		default:
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	metadata, err := client.GetFileMetadata(context.Background(), testSourceFile())
	require.NoError(t, err)
	assert.Nil(t, metadata.Properties)
	assert.Equal(t, 1, propertiesRequests)
	assert.Equal(t, 1, storageInfoRequests)
}

// TestGetFileMetadata_propertiesNotFoundOtherBody_itemActuallyGone verifies that when the
// corroborating storage-info re-check also 404s, the item is correctly classified as gone.
func TestGetFileMetadata_propertiesNotFoundOtherBody_itemActuallyGone(t *testing.T) {
	fastNotFoundRecheck(t)
	storagePath := "/api/storage/" + testSourceRelativePath()
	storageInfoCall := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			storageInfoCall++
			if storageInfoCall == 1 {
				w.WriteHeader(http.StatusOK)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"size": "11",
					"checksums": map[string]string{
						"sha1": "sha1-value",
					},
				}))
				return
			}
			// Corroborating re-check: the item is gone by the time properties are fetched.
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"status":404,"message":"Unable to find item"}]}`))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"status":404,"message":"Unable to find item"}]}`))
		case r.URL.Path == testRootPath:
			_, _ = w.Write([]byte(testRootBody))
		default:
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	metadata, err := client.GetFileMetadata(context.Background(), testSourceFile())
	assert.Nil(t, metadata)
	assert.True(t, IsSourceItemGone(err))
}

// TestGetFileMetadata_notFound_confirmedByCorroboratingCheck verifies that a storage-info 404
// is classified as ErrSourceItemGone only after the control probe (repository root 200) and a
// second item check confirm it: item 404, root 200, item 404.
func TestGetFileMetadata_notFound_confirmedByCorroboratingCheck(t *testing.T) {
	var itemRequests, rootRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch storageRequestKind(r) {
		case "root":
			rootRequests++
			_, _ = w.Write([]byte(testRootBody))
		case "item":
			itemRequests++
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"status":404,"message":"Unable to find item"}]}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	metadata, err := client.GetFileMetadata(context.Background(), testSourceFile())
	assert.Nil(t, metadata)
	assert.True(t, IsSourceItemGone(err))
	assert.Equal(t, 2, itemRequests, "the initial storage-info 404 plus one confirming item check")
	assert.Equal(t, 1, rootRequests, "one control probe")
}

// TestGetFileMetadata_notFound_transient_notGone verifies that a storage-info 404 which does
// NOT hold up on corroborating re-check (e.g. a transient proxy hiccup) is surfaced as a plain
// error, not silently classified as a deletion.
func TestGetFileMetadata_notFound_transient_notGone(t *testing.T) {
	storagePath := "/api/storage/" + testSourceRelativePath()
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testRootPath {
			_, _ = w.Write([]byte(testRootBody))
			return
		}
		if r.URL.Path != storagePath || r.URL.RawQuery != "" {
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		requestCount++
		if requestCount == 1 {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"status":404,"message":"Unable to find item"}]}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"size": "11",
			"checksums": map[string]string{
				"sha1": "sha1-value",
			},
		}))
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	metadata, err := client.GetFileMetadata(context.Background(), testSourceFile())
	assert.Nil(t, metadata)
	require.Error(t, err)
	assert.False(t, IsSourceItemGone(err), "a 404 that doesn't hold up on corroboration must not be classified as a deletion")
}

func TestGetFileReader_success(t *testing.T) {
	fileContent := "streaming-bytes"
	downloadPath := "/" + testSourceRelativePath()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != downloadPath {
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(fileContent))
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	reader, err := client.GetFileReader(context.Background(), testSourceFile())
	require.NoError(t, err)
	require.NotNil(t, reader)
	defer func() {
		assert.NoError(t, reader.Close())
	}()

	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, fileContent, string(got))
}

// closeTrackingTransport wraps a RoundTripper and records, per request path, whether the
// response body handed to the caller was closed.
type closeTrackingTransport struct {
	base http.RoundTripper
	mu   sync.Mutex
	// closed maps request path -> body closed.
	closed map[string]bool
}

type closeTrackingBody struct {
	io.ReadCloser
	onClose func()
}

func (b *closeTrackingBody) Close() error {
	b.onClose()
	return b.ReadCloser.Close()
}

func (c *closeTrackingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := c.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	reqPath := req.URL.Path
	c.mu.Lock()
	c.closed[reqPath] = false
	c.mu.Unlock()
	resp.Body = &closeTrackingBody{ReadCloser: resp.Body, onClose: func() {
		c.mu.Lock()
		c.closed[reqPath] = true
		c.mu.Unlock()
	}}
	return resp, nil
}

func (c *closeTrackingTransport) isClosed(reqPath string) (closed, seen bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	closed, seen = c.closed[reqPath]
	return
}

// trackStreamBodies makes the client's content-GET (stream) transport record body closes.
func trackStreamBodies(client *SourceClient) *closeTrackingTransport {
	httpClient := client.streamServiceManager.Client().GetHttpClient().GetClient()
	tracker := &closeTrackingTransport{base: httpClient.Transport, closed: map[string]bool{}}
	httpClient.Transport = tracker
	return tracker
}

// TestGetFileReader_notFound_itemGone_closesBody: a content-GET 404 corroborated by the control
// probe (root 200) and a storage-info 404 is a real deletion (F-09/B-13 "skipped, logged"), and
// the 404 response body of the content GET must be closed.
func TestGetFileReader_notFound_itemGone_closesBody(t *testing.T) {
	downloadPath := "/" + testSourceRelativePath()
	var itemRequests, rootRequests int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch storageRequestKind(r) {
		case "content":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"status":404,"message":"Unable to find item"}]}`))
		case "root":
			rootRequests++
			_, _ = w.Write([]byte(testRootBody))
		case "item":
			itemRequests++
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"status":404,"message":"Unable to find item"}]}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)
	tracker := trackStreamBodies(client)

	reader, err := client.GetFileReader(context.Background(), testSourceFile())
	assert.Nil(t, reader)
	assert.True(t, IsSourceItemGone(err))
	assert.Equal(t, 1, rootRequests)
	assert.Equal(t, 1, itemRequests)
	closed, seen := tracker.isClosed(downloadPath)
	assert.True(t, seen, "content GET was not observed")
	assert.True(t, closed, "the 404 response body of the content GET must be closed")
}

// TestGetFileReader_notFound_storageInfoExists_notGone covers B-13/B-38: a content-GET 404 while
// storage-info says the item exists (transient 404, blacked-out repo, ...) must be a retryable
// error so the file lands in the errors CSV, never a silent "gone" skip. The error text must
// name the persistent causes instead of promising that it is transient (M3), and the content 404
// body must be closed.
func TestGetFileReader_notFound_storageInfoExists_notGone(t *testing.T) {
	downloadPath := "/" + testSourceRelativePath()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch storageRequestKind(r) {
		case "content":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"status":404,"message":"Unable to find item"}]}`))
		case "root":
			_, _ = w.Write([]byte(testRootBody))
		case "item":
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"size": "11"}))
		default:
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)
	tracker := trackStreamBodies(client)

	reader, err := client.GetFileReader(context.Background(), testSourceFile())
	assert.Nil(t, reader)
	require.Error(t, err)
	assert.False(t, IsSourceItemGone(err), "content 404 with an existing item must not be classified as a deletion")
	for _, cause := range []string{"blacked out", "include/exclude", "filestore"} {
		assert.Contains(t, err.Error(), cause, "the error must name the persistent causes of a content 404 on an existing item")
	}
	assert.NotContains(t, err.Error(), "treating this as a transient error")
	closed, _ := tracker.isClosed(downloadPath)
	assert.True(t, closed, "the 404 response body must be closed")
}

// TestGetFileReader_notFound_confirmError_isRetryableNotGone: when the corroboration itself fails
// (here the control probe is forbidden) the content 404 must surface as an error saying it could
// not be corroborated, never as "gone", and the response body must still be closed.
func TestGetFileReader_notFound_confirmError_isRetryableNotGone(t *testing.T) {
	downloadPath := "/" + testSourceRelativePath()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch storageRequestKind(r) {
		case "content":
			w.WriteHeader(http.StatusNotFound)
		case "root":
			w.WriteHeader(http.StatusForbidden)
		default:
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)
	tracker := trackStreamBodies(client)

	reader, err := client.GetFileReader(context.Background(), testSourceFile())
	assert.Nil(t, reader)
	require.Error(t, err)
	assert.False(t, IsSourceItemGone(err))
	assert.Contains(t, err.Error(), "could not corroborate")
	closed, _ := tracker.isClosed(downloadPath)
	assert.True(t, closed, "the 404 response body must be closed")
}

func TestGetFileReader_cancellation_closesBody(t *testing.T) {
	downloadPath := "/" + testSourceRelativePath()
	requestCancelled := make(chan struct{}, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != downloadPath {
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		for {
			select {
			case <-r.Context().Done():
				requestCancelled <- struct{}{}
				return
			default:
				_, _ = w.Write([]byte("x"))
				flusher.Flush()
				time.Sleep(5 * time.Millisecond)
			}
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader, err := client.GetFileReader(ctx, testSourceFile())
	require.NoError(t, err)
	require.NotNil(t, reader)

	buf := make([]byte, 1)
	_, err = reader.Read(buf)
	require.NoError(t, err)

	cancel()

	_, err = reader.Read(buf)
	assert.ErrorIs(t, err, context.Canceled)
	assert.NoError(t, reader.Close())

	select {
	case <-requestCancelled:
	case <-time.After(time.Second):
		t.Fatal("expected server request to observe cancellation")
	}
}

func newTestSourceServerDetails(serverURL string) *config.ServerDetails {
	return &config.ServerDetails{
		Url:            serverURL + "/",
		ArtifactoryUrl: serverURL + "/",
	}
}

func TestNewSourceClient_streamServiceManagerHasNoOverallTimeoutOrRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	streamConfig := client.streamServiceManager.GetConfig()
	assert.Zero(t, streamConfig.GetOverallRequestTimeout())
	assert.Zero(t, streamConfig.GetHttpRetries())

	metadataConfig := client.metadataServiceManager.GetConfig()
	assert.Equal(t, time.Minute, metadataConfig.GetOverallRequestTimeout())
	assert.Equal(t, metadataTransferRetries, metadataConfig.GetHttpRetries())
	assert.Equal(t, metadataRetryWaitMilliSecs, metadataConfig.GetHttpRetryWaitMilliSecs())
}

func TestGetFileReader_unrelatedErrorContaining404_notSourceGone(t *testing.T) {
	downloadPath := "/" + testSourceRelativePath()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != downloadPath {
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"message":"upstream returned 404 from cache"}]}`))
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	reader, err := client.GetFileReader(context.Background(), testSourceFile())
	assert.Nil(t, reader)
	require.Error(t, err)
	assert.False(t, IsSourceItemGone(err))
	// The source's explanation must reach the error (and so the errors CSV), not just the status line.
	assert.Contains(t, err.Error(), "upstream returned 404 from cache")
}

func TestGetFileReader_streamSurvivesSlowResponse(t *testing.T) {
	downloadPath := "/" + testSourceRelativePath()
	fileContent := "delayed-stream"
	streamDelay := 300 * time.Millisecond

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != downloadPath {
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		time.Sleep(streamDelay)
		_, _ = w.Write([]byte(fileContent))
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), streamDelay*3)
	defer cancel()

	reader, err := client.GetFileReader(ctx, testSourceFile())
	require.NoError(t, err)
	require.NotNil(t, reader)
	defer func() {
		assert.NoError(t, reader.Close())
	}()

	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, fileContent, string(got))
}

// TestGetFileReader_nonRespondingPeer_boundedByResponseHeaderTimeout reproduces B-17(b): a peer
// that accepts the TCP connection but never writes any response (unlike a slow-but-responding
// server, or a connection refused/reset). Without a ResponseHeaderTimeout on the streaming
// transport, GetFileReader would block forever on such a peer, since the streaming service
// manager's overall http.Client.Timeout is intentionally 0 (see
// TestNewSourceClient_streamServiceManagerHasNoOverallTimeoutOrRetries) to allow large file
// bodies to stream for as long as they need. streamResponseHeaderTimeout is shrunk here so the
// test doesn't have to wait out the multi-minute production value.
func TestGetFileReader_nonRespondingPeer_boundedByResponseHeaderTimeout(t *testing.T) {
	origTimeout := streamResponseHeaderTimeout
	streamResponseHeaderTimeout = 200 * time.Millisecond
	defer func() { streamResponseHeaderTimeout = origTimeout }()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			// Accept the connection and drain whatever the client sends, but never write a
			// response and never close the connection - simulating a hung peer.
			go func(c net.Conn) {
				buf := make([]byte, 4096)
				for {
					if _, readErr := c.Read(buf); readErr != nil {
						return
					}
				}
			}(conn)
		}
	}()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails("http://"+ln.Addr().String()))
	require.NoError(t, err)

	type result struct {
		reader io.ReadCloser
		err    error
	}
	done := make(chan result, 1)
	go func() {
		reader, getErr := client.GetFileReader(context.Background(), testSourceFile())
		done <- result{reader, getErr}
	}()

	select {
	case res := <-done:
		require.Error(t, res.err, "a non-responding peer must fail once the response-header timeout elapses, not succeed")
		assert.Nil(t, res.reader)
	case <-time.After(5 * time.Second):
		t.Fatal("GetFileReader hung indefinitely against a non-responding peer instead of being bounded by streamResponseHeaderTimeout")
	}
}

// TestGetFileReader_contextCancel_abortsRoundTrip verifies that cancelling the caller
// context while the server has not yet sent any response headers causes GetFileReader
// to return promptly with context.Canceled, not block until the server decides to reply.
func TestGetFileReader_contextCancel_abortsRoundTrip(t *testing.T) {
	downloadPath := "/" + testSourceRelativePath()
	serverReceivedRequest := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != downloadPath {
			t.Errorf("unexpected request: %s", r.URL.String())
			return
		}
		// Signal that the server has the request; block until it's cancelled.
		close(serverReceivedRequest)
		<-r.Context().Done()
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := client.GetFileReader(ctx, testSourceFile())
		done <- err
	}()

	// Wait for server to start handling the request, then cancel.
	select {
	case <-serverReceivedRequest:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not receive request in time")
	}
	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("GetFileReader did not return promptly after context cancellation")
	}
}

// TestGetFileMetadata_contextCancel_abortsMetadataRoundTrip verifies that cancelling the
// caller context while fetchFileInfo is waiting for a response causes GetFileMetadata to
// return promptly with context.Canceled.
func TestGetFileMetadata_contextCancel_abortsMetadataRoundTrip(t *testing.T) {
	storagePath := "/api/storage/" + testSourceRelativePath()
	serverReceivedRequest := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != storagePath {
			return
		}
		close(serverReceivedRequest)
		<-r.Context().Done()
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := client.GetFileMetadata(ctx, testSourceFile())
		done <- err
	}()

	select {
	case <-serverReceivedRequest:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not receive metadata request in time")
	}
	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("GetFileMetadata did not return promptly after context cancellation")
	}
}

// TestGetFileMetadata_sizeParseError_noFallback verifies that when the fileinfo size
// field is unparseable and the caller's FileRepresentation carries no positive size,
// GetFileMetadata returns a wrapped, path-specific error rather than silently using zero.
func TestGetFileMetadata_sizeParseError_noFallback(t *testing.T) {
	storagePath := "/api/storage/" + testSourceRelativePath()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"size": "not-a-number",
				"checksums": map[string]string{
					"sha1": "sha1-value",
				},
			}))
		default:
			// other endpoints won't be reached before error is returned
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	// Size == 0: no positive fallback, so a parse error must be returned.
	file := testSourceFile()
	file.Size = 0

	metadata, err := client.GetFileMetadata(context.Background(), file)
	assert.Nil(t, metadata)
	require.Error(t, err)
	assert.Contains(t, err.Error(), testSourceRelativePath())
}

// TestGetFileMetadata_sizeParseError_fallbackUsed verifies that when the fileinfo size
// is unparseable but the FileRepresentation carries a positive size, the positive size
// is used as a fallback and no error is returned.
func TestGetFileMetadata_sizeParseError_fallbackUsed(t *testing.T) {
	storagePath := "/api/storage/" + testSourceRelativePath()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"size": "not-a-number",
				"checksums": map[string]string{
					"sha1": "sha1-value",
				},
			}))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"No properties could be found."}]}`))
		case r.URL.Path == storagePath && r.URL.RawQuery == "stats":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"Unable to find item"}]}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	// testSourceFile() has Size=11 > 0, so the fallback must be used.
	metadata, err := client.GetFileMetadata(context.Background(), testSourceFile())
	require.NoError(t, err)
	assert.Equal(t, int64(11), metadata.Size)
}

func TestGetFileMetadata_folderHasNoSizeField(t *testing.T) {
	storagePath := "/api/storage/" + testSourceRelativePath()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"repo":         testSourceRepo,
				"path":         testSourcePath + "/" + testSourceName,
				"created":      "2020-01-01T00:00:00.000Z",
				"createdBy":    "admin",
				"lastModified": "2020-01-02T00:00:00.000Z",
				"modifiedBy":   "deployer",
				"children":     []map[string]any{{"uri": "/child", "folder": false}},
			}))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"No properties could be found."}]}`))
		case r.URL.Path == storagePath && r.URL.RawQuery == "stats":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"Unable to find item"}]}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	file := testSourceFile()
	file.Size = 0

	metadata, err := client.GetFileMetadata(context.Background(), file)
	require.NoError(t, err)
	require.NotNil(t, metadata)
	assert.Zero(t, metadata.Size)
}

// TestGetFileMetadata_folderItem_skipsStatsGET verifies that a real folder item (Name == "",
// as produced for AQL folder results) resolves Size 0 and never issues the "stats" GET, since
// download statistics don't apply to folders.
func TestGetFileMetadata_folderItem_skipsStatsGET(t *testing.T) {
	folderRelativePath := testSourceRepo + "/" + testSourcePath
	storagePath := "/api/storage/" + folderRelativePath
	statsRequested := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"repo":     testSourceRepo,
				"path":     testSourcePath,
				"children": []map[string]any{{"uri": "/child", "folder": false}},
			}))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"No properties could be found."}]}`))
		case r.URL.Path == storagePath && r.URL.RawQuery == "stats":
			statsRequested = true
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		default:
			t.Errorf("unexpected request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	folder := api.FileRepresentation{Repo: testSourceRepo, Path: testSourcePath, Name: ""}
	metadata, err := client.GetFileMetadata(context.Background(), folder)
	require.NoError(t, err)
	require.NotNil(t, metadata)
	assert.Zero(t, metadata.Size)
	assert.False(t, statsRequested, "stats GET must be skipped for a folder item")
}

// TestSourceClient_metadataAndContentGET_usesOnlySourceCredentials exercises both
// metadata GET and content GET against a live source httptest while a distinct
// target httptest must receive zero traffic.
func TestSourceClient_metadataAndContentGET_usesOnlySourceCredentials(t *testing.T) {
	const sourceToken = "source-secret-token"
	const targetToken = "target-secret-token"
	storagePath := "/api/storage/" + testSourceRelativePath()
	downloadPath := "/" + testSourceRelativePath()
	fileContent := "hello world"

	var sourceAuth []string
	var sourcePaths []string
	var sawContentGET bool
	targetHits := 0

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits++
		t.Errorf("source client contacted target: %s %s auth=%q", r.Method, r.URL.String(), r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusForbidden)
	}))
	defer target.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourcePaths = append(sourcePaths, r.URL.Path)
		sourceAuth = append(sourceAuth, r.Header.Get("Authorization"))
		switch {
		case r.URL.Path == storagePath && r.URL.RawQuery == "":
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"size":      strconv.Itoa(len(fileContent)),
				"checksums": map[string]string{"sha1": "sha1-value"},
			}))
		case r.URL.Path == storagePath && r.URL.RawQuery == "properties":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"No properties could be found."}]}`))
		case r.URL.Path == storagePath && r.URL.RawQuery == "stats":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"Unable to find item"}]}`))
		case r.URL.Path == downloadPath:
			sawContentGET = true
			_, _ = w.Write([]byte(fileContent))
		default:
			t.Errorf("unexpected source request: %s", r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer source.Close()

	sourceDetails := newTestSourceServerDetails(source.URL)
	sourceDetails.AccessToken = sourceToken
	targetDetails := newTestSourceServerDetails(target.URL)
	targetDetails.AccessToken = targetToken
	require.NotEqual(t, sourceDetails.ArtifactoryUrl, targetDetails.ArtifactoryUrl)

	client, err := NewSourceClient(context.Background(), sourceDetails)
	require.NoError(t, err)

	_, err = client.GetFileMetadata(context.Background(), testSourceFile())
	require.NoError(t, err)

	reader, err := client.GetFileReader(context.Background(), testSourceFile())
	require.NoError(t, err)
	require.NotNil(t, reader)
	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, fileContent, string(got))
	assert.NoError(t, reader.Close())

	require.NotEmpty(t, sourceAuth, "expected source traffic")
	require.True(t, sawContentGET, "content GET must be exercised")
	for i, auth := range sourceAuth {
		assert.Equal(t, "Bearer "+sourceToken, auth, "source request %d", i)
		assert.NotContains(t, auth, targetToken)
	}
	assert.Zero(t, targetHits)
	for _, p := range sourcePaths {
		assert.NotContains(t, p, "/api/plugins/execute")
	}
}

// --- confirmItemGone control probe (B-20 / JFMIG-93 review blocker B1) ---

const testRootBody = `{"repo":"` + testSourceRepo + `","path":"/","children":[]}`

// storageRequestKind classifies a request to the fake source by what it targets.
func storageRequestKind(r *http.Request) string {
	rel := testSourceRelativePath()
	switch {
	case r.URL.Path == "/"+rel:
		return "content"
	case r.URL.Path == "/api/storage/"+rel && r.URL.RawQuery == "":
		return "item"
	case r.URL.Path == "/api/storage/"+rel && r.URL.RawQuery == "properties":
		return "properties"
	case r.URL.Path == "/api/storage/"+rel && r.URL.RawQuery == "stats":
		return "stats"
	case r.URL.Path == "/api/storage/"+testSourceRepo:
		return "root"
	}
	return "unknown"
}

// fastConfirmRetry shrinks the interval between retries of the confirmation requests.
func fastConfirmRetry(t *testing.T) {
	t.Helper()
	old := confirmRetryInterval
	confirmRetryInterval = time.Millisecond
	t.Cleanup(func() { confirmRetryInterval = old })
}

// confirmEntryPoints are the three callers of confirmItemGone, each driven through its public method.
// "setup" prepares the request counters the fake server uses to answer the first, ambiguous request.
var confirmEntryPoints = []struct {
	name string
	// firstRequestOK: the first item (storage-info) request answers 200 so the flow reaches the
	// request that triggers the confirmation (properties 404).
	firstRequestOK bool
	run            func(client *SourceClient) error
}{
	{"storage-info 404", false, func(c *SourceClient) error {
		_, err := c.GetFileMetadata(context.Background(), testSourceFile())
		return err
	}},
	{"properties 404", true, func(c *SourceClient) error {
		_, err := c.GetFileMetadata(context.Background(), testSourceFile())
		return err
	}},
	{"content 404", false, func(c *SourceClient) error {
		r, err := c.GetFileReader(context.Background(), testSourceFile())
		if r != nil {
			_ = r.Close()
		}
		return err
	}},
}

// TestConfirmItemGone_outageWindowLongerThanRecheckSpan_isRetryableNotGone is the B-20 acceptance
// test: while the source answers 404 for the item AND for the repository root (a router/Artifactory
// restart window of any length), the item must never be declared gone. The window is held open for
// the whole call, so no number of re-checks can see the end of it; only a control probe can tell
// "outage" from "deleted". Once the window closes the same call succeeds.
func TestConfirmItemGone_outageWindowLongerThanRecheckSpan_isRetryableNotGone(t *testing.T) {
	for _, ep := range confirmEntryPoints {
		t.Run(ep.name, func(t *testing.T) {
			var windowOpen atomic.Bool
			windowOpen.Store(true)
			var itemRequests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				kind := storageRequestKind(r)
				if kind == "item" && ep.firstRequestOK && itemRequests.Add(1) == 1 {
					_, _ = w.Write([]byte(`{"size":"11"}`))
					return
				}
				if windowOpen.Load() && (kind == "item" || kind == "properties" || kind == "root" || kind == "content" || kind == "stats") {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`404 page not found`))
					return
				}
				switch kind {
				case "root":
					_, _ = w.Write([]byte(testRootBody))
				case "item":
					_, _ = w.Write([]byte(`{"size":"11"}`))
				case "properties":
					_, _ = w.Write([]byte(`{"properties":{"k":["v"]}}`))
				case "content":
					_, _ = w.Write([]byte("hello world"))
				case "stats":
					w.WriteHeader(http.StatusNotFound)
				default:
					http.Error(w, "unexpected request", http.StatusInternalServerError)
				}
			}))
			defer server.Close()
			client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
			require.NoError(t, err)

			start := time.Now()
			err = ep.run(client)
			elapsed := time.Since(start)
			require.Error(t, err, "a 404 while the repository root also 404s is an outage, not a deletion")
			assert.False(t, IsSourceItemGone(err), "item must not be declared gone inside an outage window: %v", err)
			assert.Less(t, elapsed, 1500*time.Millisecond, "an outage must be reported without sleeping through re-checks")

			// The window closes: the very same call now succeeds (the failure was retryable).
			windowOpen.Store(false)
			itemRequests.Store(10)
			assert.NoError(t, ep.run(client))
		})
	}
}

// TestConfirmItemGone_rootOK_itemMissing_goneWithoutStall: the repository root answers (200, JSON)
// and the item 404s, so the item really is deleted. This must be decided at once, without the
// multi-second sleeping the old consecutive re-checks cost every deleted file.
func TestConfirmItemGone_rootOK_itemMissing_goneWithoutStall(t *testing.T) {
	for _, ep := range confirmEntryPoints {
		t.Run(ep.name, func(t *testing.T) {
			var rootRequests atomic.Int32
			var itemRequests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch storageRequestKind(r) {
				case "root":
					rootRequests.Add(1)
					_, _ = w.Write([]byte(testRootBody))
				case "item":
					if ep.firstRequestOK && itemRequests.Add(1) == 1 {
						_, _ = w.Write([]byte(`{"size":"11"}`))
						return
					}
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"errors":[{"status":404,"message":"Unable to find item"}]}`))
				case "properties", "content":
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"errors":[{"status":404,"message":"Unable to find item"}]}`))
				default:
					http.Error(w, "unexpected request", http.StatusInternalServerError)
				}
			}))
			defer server.Close()
			client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
			require.NoError(t, err)

			start := time.Now()
			err = ep.run(client)
			elapsed := time.Since(start)
			assert.True(t, IsSourceItemGone(err), "root 200 + item 404 means deleted, got: %v", err)
			assert.Less(t, elapsed, 500*time.Millisecond, "deleted items must not stall the worker")
			assert.GreaterOrEqual(t, int(rootRequests.Load()), 1, "the repository root must be probed before declaring gone")
		})
	}
}

// TestConfirmItemGone_rootProbeUnusable_isRetryableNotGone: a root probe that does not prove the
// repository is reachable and healthy (404, 403, 5xx, a 200 that is not Artifactory JSON) must
// never let an item be declared gone.
func TestConfirmItemGone_rootProbeUnusable_isRetryableNotGone(t *testing.T) {
	fastConfirmRetry(t)
	cases := map[string]func(w http.ResponseWriter){
		"root 404": func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`404 page not found`))
		},
		"root 403":         func(w http.ResponseWriter) { w.WriteHeader(http.StatusForbidden) },
		"root 503":         func(w http.ResponseWriter) { w.WriteHeader(http.StatusServiceUnavailable) },
		"root 200 html":    func(w http.ResponseWriter) { _, _ = w.Write([]byte(`<html><body>Starting up</body></html>`)) },
		"root 200 empty":   func(w http.ResponseWriter) {},
		"root 200 no repo": func(w http.ResponseWriter) { _, _ = w.Write([]byte(`{"errors":[]}`)) },
	}
	for name, rootAnswer := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if storageRequestKind(r) == "root" {
					rootAnswer(w)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()
			client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
			require.NoError(t, err)

			_, err = client.GetFileMetadata(context.Background(), testSourceFile())
			require.Error(t, err)
			assert.False(t, IsSourceItemGone(err), "unusable root probe must not produce 'gone': %v", err)
		})
	}
}

// TestConfirmItemGone_serverErrorsAreBoundedAndHonourContext: the confirmation requests must not
// inherit the metadata client's 600 x 5s retry (about 50 minutes on a persistent 503), and must stop
// promptly when the context is cancelled.
func TestConfirmItemGone_serverErrorsAreBoundedAndHonourContext(t *testing.T) {
	t.Run("bounded", func(t *testing.T) {
		fastConfirmRetry(t)
		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()
		client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
		require.NoError(t, err)

		// Safety net so a regression fails after a few seconds instead of retrying for ~50 minutes.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		time.AfterFunc(3*time.Second, cancel)

		gone, err := client.confirmItemGone(ctx, client.metadataServiceManager, testSourceRelativePath())
		assert.False(t, gone)
		require.Error(t, err)
		assert.NotErrorIs(t, err, context.Canceled, "the retries must give up on their own")
		assert.Equal(t, int32(confirmRetryAttempts), requests.Load()) // #nosec G115 -- small constant
	})
	t.Run("context cancelled during retry wait", func(t *testing.T) {
		old, oldAttempts := confirmRetryInterval, confirmRetryAttempts
		confirmRetryInterval, confirmRetryAttempts = time.Hour, 5
		t.Cleanup(func() { confirmRetryInterval, confirmRetryAttempts = old, oldAttempts })
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()
		client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(100*time.Millisecond, cancel)
		start := time.Now()
		gone, err := client.confirmItemGone(ctx, client.metadataServiceManager, testSourceRelativePath())
		assert.False(t, gone)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Less(t, time.Since(start), 2*time.Second)
	})
}

// TestConfirmItemGone_itemNonArtifactory404_isRetryableNotGone (N1): a healthy repo root with a
// plain-text/HTML/empty 404 on the item path is not Artifactory's "Unable to find item" answer,
// so the item must not be declared gone.
func TestConfirmItemGone_itemNonArtifactory404_isRetryableNotGone(t *testing.T) {
	for name, body := range map[string]string{"plain": "404 page not found", "html": "<html>404</html>", "empty": ""} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if storageRequestKind(r) == "root" {
					_, _ = w.Write([]byte(testRootBody))
					return
				}
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
			require.NoError(t, err)
			_, err = client.GetFileMetadata(context.Background(), testSourceFile())
			require.Error(t, err)
			assert.False(t, IsSourceItemGone(err), "non-Artifactory 404 body must not prove deletion: %v", err)
		})
	}
}

// --- N2: the repository-root probe must be cheap (bounded read, cached, deduplicated) ---

const testItemNotFoundBody = `{"errors":[{"status":404,"message":"Unable to find item"}]}`

// newRootProbeServer answers the repository root with rootHandler and every other storage request
// with an Artifactory 404 (a deleted item). It counts the root requests.
func newRootProbeServer(t *testing.T, rootRequests *atomic.Int32, rootHandler func(w http.ResponseWriter, r *http.Request)) *SourceClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if storageRequestKind(r) == "root" {
			rootRequests.Add(1)
			rootHandler(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(testItemNotFoundBody))
	}))
	t.Cleanup(server.Close)
	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)
	return client
}

func setRootProbeCacheTTL(t *testing.T, ttl time.Duration) {
	t.Helper()
	old := rootProbeCacheTTL
	rootProbeCacheTTL = ttl
	t.Cleanup(func() { rootProbeCacheTTL = old })
}

// TestConfirmItemGone_rootProbeIsCachedPerRepo: thousands of "gone" decisions in one repo must not
// each cost a root request (a first-level children listing on the server).
func TestConfirmItemGone_rootProbeIsCachedPerRepo(t *testing.T) {
	setRootProbeCacheTTL(t, time.Minute)
	var rootRequests atomic.Int32
	client := newRootProbeServer(t, &rootRequests, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testRootBody))
	})
	for i := 0; i < 20; i++ {
		gone, err := client.confirmItemGone(context.Background(), client.metadataServiceManager, testSourceRelativePath())
		require.NoError(t, err)
		assert.True(t, gone)
	}
	assert.Equal(t, int32(1), rootRequests.Load(), "a successful probe must be reused within the TTL")
}

// TestConfirmItemGone_concurrentRootProbesAreDeduplicated: workers that arrive while a probe is in
// flight share it (singleflight), even when nothing is cached yet.
func TestConfirmItemGone_concurrentRootProbesAreDeduplicated(t *testing.T) {
	setRootProbeCacheTTL(t, time.Minute)
	var rootRequests atomic.Int32
	client := newRootProbeServer(t, &rootRequests, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte(testRootBody))
	})
	const workers = 16
	var wg sync.WaitGroup
	results := make([]bool, workers)
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = client.confirmItemGone(context.Background(), client.metadataServiceManager, testSourceRelativePath())
		}(i)
	}
	wg.Wait()
	for i := 0; i < workers; i++ {
		require.NoError(t, errs[i])
		assert.True(t, results[i])
	}
	assert.Equal(t, int32(1), rootRequests.Load())
}

// TestConfirmItemGone_rootProbeCacheExpires: the cached success is only valid for the TTL.
func TestConfirmItemGone_rootProbeCacheExpires(t *testing.T) {
	setRootProbeCacheTTL(t, 50*time.Millisecond)
	var rootRequests atomic.Int32
	client := newRootProbeServer(t, &rootRequests, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testRootBody))
	})
	_, err := client.confirmItemGone(context.Background(), client.metadataServiceManager, testSourceRelativePath())
	require.NoError(t, err)
	time.Sleep(120 * time.Millisecond)
	_, err = client.confirmItemGone(context.Background(), client.metadataServiceManager, testSourceRelativePath())
	require.NoError(t, err)
	assert.Equal(t, int32(2), rootRequests.Load())
}

// TestConfirmItemGone_failedRootProbeIsNeverCached: only a successful probe may be reused. After a
// failure the next decision probes again, and once the root is healthy "gone" is reachable.
func TestConfirmItemGone_failedRootProbeIsNeverCached(t *testing.T) {
	fastConfirmRetry(t)
	setRootProbeCacheTTL(t, time.Minute)
	var healthy atomic.Bool
	var rootRequests atomic.Int32
	client := newRootProbeServer(t, &rootRequests, func(w http.ResponseWriter, _ *http.Request) {
		if !healthy.Load() {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`404 page not found`))
			return
		}
		_, _ = w.Write([]byte(testRootBody))
	})
	for i := 0; i < 3; i++ {
		gone, err := client.confirmItemGone(context.Background(), client.metadataServiceManager, testSourceRelativePath())
		require.Error(t, err)
		assert.False(t, gone)
	}
	assert.Equal(t, int32(3), rootRequests.Load(), "a failed probe must be repeated, not cached")

	healthy.Store(true)
	gone, err := client.confirmItemGone(context.Background(), client.metadataServiceManager, testSourceRelativePath())
	require.NoError(t, err)
	assert.True(t, gone)
}

// TestProbeRepositoryRoot_readsAtMostABoundedPrefix: the root answer is a children listing that can
// be tens of MB. The probe must stop reading once it has the "repo" field instead of buffering it.
func TestProbeRepositoryRoot_readsAtMostABoundedPrefix(t *testing.T) {
	const totalBytes = 64 << 20
	var written atomic.Int64
	var rootRequests atomic.Int32
	client := newRootProbeServer(t, &rootRequests, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		n, _ := w.Write([]byte(`{"repo":"` + testSourceRepo + `","children":[`))
		written.Add(int64(n))
		chunk := []byte(strings.Repeat(`{"uri":"/some-child-entry","folder":false},`, 1500))
		for written.Load() < totalBytes && r.Context().Err() == nil {
			n, err := w.Write(chunk)
			written.Add(int64(n))
			if err != nil {
				return
			}
		}
		_, _ = w.Write([]byte(`{}]}`))
	})

	err := probeRepositoryRoot(context.Background(), client.metadataServiceManager, testSourceRepo)
	require.NoError(t, err)
	// Let the server notice the closed connection.
	time.Sleep(200 * time.Millisecond)
	assert.Less(t, written.Load(), int64(totalBytes/2), "the probe must not download the whole listing")
}

// TestProbeRepositoryRoot_noRepoFieldWithinBoundedPrefix_fails: when the body has no "repo" field
// in the bounded prefix the probe fails (the safe, retryable side) instead of reading without bound.
func TestProbeRepositoryRoot_noRepoFieldWithinBoundedPrefix_fails(t *testing.T) {
	var rootRequests atomic.Int32
	client := newRootProbeServer(t, &rootRequests, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"children":[`))
		chunk := []byte(strings.Repeat(`{"uri":"/some-child-entry","folder":false},`, 1500))
		for i := 0; i < 600 && r.Context().Err() == nil; i++ {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
		_, _ = w.Write([]byte(`{}],"repo":"` + testSourceRepo + `"}`))
	})
	err := probeRepositoryRoot(context.Background(), client.metadataServiceManager, testSourceRepo)
	require.Error(t, err)
}

// --- N3: probe-before-item ordering ---

// TestConfirmItemGone_probeRunsBeforeItemCheck: the outage window closes at the very first root
// request (the root 404s once, then everything is healthy and the item answers with an Artifactory
// 404). Probing first sees the outage and returns a retryable error; checking the item first would
// see "healthy deleted item", probe the now healthy root and wrongly declare the item gone.
func TestConfirmItemGone_probeRunsBeforeItemCheck(t *testing.T) {
	var mu sync.Mutex
	var order []string
	var rootRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kind := storageRequestKind(r)
		mu.Lock()
		order = append(order, kind)
		mu.Unlock()
		switch kind {
		case "root":
			if rootRequests.Add(1) == 1 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`404 page not found`))
				return
			}
			_, _ = w.Write([]byte(testRootBody))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(testItemNotFoundBody))
		}
	}))
	defer server.Close()
	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	gone, err := client.confirmItemGone(context.Background(), client.metadataServiceManager, testSourceRelativePath())
	assert.False(t, gone, "the outage visible at the first root request must win")
	require.Error(t, err)
	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, order)
	assert.Equal(t, "root", order[0], "the control probe must be the first confirmation request")
	assert.NotContains(t, order, "item", "no item check once the probe failed")
}

// --- N4: test-strength gaps ---

// TestGetFileMetadata_ambiguousPropertiesWaitHonoursContext: the wait between properties
// re-requests must be cancelled with the context. The backoff is an hour, so only a context-aware
// wait returns; a safety timer makes a regression fail fast instead of hanging until the go test
// timeout.
func TestGetFileMetadata_ambiguousPropertiesWaitHonoursContext(t *testing.T) {
	old := notFoundRecheckBackoff
	notFoundRecheckBackoff = time.Hour
	t.Cleanup(func() { notFoundRecheckBackoff = old })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch storageRequestKind(r) {
		case "root":
			_, _ = w.Write([]byte(testRootBody))
		case "item":
			_, _ = w.Write([]byte(`{"size":"11"}`))
		default: // properties: an ambiguous, non-Artifactory 404 on an item that exists
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`404 page not found`))
		}
	}))
	defer server.Close()
	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(100*time.Millisecond, cancel)
	done := make(chan error, 1)
	go func() {
		_, callErr := client.GetFileMetadata(ctx, testSourceFile())
		done <- callErr
	}()
	select {
	case callErr := <-done:
		assert.ErrorIs(t, callErr, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("the properties re-request wait ignored context cancellation")
	}
}

// TestConfirmItemGone_retriesAreSpacedByConfirmRetryInterval: consecutive confirmation attempts
// against a failing source must be at least confirmRetryInterval apart, so a restart is not
// hammered by back-to-back requests.
func TestConfirmItemGone_retriesAreSpacedByConfirmRetryInterval(t *testing.T) {
	oldInterval, oldAttempts := confirmRetryInterval, confirmRetryAttempts
	confirmRetryInterval, confirmRetryAttempts = 100*time.Millisecond, 3
	t.Cleanup(func() { confirmRetryInterval, confirmRetryAttempts = oldInterval, oldAttempts })
	var mu sync.Mutex
	var stamps []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		stamps = append(stamps, time.Now())
		mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client, err := NewSourceClient(context.Background(), newTestSourceServerDetails(server.URL))
	require.NoError(t, err)

	gone, err := client.confirmItemGone(context.Background(), client.metadataServiceManager, testSourceRelativePath())
	assert.False(t, gone)
	require.Error(t, err)
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, stamps, 3)
	for i := 1; i < len(stamps); i++ {
		assert.GreaterOrEqual(t, stamps[i].Sub(stamps[i-1]), 90*time.Millisecond, "attempt %d came too soon after attempt %d", i+1, i)
	}
}
