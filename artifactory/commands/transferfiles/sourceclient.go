package transferfiles

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	coreutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	artifactoryutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

var ErrSourceItemGone = errors.New("source item no longer exists")

// notFoundRecheckAttempts is how many times an unrecognized properties-404 (proxy page, plain
// text, ...) is requested in total before the item is failed instead of transferred without
// properties.
const notFoundRecheckAttempts = 3

// notFoundRecheckBackoff is the pause between those properties re-requests. It is a variable so
// tests can shrink it.
var notFoundRecheckBackoff = time.Second

// confirmRetryAttempts and confirmRetryInterval bound the retries of the confirmation requests
// (control probe and item existence check) on 5xx/429/transport errors. They deliberately do not
// reuse the metadata client's 600 x 5s retry, which would stall a worker for ~50 minutes on a
// source that keeps answering 503. Variables so tests can shrink them.
var confirmRetryAttempts = 3
var confirmRetryInterval = time.Second

// rootProbeCacheTTL is how long a successful repository-root probe is reused, so a burst of
// "gone" decisions in one repo costs one root request instead of one each. It is also the span in
// which an outage that starts right after a successful probe cannot be seen by the next decisions
// (see confirmItemGone). 0 disables reuse; concurrent probes are still shared. Variable for tests.
var rootProbeCacheTTL = 2 * time.Second

// rootProbeMaxBodyBytes bounds how much of the root probe answer is read. The answer is the
// repository's first-level children listing, which can be tens of MB; the probe only needs the
// leading "repo" field.
const rootProbeMaxBodyBytes = 1 << 20

// rootProbeTimeout bounds one shared probe flight, which runs detached from any single caller's
// context so that one cancelled worker does not fail the probe for the others.
const rootProbeTimeout = 2 * time.Minute

// propertiesNotFoundWarned makes the non-English properties-404 Warn appear once per process;
// later occurrences are logged at Debug so a server answering in another locale does not flood
// the log with one Warn per file.
var propertiesNotFoundWarned atomic.Bool

func IsSourceItemGone(err error) bool {
	return errors.Is(err, ErrSourceItemGone)
}

type SourceFileMetadata struct {
	Repo             string
	Path             string
	Name             string
	Size             int64
	Sha1             string
	Sha256           string
	Md5              string
	Created          string
	CreatedBy        string
	LastModified     string
	ModifiedBy       string
	Properties       map[string][]string
	DownloadCount    int64
	LastDownloaded   int64
	LastDownloadedBy string
}

type itemStatistics struct {
	DownloadCount    int64  `json:"downloadCount"`
	LastDownloaded   int64  `json:"lastDownloaded"`
	LastDownloadedBy string `json:"lastDownloadedBy"`
}

type SourceClient struct {
	serverDetails          *config.ServerDetails
	metadataServiceManager artifactory.ArtifactoryServicesManager
	streamServiceManager   artifactory.ArtifactoryServicesManager
	rootProbes             rootProbeCache
}

func createStreamingTransferServiceManager(ctx context.Context, serverDetails *config.ServerDetails, httpClient *http.Client) (artifactory.ArtifactoryServicesManager, error) {
	return coreutils.CreateServiceManagerWithContextAndHttpClient(ctx, serverDetails, false, 0, 0, 0, 0, httpClient)
}

func NewSourceClient(ctx context.Context, serverDetails *config.ServerDetails) (*SourceClient, error) {
	// Bounded retries, as on the target side: the transfer manager's 600 x 5s policy would stall a
	// worker for ~50 minutes per call on an item whose metadata keeps answering 5xx. A failed item
	// is reported in the errors CSV and retried by a later phase instead.
	metadataServiceManager, err := createMetadataTransferServiceManager(ctx, serverDetails, nil)
	if err != nil {
		return nil, err
	}
	streamTransport, err := newDefaultStreamingTransport(serverDetails)
	if err != nil {
		return nil, err
	}
	streamServiceManager, err := createStreamingTransferServiceManager(ctx, serverDetails, httpClientWithTransport(streamTransport, 0))
	if err != nil {
		return nil, err
	}
	return &SourceClient{
		serverDetails:          serverDetails,
		metadataServiceManager: metadataServiceManager,
		streamServiceManager:   streamServiceManager,
	}, nil
}

func (sc *SourceClient) GetFileMetadata(ctx context.Context, file api.FileRepresentation) (*SourceFileMetadata, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	relativePath := fileRelativePath(file)
	fileInfo, err := sc.fetchFileInfo(ctx, sc.metadataServiceManager, relativePath)
	if err != nil {
		return nil, err
	}

	// Folder-info responses carry no "size" field; treat that as size 0 rather than a parse failure.
	// Fall back to the AQL-reported size when it's available.
	var size int64
	if fileInfo.Size == "" {
		if file.Size > 0 {
			size = file.Size
		}
	} else {
		var parseErr error
		size, parseErr = strconv.ParseInt(fileInfo.Size, 10, 64)
		if parseErr != nil {
			if file.Size > 0 {
				size = file.Size
			} else {
				return nil, fmt.Errorf("failed to parse file size %q for %s: %w", fileInfo.Size, relativePath, parseErr)
			}
		}
	}

	itemProps, err := sc.fetchItemProperties(ctx, sc.metadataServiceManager, relativePath)
	if err != nil {
		return nil, err
	}

	metadata := &SourceFileMetadata{
		Repo:         file.Repo,
		Path:         file.Path,
		Name:         file.Name,
		Size:         size,
		Sha1:         fileInfo.Checksums.Sha1,
		Sha256:       fileInfo.Checksums.Sha256,
		Md5:          fileInfo.Checksums.Md5,
		Created:      fileInfo.Created,
		CreatedBy:    fileInfo.CreatedBy,
		LastModified: fileInfo.LastModified,
		ModifiedBy:   fileInfo.ModifiedBy,
	}
	if itemProps != nil {
		metadata.Properties = itemProps.Properties
	}
	if file.Name != "" {
		stats, statsErr := sc.fetchItemStatistics(ctx, sc.metadataServiceManager, relativePath)
		if statsErr != nil {
			return nil, statsErr
		}
		if stats != nil {
			metadata.DownloadCount = stats.DownloadCount
			metadata.LastDownloaded = stats.LastDownloaded
			metadata.LastDownloadedBy = stats.LastDownloadedBy
		}
	}
	return metadata, nil
}

func (sc *SourceClient) fetchFileInfo(ctx context.Context, manager artifactory.ArtifactoryServicesManager, relativePath string) (*artifactoryutils.FileInfo, error) {
	resp, body, err := sendStorageGet(ctx, manager, relativePath, "")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		gone, confirmErr := sc.confirmItemGone(ctx, manager, relativePath)
		if confirmErr != nil {
			return nil, confirmErr
		}
		if gone {
			return nil, ErrSourceItemGone
		}
		// The initial 404 didn't hold up on a corroborating re-check: the item exists
		// after all (a transient proxy hiccup, restart window, etc). Surface this as a
		// plain, retryable error instead of silently treating the file as deleted.
		return nil, fmt.Errorf("received a 404 from the source storage-info endpoint for %q, but a corroborating existence check found the item still exists; treating this as a transient error, not a deletion", relativePath)
	}
	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return nil, err
	}

	result := &artifactoryutils.FileInfo{}
	if err = json.Unmarshal(body, result); err != nil {
		return nil, errorutils.CheckError(err)
	}
	return result, nil
}

// noPropertiesMessage is the Artifactory 7 English error body of a properties GET on an item
// that exists but has no properties.
const noPropertiesMessage = "No properties could be found"

func (sc *SourceClient) fetchItemProperties(ctx context.Context, manager artifactory.ArtifactoryServicesManager, relativePath string) (*artifactoryutils.ItemProperties, error) {
	resp, body, err := sendStorageGet(ctx, manager, relativePath, "properties")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		if strings.Contains(string(body), noPropertiesMessage) {
			return nil, nil
		}
		// The body didn't match the pinned English string. Corroborate with a direct
		// existence check before concluding anything: the item may have been deleted, or the
		// 404 may not come from Artifactory at all.
		gone, confirmErr := sc.confirmItemGone(ctx, manager, relativePath)
		if confirmErr != nil {
			return nil, confirmErr
		}
		if gone {
			return nil, ErrSourceItemGone
		}
		// The item exists. A 404 that is a well-formed Artifactory JSON error is Artifactory's
		// own "no properties" answer in another locale/version: accept it, but log it so it
		// stays auditable.
		if isArtifactoryJSONNotFound(body) {
			logNonEnglishPropertiesNotFound(relativePath, body)
			return nil, nil
		}
		// Anything else (proxy HTML page, plain text, empty body, ...) could be a transient
		// failure. Returning "no properties" would silently drop them, so re-request the
		// properties (bounded, with backoff) and fail the item if the answer stays ambiguous.
		return sc.retryAmbiguousProperties(ctx, manager, relativePath)
	}
	return parseItemProperties(resp, body)
}

// isArtifactoryJSONNotFound reports whether body is an Artifactory JSON error carrying a 404
// entry, i.e. {"errors":[{"status":404,"message":"..."}]}. It deliberately ignores the message
// text, which is localized/version dependent, and requires the structure and status so proxy
// pages and plain-text bodies do not qualify.
func isArtifactoryJSONNotFound(body []byte) bool {
	var parsed struct {
		Errors []struct {
			Status int `json:"status"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false
	}
	for _, e := range parsed.Errors {
		if e.Status == http.StatusNotFound {
			return true
		}
	}
	return false
}

// logNonEnglishPropertiesNotFound logs the first such answer of the process as a Warn and the
// rest at Debug level.
func logNonEnglishPropertiesNotFound(relativePath string, body []byte) {
	msg := fmt.Sprintf("Properties lookup for %s returned a non-English/unknown 404 message (%s); treating as no properties.", relativePath, truncateForLog(body))
	if propertiesNotFoundWarned.CompareAndSwap(false, true) {
		log.Warn(msg + " Further occurrences are logged at debug level.")
		return
	}
	log.Debug(msg)
}

func truncateForLog(body []byte) string {
	const maxLen = 200
	msg := strings.TrimSpace(string(body))
	if len(msg) > maxLen {
		return msg[:maxLen] + "..."
	}
	return msg
}

func parseItemProperties(resp *http.Response, body []byte) (*artifactoryutils.ItemProperties, error) {
	if err := errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return nil, err
	}
	result := &artifactoryutils.ItemProperties{}
	if err := json.Unmarshal(body, result); err != nil {
		return nil, errorutils.CheckError(err)
	}
	return result, nil
}

// retryAmbiguousProperties re-issues the properties GET after a non-Artifactory 404 (proxy page,
// plain text, empty body, ...) on an item that storage-info says exists. The first request (the ambiguous one) already counted as
// attempt 1 of notFoundRecheckAttempts.
func (sc *SourceClient) retryAmbiguousProperties(ctx context.Context, manager artifactory.ArtifactoryServicesManager, relativePath string) (*artifactoryutils.ItemProperties, error) {
	for attempt := 2; attempt <= notFoundRecheckAttempts; attempt++ {
		if err := sleepWithContext(ctx, notFoundRecheckBackoff); err != nil {
			return nil, err
		}
		resp, body, err := sendStorageGet(ctx, manager, relativePath, "properties")
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusNotFound {
			return parseItemProperties(resp, body)
		}
		if strings.Contains(string(body), noPropertiesMessage) {
			return nil, nil
		}
		if isArtifactoryJSONNotFound(body) {
			log.Warn(fmt.Sprintf("Properties lookup for %s returned a non-English/unknown 404 message (%s); treating as no properties.", relativePath, truncateForLog(body)))
			return nil, nil
		}
	}
	return nil, fmt.Errorf("received a non-Artifactory 404 from the source properties endpoint for %q %d times, although the item exists; failing the item instead of transferring it without its properties", relativePath, notFoundRecheckAttempts)
}

// confirmItemGone corroborates an ambiguous 404 seen on another endpoint before the caller
// finalizes it as a deletion. A 404 for the item says nothing by itself: a router/Artifactory
// restart or proxy blip answers 404 for every path, for as long as it lasts, so no number
// of re-checks of the item can tell an outage from a deletion. Instead the repository root
// (api/storage/<repo>) is used as a control probe:
//   - root answers 200 with an Artifactory JSON body and the item 404s: the source is healthy and
//     the item is really gone (decided at once, no sleeping);
//   - root 404s, errors, or answers something that is not Artifactory JSON: the source is in an
//     outage window; an error is returned (never "gone") so the item fails and is retried;
//   - item answers 200: the item exists.
//
// The item check runs after the probe. A successful probe is reused for rootProbeCacheTTL, so the
// healthy instant it proves can be up to that long before the item check; an outage that starts
// inside that span and answers the item with an Artifactory-shaped 404 is the residual risk.
func (sc *SourceClient) confirmItemGone(ctx context.Context, manager artifactory.ArtifactoryServicesManager, relativePath string) (bool, error) {
	repoKey := repoKeyOf(relativePath)
	if err := sc.rootProbes.probe(ctx, manager, repoKey); err != nil {
		return false, fmt.Errorf("could not confirm whether %q was deleted, because the source repository %q did not pass the control probe (the source may be restarting or unreachable); treating this as a retryable error, not a deletion: %w", relativePath, repoKey, err)
	}
	resp, body, err := sendConfirmationGet(ctx, manager, relativePath, 0)
	if err != nil {
		return false, err
	}
	if resp.StatusCode == http.StatusNotFound {
		if isArtifactoryJSONNotFound(body) {
			return true, nil
		}
		return false, fmt.Errorf("item %q answered 404 with a body that is not an Artifactory error (%s) although the repository root is healthy; treating this as a retryable error, not a deletion", relativePath, truncateForLog(body))
	}
	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return false, err
	}
	return false, nil
}

func repoKeyOf(relativePath string) string {
	trimmed := strings.TrimPrefix(path.Clean(relativePath), "/")
	return strings.SplitN(trimmed, "/", 2)[0]
}

// rootProbeCache reuses successful repository-root probes for rootProbeCacheTTL and shares the
// probe that is in flight for a repository between all callers that ask meanwhile (the same
// de-duplication golang.org/x/sync/singleflight gives, plus waiter counting: the shared request is
// cancelled when its last waiter gives up, so no probe outlives the callers that need it). Failed
// probes are never stored. The zero value is ready to use.
type rootProbeCache struct {
	mu      sync.Mutex
	okUntil map[string]time.Time
	flights map[string]*rootProbeFlight
}

type rootProbeFlight struct {
	done    chan struct{}
	err     error // valid after done is closed
	waiters int
	cancel  context.CancelFunc
}

func (c *rootProbeCache) freshLocked(repoKey string) bool {
	until, ok := c.okUntil[repoKey]
	return ok && time.Now().Before(until)
}

// probe returns nil if the repository root was proven healthy now or within the TTL.
func (c *rootProbeCache) probe(ctx context.Context, manager artifactory.ArtifactoryServicesManager, repoKey string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	if c.freshLocked(repoKey) {
		c.mu.Unlock()
		return nil
	}
	flight := c.flights[repoKey]
	if flight == nil {
		// The shared request is detached from any single caller's context (one cancelled worker
		// must not fail the probe for the others) and bounded by rootProbeTimeout.
		flightCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rootProbeTimeout)
		flight = &rootProbeFlight{done: make(chan struct{}), cancel: cancel}
		if c.flights == nil {
			c.flights = make(map[string]*rootProbeFlight)
		}
		c.flights[repoKey] = flight
		go c.run(flightCtx, manager, repoKey, flight)
	}
	flight.waiters++
	c.mu.Unlock()

	select {
	case <-flight.done:
		return flight.err
	case <-ctx.Done():
		c.mu.Lock()
		flight.waiters--
		last := flight.waiters == 0
		if last && c.flights[repoKey] == flight {
			delete(c.flights, repoKey) // newcomers start a fresh probe instead of joining a cancelled one
		}
		c.mu.Unlock()
		if last {
			flight.cancel()
			<-flight.done
		}
		return ctx.Err()
	}
}

func (c *rootProbeCache) run(ctx context.Context, manager artifactory.ArtifactoryServicesManager, repoKey string, flight *rootProbeFlight) {
	err := probeRepositoryRoot(ctx, manager, repoKey)
	c.mu.Lock()
	if err == nil && rootProbeCacheTTL > 0 {
		if c.okUntil == nil {
			c.okUntil = make(map[string]time.Time)
		}
		c.okUntil[repoKey] = time.Now().Add(rootProbeCacheTTL)
	}
	if c.flights[repoKey] == flight {
		delete(c.flights, repoKey)
	}
	c.mu.Unlock()
	flight.err = err
	flight.cancel()
	close(flight.done)
}

// probeRepositoryRoot succeeds only if GET api/storage/<repoKey> answers 200 with an Artifactory
// storage-info JSON body (carrying the "repo" field). Only a bounded prefix of the answer is read.
//
// api/repositories/<repoKey> was considered as a cheaper existence check and rejected: it is
// admin-only configuration, answers an empty 400 (not a 404) for unknown keys, and does not know
// the "<remote>-cache" keys that are transferred (artifactory-service RepositoriesResource /
// RestAddonImpl.getRepositoryConfiguration).
func probeRepositoryRoot(ctx context.Context, manager artifactory.ArtifactoryServicesManager, repoKey string) error {
	resp, body, err := sendConfirmationGet(ctx, manager, repoKey, rootProbeMaxBodyBytes)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("repository root probe returned HTTP %d: %s", resp.StatusCode, truncateForLog(body))
	}
	if repo, decodeErr := decodeRepoField(body); decodeErr != nil || repo == "" {
		return fmt.Errorf("repository root probe returned HTTP 200 but not an Artifactory storage-info body (no \"repo\" field in the first %d bytes): %s", rootProbeMaxBodyBytes, truncateForLog(body))
	}
	return nil
}

// decodeRepoField returns the value of the top-level "repo" string of a JSON object, skipping the
// other values token by token (so a huge "children" array is never materialized). It fails on
// truncated or malformed input before the field is reached.
func decodeRepoField(body []byte) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	if tok, err := dec.Token(); err != nil {
		return "", err
	} else if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return "", errors.New("not a JSON object")
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return "", err
		}
		if key, _ := keyTok.(string); key == "repo" {
			var repo string
			if err = dec.Decode(&repo); err != nil {
				return "", err
			}
			return repo, nil
		}
		if err = skipJSONValue(dec); err != nil {
			return "", err
		}
	}
	return "", errors.New("no repo field")
}

func skipJSONValue(dec *json.Decoder) error {
	depth := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if delim, ok := tok.(json.Delim); ok {
			if delim == '{' || delim == '[' {
				depth++
			} else {
				depth--
			}
		}
		if depth == 0 {
			return nil
		}
	}
}

// sendConfirmationGet is sendStorageGet for the existence checks: same request, but the retries
// on 5xx/429/transport errors are bounded (confirmRetryAttempts) and the wait between them is
// cancelled with the context.
func sendConfirmationGet(ctx context.Context, manager artifactory.ArtifactoryServicesManager, relativePath string, maxBodyBytes int64) (*http.Response, []byte, error) {
	fullURL, err := storageURL(manager, relativePath, "")
	if err != nil {
		return nil, nil, err
	}
	attempts := confirmRetryAttempts
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			if err = sleepWithContext(ctx, confirmRetryInterval); err != nil {
				return nil, nil, err
			}
		}
		resp, body, retry, reqErr := doManagerGetOnce(ctx, manager, fullURL, true, maxBodyBytes)
		if !retry {
			return resp, body, reqErr
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, nil, ctxErr
		}
		if reqErr != nil {
			lastErr = reqErr
		} else {
			lastErr = fmt.Errorf("received HTTP %d", resp.StatusCode)
		}
	}
	return nil, nil, fmt.Errorf("GET %s failed after %d attempts: %w", fullURL, attempts, lastErr)
}

// sleepWithContext waits for d, returning early with the context error if ctx is done first.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (sc *SourceClient) fetchItemStatistics(ctx context.Context, manager artifactory.ArtifactoryServicesManager, relativePath string) (*itemStatistics, error) {
	resp, body, err := sendStorageGet(ctx, manager, relativePath, "stats")
	if err != nil {
		if isContextDoneError(ctx, err) {
			return nil, err
		}
		log.Warn("Couldn't get stats for", relativePath+". Reason:", err.Error())
		return nil, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		log.Warn("Couldn't get stats for", relativePath+". Reason:", err.Error())
		return nil, nil
	}

	result := &itemStatistics{}
	if err = json.Unmarshal(body, result); err != nil {
		log.Warn("Couldn't get stats for", relativePath+". Reason:", err.Error())
		return nil, nil
	}
	return result, nil
}

func storageURL(manager artifactory.ArtifactoryServicesManager, relativePath, query string) (string, error) {
	artDetails := manager.GetConfig().GetServiceDetails()
	restAPI := path.Join("api/storage", path.Clean(relativePath))
	fullURL, err := clientutils.BuildUrl(artDetails.GetUrl(), restAPI, map[string]string{})
	if err != nil {
		return "", err
	}
	if query != "" {
		fullURL += "?" + query
	}
	return fullURL, nil
}

func sendStorageGet(ctx context.Context, manager artifactory.ArtifactoryServicesManager, relativePath, query string) (*http.Response, []byte, error) {
	fullURL, err := storageURL(manager, relativePath, query)
	if err != nil {
		return nil, nil, err
	}
	return doManagerGet(ctx, manager, fullURL, true)
}

func doManagerGet(ctx context.Context, manager artifactory.ArtifactoryServicesManager, fullURL string, closeBody bool) (*http.Response, []byte, error) {
	cfg := manager.GetConfig()

	var resp *http.Response
	var body []byte
	// RetryExecutor sleeps between attempts with a plain time.Sleep and only checks ctx
	// between attempts, so cancellation can lag by up to one retry interval. Same behavior
	// as other client-go callers of RetryExecutor.
	retryExecutor := clientutils.RetryExecutor{
		Context:                  ctx,
		MaxRetries:               cfg.GetHttpRetries(),
		RetriesIntervalMilliSecs: cfg.GetHttpRetryWaitMilliSecs(),
		ErrorMessage:             fmt.Sprintf("Failure occurred while sending GET request to %s", fullURL),
		ExecutionHandler: func() (bool, error) {
			var retry bool
			var err error
			resp, body, retry, err = doManagerGetOnce(ctx, manager, fullURL, closeBody, 0)
			return retry, err
		},
	}
	err := retryExecutor.Execute()
	return resp, body, err
}

// doManagerGetOnce performs a single GET. retry reports whether the outcome is worth retrying
// (transport error, 5xx or 429); in that case a non-nil resp has already been drained and closed.
// maxBodyBytes > 0 reads at most that many bytes of the body and closes it without draining the
// rest, so that connection is not reused.
func doManagerGetOnce(ctx context.Context, manager artifactory.ArtifactoryServicesManager, fullURL string, closeBody bool, maxBodyBytes int64) (resp *http.Response, body []byte, retry bool, err error) {
	httpClientsDetails := manager.GetConfig().GetServiceDetails().CreateHttpClientDetails()
	// Refresh an expiring access token before every attempt, as client-go's own Send* does.
	if preErr := manager.GetConfig().GetServiceDetails().RunPreRequestFunctions(&httpClientsDetails); preErr != nil {
		return nil, nil, false, preErr
	}
	httpCli := manager.Client().GetHttpClient()

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if reqErr != nil {
		return nil, nil, !isContextDoneError(ctx, reqErr), reqErr
	}
	applyHttpClientDetails(req, httpClientsDetails)
	resp, doErr := httpCli.GetClient().Do(req)
	if doErr != nil {
		if isContextDoneError(ctx, doErr) {
			return nil, nil, false, doErr
		}
		return nil, nil, true, doErr
	}
	if resp == nil {
		return nil, nil, false, errorutils.CheckErrorf("received empty response from server")
	}
	if shouldRetryHTTPStatus(resp.StatusCode) {
		drainAndClose(resp.Body)
		return resp, nil, true, nil
	}
	if closeBody {
		var readErr error
		if maxBodyBytes > 0 {
			defer func() { _ = resp.Body.Close() }()
			body, readErr = io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		} else {
			defer drainAndClose(resp.Body)
			body, readErr = io.ReadAll(resp.Body)
		}
		if readErr != nil {
			return resp, nil, false, readErr
		}
	}
	return resp, body, false, nil
}

// applyHttpClientDetails mirrors the authentication and User-Agent behavior that
// jfrog-client-go's http/httpclient.HttpClient.doRequest applies via its own
// setAuthentication/addUserAgentHeader helpers, for the raw *http.Client requests this package
// sends outside the service manager (so a per-call ctx can actually abort them). Keep both in
// sync if client-go adds an auth mode or changes those defaults.
// Unlike doRequest, it does not set req.Close: a transfer sends several requests per file to the
// same hosts, and a new TCP+TLS connection for each would dominate the runtime of repositories
// with many small files. Callers drain and close response bodies so connections can be reused.
func applyHttpClientDetails(req *http.Request, details httputils.HttpClientDetails) {
	req.Header.Set("User-Agent", clientutils.GetUserAgent())
	switch {
	case details.ApiKey != "":
		if details.User != "" {
			req.SetBasicAuth(details.User, details.ApiKey)
		} else {
			req.Header.Set("X-JFrog-Art-Api", details.ApiKey)
		}
	case details.AccessToken != "":
		if httpclient.IsApiKey(details.AccessToken) {
			req.SetBasicAuth(details.User, details.AccessToken)
		} else {
			req.Header.Set("Authorization", "Bearer "+details.AccessToken)
		}
	case details.Password != "":
		req.SetBasicAuth(details.User, details.Password)
	}
	for name, value := range details.Headers {
		req.Header.Set(name, value)
	}
}

func shouldRetryHTTPStatus(statusCode int) bool {
	return statusCode >= 500 || statusCode == http.StatusTooManyRequests
}

func isContextDoneError(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return true
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func drainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}

func (sc *SourceClient) GetFileReader(ctx context.Context, file api.FileRepresentation) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	artDetails := sc.streamServiceManager.GetConfig().GetServiceDetails()
	readPath, err := clientutils.BuildUrl(artDetails.GetUrl(), fileRelativePath(file), map[string]string{})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, readPath, nil)
	if err != nil {
		return nil, err
	}
	httpClientsDetails := artDetails.CreateHttpClientDetails()
	// Refresh an expiring access token before the request, as client-go's own Send* does.
	if err = artDetails.RunPreRequestFunctions(&httpClientsDetails); err != nil {
		return nil, err
	}
	applyHttpClientDetails(req, httpClientsDetails)
	resp, err := sc.streamServiceManager.Client().GetHttpClient().GetClient().Do(req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errorutils.CheckErrorf("received empty response from source download")
	}
	if resp.StatusCode == http.StatusNotFound {
		drainAndClose(resp.Body)
		// A content-GET 404 alone is not proof of deletion: a transient 404 window (an
		// Artifactory or router restart) or a blacked-out repo returns it for files that exist. Corroborate with
		// storage-info; only a confirmed absence is "gone", anything else is a retryable
		// failure so the file lands in the errors CSV instead of being silently dropped.
		relativePath := fileRelativePath(file)
		gone, confirmErr := sc.confirmItemGone(ctx, sc.metadataServiceManager, relativePath)
		if confirmErr != nil {
			return nil, fmt.Errorf("received a 404 downloading %q from the source and could not corroborate whether the item exists: %w", relativePath, confirmErr)
		}
		if gone {
			return nil, ErrSourceItemGone
		}
		return nil, fmt.Errorf("received a 404 downloading %q from the source, but the storage-info endpoint says the item exists. This is usually persistent, not transient: storage-info does not check, but a download does, whether the repository is blacked out, whether the path is excluded by the repository's include/exclude patterns, and whether the binary is present in the source filestore. The item is reported as a failure and will fail again on every retry until that is fixed on the source (or the 404 was a one-off blip, in which case a retry succeeds)", relativePath)
	}
	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		drainAndClose(resp.Body)
		return nil, errors.Join(errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK), readErr)
	}
	return &contextReadCloser{ctx: ctx, rc: resp.Body}, nil
}

func fileRelativePath(file api.FileRepresentation) string {
	return path.Join(file.Repo, file.Path, file.Name)
}

func closeReadCloser(rc io.ReadCloser) {
	if rc != nil {
		_ = rc.Close()
	}
}

type contextReadCloser struct {
	ctx       context.Context
	rc        io.ReadCloser
	closeOnce sync.Once
	closeErr  error
}

func (c *contextReadCloser) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		_ = c.Close()
		return 0, err
	}
	return c.rc.Read(p)
}

// Close is idempotent: Read may already have closed rc on ctx cancellation, and callers
// close the reader again on their own error paths, which would otherwise double-close a
// non-idempotent underlying ReadCloser.
func (c *contextReadCloser) Close() error {
	c.closeOnce.Do(func() {
		c.closeErr = c.rc.Close()
	})
	return c.closeErr
}
