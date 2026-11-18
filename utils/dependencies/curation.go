package dependencies

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	biutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/http/jfroghttpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// Text that the Curation service puts in the message of a blocked download.
const curationBlockMarker = "jfrog packages curation"

// isBlockedByCuration returns true if the error is a 403 response of the JFrog Curation service.
// The response is a JSON problem with "type": "Curation", and the same text in the message of its first error.
// Any other 403, such as a missing permission, is not a Curation block.
func isBlockedByCuration(err error) bool {
	var responseErr *errorutils.HttpResponseError
	if !errors.As(err, &responseErr) || responseErr.StatusCode != http.StatusForbidden {
		return false
	}
	var problem struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(responseErr.Body, &problem) == nil && strings.EqualFold(problem.Type, "Curation") {
		return true
	}
	return strings.Contains(strings.ToLower(string(responseErr.Body)), curationBlockMarker)
}

// wrapBlockedByCuration lets the caller test the error with errors.Is(err, biutils.ErrBlockedByCuration). The original error stays in the message.
func wrapBlockedByCuration(err error) error {
	return fmt.Errorf("%w: %w", biutils.ErrBlockedByCuration, err)
}

// checkCurationBlock returns err wrapped as a Curation block, if the server refuses the file because of Curation.
// Otherwise it returns nil.
func checkCurationBlock(client *jfroghttpclient.JfrogHttpClient, httpClientDetails httputils.HttpClientDetails, url string, err error) error {
	if isBlockedByCuration(err) {
		return wrapBlockedByCuration(err)
	}
	var responseErr *errorutils.HttpResponseError
	if !errors.As(err, &responseErr) || responseErr.StatusCode != http.StatusForbidden || len(responseErr.Body) > 0 {
		return nil
	}
	// The response of a HEAD request, or of a download, has no body. Read the start of the file to get the reason of the 403.
	// Only a refused request returns a body of any size, so this does not download the file.
	// The headers are a map that the caller shares: change a copy.
	httpClientDetails.Headers = maps.Clone(httpClientDetails.Headers)
	httpClientDetails.AddHeader("Range", "bytes=0-1023")
	resp, body, _, getErr := client.SendGet(url, true, &httpClientDetails)
	if getErr != nil {
		// Without the body there is no proof of a Curation block. The caller reports the original error.
		log.Debug("Failed to read the reason of the 403 response:", getErr.Error())
		return nil
	}
	if reasonErr := errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK, http.StatusPartialContent); isBlockedByCuration(reasonErr) {
		return wrapBlockedByCuration(reasonErr)
	}
	return nil
}

const (
	// The URLs that Curation blocked, and when.
	blockedCacheFileName = "blocked.json"
	// The URLs of the jars on disk that the server confirmed as allowed, and when.
	verifiedCacheFileName = "verified.json"
)

func urlCachePath(fileName string) (string, error) {
	dependenciesPath, err := config.GetJfrogDependenciesPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dependenciesPath, versionsCacheDirName, fileName), nil
}

// readURLCache returns the URLs of a cache file, and when each one was saved. It is only a speed-up, so a missing or damaged file is not an error.
func readURLCache(fileName string) (urls map[string]time.Time, cachePath string) {
	cachePath, err := urlCachePath(fileName)
	if err != nil {
		return nil, ""
	}
	if content, err := os.ReadFile(cachePath); err == nil && json.Unmarshal(content, &urls) != nil {
		urls = nil // A damaged file is ignored.
	}
	if urls == nil { // Also the case for the text "null", which is valid JSON.
		urls = map[string]time.Time{}
	}
	return urls, cachePath
}

func readBlockedCache() (blocked map[string]time.Time, cachePath string) {
	return readURLCache(blockedCacheFileName)
}

// rememberURL saves a URL in a cache file, and removes the URLs that are older than the cache time.
// Two commands that write at the same time can drop one entry. This is accepted: a lost entry costs one extra request, never a wrong result.
func rememberURL(fileName, url string) {
	urls, cachePath := readURLCache(fileName)
	if cachePath == "" {
		return
	}
	for u, seen := range urls {
		if time.Since(seen) >= extractorVersionsCacheTTL {
			delete(urls, u)
		}
	}
	urls[url] = time.Now().UTC()
	content, err := json.Marshal(urls)
	if err == nil {
		err = writeCacheFile(cachePath, content)
	}
	if err != nil {
		log.Debug("Failed to save", fileName+":", err.Error())
	}
}

func rememberBlocked(url string) { rememberURL(blockedCacheFileName, url) }

func rememberVerified(url string) { rememberURL(verifiedCacheFileName, url) }

func recentlyVerified(url string) bool {
	verified, _ := readURLCache(verifiedCacheFileName)
	seen, ok := verified[url]
	return ok && time.Since(seen) < extractorVersionsCacheTTL
}

// checkRecentlyBlocked skips the requests for an extractor that Curation blocked a short time ago.
// A blocked extractor is tried again on every command otherwise, and each try sends two requests.
// The result is kept for extractorVersionsCacheTTL. Delete the file to try again at once.
func checkRecentlyBlocked(url string) error {
	blocked, cachePath := readBlockedCache()
	if seen, ok := blocked[url]; ok && time.Since(seen) < extractorVersionsCacheTTL {
		return fmt.Errorf("%w: %s was blocked at %s. It is not tried again for %s. Delete %s to try it now",
			biutils.ErrBlockedByCuration, url, seen.Format(time.RFC3339), extractorVersionsCacheTTL, cachePath)
	}
	return nil
}
