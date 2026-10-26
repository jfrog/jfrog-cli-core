package transferfiles

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/artifactory"
	artifactoryutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/auth"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	maxPropertyValueLength = 2400
	// Properties whose encoded matrix string exceeds 4000 chars are sent via PATCH
	// (/api/metadata) instead of the deploy URL matrix params, which would make the URL too long.
	maxPropertyEncodedStringLength = 4000
	// itemStatisticsSuffix is Artifactory's internal PUT target for writing download
	// statistics onto an existing item (path + ":statistics"). The target must accept
	// application/xml artifactory.stats bodies on that suffix.
	itemStatisticsSuffix = ":statistics"
	// statsRetries/statsRetryWaitMilliSecs bound the best-effort ApplyStatistics PUT to a short
	// budget instead of inheriting the transfer manager's full retries/wait policy (600 x 5s),
	// which could stall a single item for tens of minutes against a target returning 5xx.
	statsRetries            = 5
	statsRetryWaitMilliSecs = 1000
)

// ErrPermanentTarget marks a target response classified as non-retryable (a 4xx other than
// 429). Wrapped onto the error returned by permanentOrRetryableResponseError so callers can use
// errors.Is instead of matching the "permanent target HTTP" message prefix.
var ErrPermanentTarget = errors.New("permanent target error")

type artifactoryStatsXML struct {
	XMLName          xml.Name `xml:"artifactory.stats"`
	DownloadCount    int64    `xml:"downloadCount"`
	LastDownloaded   int64    `xml:"lastDownloaded"`
	LastDownloadedBy string   `xml:"lastDownloadedBy"`
}

type updateItemPropertiesBody struct {
	Props map[string][]string `json:"props"`
}

type TargetDeployOptions struct {
	BuildInfoRepo             bool
	MinChecksumDeploySize     int64
	CheckExistenceInFilestore bool
	PackageType               string
}

type ChecksumDeployOutcome int

const (
	ChecksumDeployMiss ChecksumDeployOutcome = iota
	ChecksumDeployHit
)

type eligiblePropertiesCacheKey struct {
	metadata    *SourceFileMetadata
	packageType string
}

type eligiblePropertiesCacheValue struct {
	properties   *artifactoryutils.Properties
	skippedLarge bool
	// viaPatch/encoded memoize propertyDelivery's decision for this item, computed once
	// alongside properties instead of being re-derived by each of TryChecksumDeploy, Put,
	// CreateFolder, and ApplyProperties.
	viaPatch bool
	encoded  string
}

type TargetClient struct {
	serverDetails          *config.ServerDetails
	metadataServiceManager artifactory.ArtifactoryServicesManager
	streamServiceManager   artifactory.ArtifactoryServicesManager

	// eligibleCache retains one result per file/folder transfer so concurrent workers
	// cannot evict each other's result between target operations.
	eligibleMu               sync.Mutex
	eligibleCache            map[eligiblePropertiesCacheKey]eligiblePropertiesCacheValue
	filterEligibleProperties func(map[string][]string, TargetDeployOptions) (*artifactoryutils.Properties, bool)
}

func NewTargetClient(ctx context.Context, serverDetails *config.ServerDetails, proxyTransport http.RoundTripper) (*TargetClient, error) {
	metadataServiceManager, err := createMetadataTransferServiceManager(ctx, serverDetails, httpClientWithTransport(proxyTransport, time.Minute))
	if err != nil {
		return nil, err
	}
	streamTransport := proxyTransport
	if streamTransport == nil {
		// No --proxy-key configured: build the same bounded-header-wait transport used for the
		// source streaming client instead of falling through to client-go's default transport,
		// which has no ResponseHeaderTimeout and would hang indefinitely against a non-responding
		// target peer.
		streamTransport, err = newDefaultStreamingTransport(serverDetails)
		if err != nil {
			return nil, err
		}
	}
	streamServiceManager, err := createStreamingTransferServiceManager(ctx, serverDetails, httpClientWithTransport(streamTransport, 0))
	if err != nil {
		return nil, err
	}
	return &TargetClient{
		serverDetails:            serverDetails,
		metadataServiceManager:   metadataServiceManager,
		streamServiceManager:     streamServiceManager,
		eligibleCache:            make(map[eligiblePropertiesCacheKey]eligiblePropertiesCacheValue),
		filterEligibleProperties: filterEligibleProperties,
	}, nil
}

// doManagerRequest sends a fixed-content request bound to ctx (so a per-call ctx cancellation
// actually aborts the in-flight request, unlike the service manager's own long-lived,
// construction-time context) and retries retryable statuses using the manager's configured
// retry budget. content may be nil for a bodyless request; it is safe to resend on retry since
// it isn't consumed like a streaming reader.
func doManagerRequest(ctx context.Context, manager artifactory.ArtifactoryServicesManager, method, fullURL string, content []byte, httpClientsDetails httputils.HttpClientDetails) (*http.Response, []byte, error) {
	cfg := manager.GetConfig()
	return doManagerRequestWithRetry(ctx, manager, method, fullURL, content, httpClientsDetails, cfg.GetHttpRetries(), cfg.GetHttpRetryWaitMilliSecs())
}

// doManagerRequestWithRetry is doManagerRequest with an explicit retry budget, for call sites
// (like ApplyStatistics) that must not inherit the transfer manager's full retry policy.
func doManagerRequestWithRetry(ctx context.Context, manager artifactory.ArtifactoryServicesManager, method, fullURL string, content []byte, httpClientsDetails httputils.HttpClientDetails, maxRetries, retryWaitMilliSecs int) (*http.Response, []byte, error) {
	httpCli := manager.Client().GetHttpClient()

	var resp *http.Response
	var body []byte
	retryExecutor := clientutils.RetryExecutor{
		Context:                  ctx,
		MaxRetries:               maxRetries,
		RetriesIntervalMilliSecs: retryWaitMilliSecs,
		ErrorMessage:             fmt.Sprintf("Failure occurred while sending %s request to %s", method, fullURL),
		ExecutionHandler: func() (bool, error) {
			var bodyReader io.Reader
			if content != nil {
				bodyReader = bytes.NewReader(content)
			}
			// Refresh an expiring access token before every attempt, as client-go's own Send* does.
			if preErr := manager.GetConfig().GetServiceDetails().RunPreRequestFunctions(&httpClientsDetails); preErr != nil {
				return false, preErr
			}
			req, reqErr := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
			if reqErr != nil {
				return !isContextDoneError(ctx, reqErr), reqErr
			}
			if content != nil {
				req.ContentLength = int64(len(content))
			}
			applyHttpClientDetails(req, httpClientsDetails)
			var doErr error
			resp, doErr = httpCli.GetClient().Do(req)
			if doErr != nil {
				if isContextDoneError(ctx, doErr) {
					return false, doErr
				}
				return true, doErr
			}
			if resp == nil {
				return false, errorutils.CheckErrorf("received empty response from target server")
			}
			if shouldRetryHTTPStatus(resp.StatusCode) {
				drainAndClose(resp.Body)
				return true, nil
			}
			defer drainAndClose(resp.Body)
			var readErr error
			body, readErr = io.ReadAll(resp.Body)
			if readErr != nil {
				return false, readErr
			}
			return false, nil
		},
	}
	err := retryExecutor.Execute()
	return resp, body, err
}

func doManagerPut(ctx context.Context, manager artifactory.ArtifactoryServicesManager, fullURL string, content []byte, httpClientsDetails httputils.HttpClientDetails) (*http.Response, []byte, error) {
	return doManagerRequest(ctx, manager, http.MethodPut, fullURL, content, httpClientsDetails)
}

func doManagerPatch(ctx context.Context, manager artifactory.ArtifactoryServicesManager, fullURL string, content []byte, httpClientsDetails httputils.HttpClientDetails) (*http.Response, []byte, error) {
	return doManagerRequest(ctx, manager, http.MethodPatch, fullURL, content, httpClientsDetails)
}

// doManagerPutStream sends a single-attempt streaming PUT bound to ctx. Unlike doManagerPut, it
// never retries: reader is typically the read side of an io.Pipe fed once from a source GET, and
// can't be rewound for a second attempt.
//
// Unlike doManagerPut/doManagerGet (which go through the ArtifactoryServicesManager's own
// JfrogHttpClient.Send* methods), this call issues the request via the raw *http.Client so the
// streaming body can be piped through directly. JfrogHttpClient.Send* runs client-go's
// pre-request interceptors (which proactively refresh an expiring access/refresh token) before
// every request; a raw *http.Client.Do call bypasses that entirely, so a long-running transfer
// whose token expires mid-stream would otherwise fail every subsequent PUT with 401. Run the
// same interceptors here, against the same httpClientsDetails, before building the request.
func doManagerPutStream(ctx context.Context, manager artifactory.ArtifactoryServicesManager, fullURL string, reader io.Reader, size int64, httpClientsDetails httputils.HttpClientDetails) (*http.Response, []byte, error) {
	if err := manager.GetConfig().GetServiceDetails().RunPreRequestFunctions(&httpClientsDetails); err != nil {
		return nil, nil, err
	}
	reqBody := reader
	if size == 0 {
		// A non-nil Body of unknown length makes Go emit Transfer-Encoding: chunked even when
		// ContentLength is explicitly set to 0, since the zero value can't be told apart from
		// "unset". http.NoBody signals "no body" so the request goes out with Content-Length: 0
		// instead, which zero-byte items (they skip checksum-deploy) require on the target.
		reqBody = http.NoBody
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fullURL, reqBody)
	if err != nil {
		return nil, nil, err
	}
	req.ContentLength = size
	applyHttpClientDetails(req, httpClientsDetails)
	resp, err := manager.Client().GetHttpClient().GetClient().Do(req)
	if err != nil {
		return nil, nil, err
	}
	if resp == nil {
		return nil, nil, nil
	}
	defer drainAndClose(resp.Body)
	if isSuccessfulDeployStatusCode(resp.StatusCode) {
		return resp, nil, nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, err
	}
	return resp, body, nil
}

func (tc *TargetClient) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Reimplements jfrog-client-go's PingService.Ping bound to ctx: the service manager's
	// Ping() runs under its own construction-time context, so a per-call ctx cancellation
	// would not abort it.
	pingURL, err := clientutils.BuildUrl(tc.metadataServiceManager.GetConfig().GetServiceDetails().GetUrl(), "api/system/ping", map[string]string{})
	if err != nil {
		return err
	}
	resp, body, err := doManagerGet(ctx, tc.metadataServiceManager, pingURL, true)
	if err != nil {
		return err
	}
	return errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK)
}

// TryChecksumDeploy does not itself guarantee the eligibleCache entry is released: on
// ChecksumDeployMiss the entry must stay cached for the Put call that follows. Callers must
// still eventually call ApplyProperties (which always releases via defer, see
// ReleaseEligibleProperties) even after a ChecksumDeployHit or a permanent/retryable error here.
func (tc *TargetClient) TryChecksumDeploy(ctx context.Context, metadata *SourceFileMetadata, options TargetDeployOptions) (ChecksumDeployOutcome, error) {
	if err := ctx.Err(); err != nil {
		return ChecksumDeployMiss, err
	}
	if !shouldTryChecksumDeploy(metadata, options) {
		return ChecksumDeployMiss, nil
	}
	deployURL, err := tc.buildDeployURL(targetRelativePath(metadata))
	if err != nil {
		return ChecksumDeployMiss, err
	}
	deployURL += tc.propertyMatrixSuffix(metadata, options)
	httpClientsDetails := tc.metadataServiceManager.GetConfig().GetServiceDetails().CreateHttpClientDetails()
	addDeployHeaders(&httpClientsDetails, tc.metadataServiceManager.GetConfig().GetServiceDetails(), metadata, options, true)
	resp, body, err := doManagerPut(ctx, tc.metadataServiceManager, deployURL, nil, httpClientsDetails)
	if err != nil {
		tc.ReleaseEligibleProperties(metadata, options)
		return ChecksumDeployMiss, err
	}
	if isChecksumDeployHit(resp.StatusCode) {
		// No further target call needs the cached eligible properties for this item: the
		// matrix params on this PUT already carried them. Release now rather than waiting on
		// ApplyProperties's deferred release.
		tc.ReleaseEligibleProperties(metadata, options)
		return ChecksumDeployHit, nil
	}
	if isChecksumDeployMiss(resp.StatusCode) {
		return ChecksumDeployMiss, nil
	}
	if err = permanentOrRetryableResponseError(resp, body, http.StatusOK, http.StatusCreated, http.StatusAccepted); err != nil {
		tc.ReleaseEligibleProperties(metadata, options)
		return ChecksumDeployMiss, err
	}
	return ChecksumDeployMiss, nil
}

// Put does not itself release the eligibleCache entry on success: the caller must still call
// ApplyProperties afterward, which always releases it via defer (see
// ReleaseEligibleProperties). On error, Put releases it directly since ApplyProperties is not
// expected to run.
func (tc *TargetClient) Put(ctx context.Context, metadata *SourceFileMetadata, reader io.Reader, options TargetDeployOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	deployURL, err := tc.buildDeployURL(targetRelativePath(metadata))
	if err != nil {
		return err
	}
	deployURL += tc.propertyMatrixSuffix(metadata, options)
	httpClientsDetails := tc.streamServiceManager.GetConfig().GetServiceDetails().CreateHttpClientDetails()
	addDeployHeaders(&httpClientsDetails, tc.streamServiceManager.GetConfig().GetServiceDetails(), metadata, options, false)
	// Skip artifactoryutils.UploadFileFromReader: it always AddChecksumHeaders including
	// X-Checksum-Md5 (even when Md5 is empty). Plain PUT must omit that header; it is
	// reserved for CheckExistenceInFilestore checksum-deploy. Use doManagerPutStream instead of
	// SendPut/UploadFileFromReader so the request is bound to this call's ctx (not just the
	// service manager's construction-time context) and can actually be aborted mid-flight.
	resp, body, err := doManagerPutStream(ctx, tc.streamServiceManager, deployURL, reader, metadata.Size, httpClientsDetails)
	if err != nil {
		tc.ReleaseEligibleProperties(metadata, options)
		return err
	}
	if resp == nil {
		tc.ReleaseEligibleProperties(metadata, options)
		return errorutils.CheckErrorf("received empty response from target deploy")
	}
	if isSuccessfulDeployStatusCode(resp.StatusCode) {
		return nil
	}
	tc.ReleaseEligibleProperties(metadata, options)
	return permanentOrRetryableResponseError(resp, body, http.StatusOK, http.StatusCreated, http.StatusAccepted)
}

func (tc *TargetClient) CreateFolder(ctx context.Context, metadata *SourceFileMetadata, options TargetDeployOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	deployURL, err := tc.buildDeployURL(targetRelativePath(metadata))
	if err != nil {
		return err
	}
	deployURL += tc.propertyMatrixSuffix(metadata, options)
	httpClientsDetails := tc.metadataServiceManager.GetConfig().GetServiceDetails().CreateHttpClientDetails()
	addIdentityHeaders(&httpClientsDetails, metadata)
	resp, body, err := doManagerPut(ctx, tc.metadataServiceManager, deployURL, nil, httpClientsDetails)
	if err != nil {
		tc.ReleaseEligibleProperties(metadata, options)
		return err
	}
	if isSuccessfulDeployStatusCode(resp.StatusCode) {
		return nil
	}
	tc.ReleaseEligibleProperties(metadata, options)
	return permanentOrRetryableResponseError(resp, body, http.StatusOK, http.StatusCreated, http.StatusAccepted)
}

func (tc *TargetClient) ApplyProperties(ctx context.Context, metadata *SourceFileMetadata, options TargetDeployOptions) (skippedLargeProps bool, err error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	entry := tc.eligiblePropertiesEntry(metadata, options)
	defer tc.ReleaseEligibleProperties(metadata, options)
	if !entry.viaPatch {
		// Short, path-safe maps are applied as matrix params on checksum/full PUT.
		return entry.skippedLarge, nil
	}
	eligibleProps := entry.properties
	relativePath := strings.TrimSuffix(targetRelativePath(metadata), "/")
	return entry.skippedLarge, tc.applyPropertiesViaPatch(ctx, relativePath, eligibleProps)
}

func (tc *TargetClient) applyPropertiesViaPatch(ctx context.Context, relativePath string, eligibleProps *artifactoryutils.Properties) error {
	setPropertiesURL, err := clientutils.BuildUrl(tc.metadataServiceManager.GetConfig().GetServiceDetails().GetUrl(), path.Join("api", "metadata", relativePath), map[string]string{})
	if err != nil {
		return err
	}
	requestBody, err := json.Marshal(updateItemPropertiesBody{Props: eligibleProps.ToMap()})
	if err != nil {
		return errorutils.CheckError(err)
	}
	httpClientsDetails := tc.metadataServiceManager.GetConfig().GetServiceDetails().CreateHttpClientDetails()
	httpClientsDetails.SetContentTypeApplicationJson()
	resp, body, err := doManagerPatch(ctx, tc.metadataServiceManager, setPropertiesURL, requestBody, httpClientsDetails)
	if err != nil {
		return err
	}
	return permanentOrRetryableResponseError(resp, body, http.StatusNoContent)
}

func (tc *TargetClient) ApplyStatistics(ctx context.Context, metadata *SourceFileMetadata, options TargetDeployOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if skipsDownloadStatistics(options) || metadata.Name == "" || !hasDownloadStatistics(metadata) {
		return nil
	}
	relativePath := strings.TrimSuffix(targetRelativePath(metadata), "/")
	deployURL, err := tc.buildDeployURL(relativePath)
	if err != nil {
		log.Warn("Couldn't set stats for", relativePath+". Reason:", err.Error())
		return nil
	}
	statsXML, err := xml.Marshal(artifactoryStatsXML{
		DownloadCount:    metadata.DownloadCount,
		LastDownloaded:   metadata.LastDownloaded,
		LastDownloadedBy: metadata.LastDownloadedBy,
	})
	if err != nil {
		log.Warn("Couldn't set stats for", relativePath+". Reason:", err.Error())
		return nil
	}
	httpClientsDetails := tc.metadataServiceManager.GetConfig().GetServiceDetails().CreateHttpClientDetails()
	httpClientsDetails.AddHeader("Content-Type", "application/xml")
	resp, body, err := doManagerRequestWithRetry(
		ctx, tc.metadataServiceManager, http.MethodPut, deployURL+itemStatisticsSuffix, statsXML, httpClientsDetails,
		statsRetries, statsRetryWaitMilliSecs,
	)
	if err != nil {
		log.Warn("Couldn't set stats for", relativePath+". Reason:", err.Error())
		return nil
	}
	if err = permanentOrRetryableResponseError(resp, body, http.StatusOK, http.StatusCreated, http.StatusNoContent); err != nil {
		log.Warn("Couldn't set stats for", relativePath+". Reason:", err.Error())
		return nil
	}
	log.Debug("Applied download statistics for", targetRelativePath(metadata))
	return nil
}

// eligiblePropertiesEntry returns the full memoized entry (filtered properties plus the
// propertyDelivery decision) for one file/folder transfer, computing both only once. The
// metadata pointer is the transfer identity shared by all target operations.
func (tc *TargetClient) eligiblePropertiesEntry(metadata *SourceFileMetadata, options TargetDeployOptions) eligiblePropertiesCacheValue {
	cacheKey := eligiblePropertiesCacheKey{metadata: metadata, packageType: options.PackageType}
	tc.eligibleMu.Lock()
	if cached, ok := tc.eligibleCache[cacheKey]; ok {
		tc.eligibleMu.Unlock()
		return cached
	}
	tc.eligibleMu.Unlock()

	eligible, skipped := tc.filterEligibleProperties(metadata.Properties, options)
	viaPatch, encoded := propertyDelivery(eligible)
	entry := eligiblePropertiesCacheValue{properties: eligible, skippedLarge: skipped, viaPatch: viaPatch, encoded: encoded}

	tc.eligibleMu.Lock()
	defer tc.eligibleMu.Unlock()
	if cached, ok := tc.eligibleCache[cacheKey]; ok {
		return cached
	}
	tc.eligibleCache[cacheKey] = entry
	return entry
}

// eligibleProperties returns the filtered property set for one file/folder transfer.
func (tc *TargetClient) eligibleProperties(metadata *SourceFileMetadata, options TargetDeployOptions) (*artifactoryutils.Properties, bool) {
	entry := tc.eligiblePropertiesEntry(metadata, options)
	return entry.properties, entry.skippedLarge
}

// HasEligibleProperties reports whether metadata has at least one property eligible for
// transfer, reusing the same memoized filter result as ApplyProperties instead of re-running
// filterEligibleProperties.
func (tc *TargetClient) HasEligibleProperties(metadata *SourceFileMetadata, options TargetDeployOptions) bool {
	eligible, _ := tc.eligibleProperties(metadata, options)
	return eligible.KeysLen() > 0
}

// ReleaseEligibleProperties drops the cached eligible properties for one transfer item.
// Callers that never reach ApplyProperties should defer this after the metadata pointer is known.
func (tc *TargetClient) ReleaseEligibleProperties(metadata *SourceFileMetadata, options TargetDeployOptions) {
	if metadata == nil {
		return
	}
	tc.eligibleMu.Lock()
	defer tc.eligibleMu.Unlock()
	delete(tc.eligibleCache, eligiblePropertiesCacheKey{metadata: metadata, packageType: options.PackageType})
}

func hasDownloadStatistics(metadata *SourceFileMetadata) bool {
	return metadata.DownloadCount > 0 || metadata.LastDownloaded > 0 || metadata.LastDownloadedBy != ""
}

// skipsDownloadStatistics matches Artifactory info repos (BuildInfo, PipeInfo). Those
// stores ingest JSON metadata and reject PUT path:statistics with application/xml (415).
// Distribution, Support, and ReleaseBundles accept statistics (lab 201) and are not skipped.
func skipsDownloadStatistics(options TargetDeployOptions) bool {
	if options.BuildInfoRepo {
		return true
	}
	switch strings.ToLower(options.PackageType) {
	case "buildinfo", "pipeinfo":
		return true
	default:
		return false
	}
}

func (tc *TargetClient) buildDeployURL(relativePath string) (string, error) {
	return clientutils.BuildUrl(tc.metadataServiceManager.GetConfig().GetServiceDetails().GetUrl(), relativePath, map[string]string{})
}

func targetRelativePath(metadata *SourceFileMetadata) string {
	if metadata.Name == "" {
		folderPath := path.Join(metadata.Repo, metadata.Path)
		if !strings.HasSuffix(folderPath, "/") {
			folderPath += "/"
		}
		return folderPath
	}
	if metadata.Path == "." {
		return path.Join(metadata.Repo, metadata.Name)
	}
	return path.Join(metadata.Repo, metadata.Path, metadata.Name)
}

func shouldTryChecksumDeploy(metadata *SourceFileMetadata, options TargetDeployOptions) bool {
	if options.BuildInfoRepo || metadata.Name == "" {
		return false
	}
	if metadata.Sha1 == "" {
		return false
	}
	return metadata.Size >= options.MinChecksumDeploySize
}

func addDeployHeaders(httpClientsDetails *httputils.HttpClientDetails, serviceDetails auth.ServiceDetails, metadata *SourceFileMetadata, options TargetDeployOptions, checksumDeploy bool) {
	addIdentityHeaders(httpClientsDetails, metadata)
	if httpClientsDetails.Headers == nil {
		httpClientsDetails.Headers = make(map[string]string)
	}
	// Always send sha1/sha256 on deploy. X-Checksum-Md5 is reserved for the
	// CheckExistenceInFilestore checksum-deploy path only (below).
	if metadata.Sha1 != "" {
		httpClientsDetails.Headers["X-Checksum-Sha1"] = metadata.Sha1
	}
	if metadata.Sha256 != "" {
		httpClientsDetails.Headers["X-Checksum"] = metadata.Sha256
	}
	artifactoryutils.AddAuthHeaders(httpClientsDetails.Headers, serviceDetails)
	if checksumDeploy {
		httpClientsDetails.AddHeader("X-Checksum-Deploy", "true")
		if options.CheckExistenceInFilestore {
			httpClientsDetails.AddHeader("X-Check-Binary-Existence-In-Filestore", "true")
			httpClientsDetails.AddHeader("X-Checksum-Md5", metadata.Md5)
			httpClientsDetails.AddHeader("X-Binary-Size", fmt.Sprintf("%d", metadata.Size))
		}
	}
}

func addIdentityHeaders(httpClientsDetails *httputils.HttpClientDetails, metadata *SourceFileMetadata) {
	if created, ok := artifactoryTimestampHeaderValue(metadata.Created); ok {
		httpClientsDetails.AddHeader("X-Artifactory-Created", created)
	}
	if metadata.CreatedBy != "" {
		httpClientsDetails.AddHeader("X-Artifactory-Created-By", metadata.CreatedBy)
	}
	if lastModified, ok := artifactoryTimestampHeaderValue(metadata.LastModified); ok {
		httpClientsDetails.AddHeader("X-Artifactory-Last-Modified", lastModified)
	}
	if metadata.ModifiedBy != "" {
		httpClientsDetails.AddHeader("X-Artifactory-Modified-By", metadata.ModifiedBy)
	}
}

func artifactoryTimestampHeaderValue(timestamp string) (string, bool) {
	if timestamp == "" {
		return "", false
	}
	if _, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
		return timestamp, true
	}
	parsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return "", false
	}
	return strconv.FormatInt(parsed.UnixMilli(), 10), true
}

func filterEligibleProperties(properties map[string][]string, options TargetDeployOptions) (*artifactoryutils.Properties, bool) {
	eligibleProps := artifactoryutils.NewProperties()
	skippedLargeProps := false
	for key, values := range properties {
		if isGeneratedPropertyKey(key, options.PackageType) {
			continue
		}
		for _, value := range values {
			if len(value) > maxPropertyValueLength {
				skippedLargeProps = true
				continue
			}
			eligibleProps.AddProperty(key, value)
		}
	}
	return eligibleProps, skippedLargeProps
}

// isPathUnsafePropertyValue reports characters that Properties.ToEncodedString cannot make
// matrix-param-safe even after url.QueryEscape: a newline/CR breaks the HTTP request line, "#"
// starts a URL fragment, "," and "\" are the multi-value/escape separators ToEncodedString
// itself parses back out, and "|" is unsafe for some reverse proxies in front of Artifactory.
// ";" and "=" are matrix-param separators too, but ToEncodedString escapes them via
// url.QueryEscape like any other value byte (-> %3B / %3D), so they round-trip safely and are
// deliberately not included here.
func isPathUnsafePropertyValue(value string) bool {
	return strings.ContainsAny(value, "\n\r#,\\|")
}

func containsPathUnsafePropertyValue(properties *artifactoryutils.Properties) bool {
	for key, values := range properties.ToMap() {
		if isPathUnsafePropertyValue(key) {
			return true
		}
		for _, value := range values {
			if isPathUnsafePropertyValue(value) {
				return true
			}
		}
	}
	return false
}

// propertyDelivery is the single decision for matrix-param vs PATCH. Put,
// checksum-deploy, and ApplyProperties must not re-derive these conditions.
func propertyDelivery(eligibleProps *artifactoryutils.Properties) (viaPatch bool, encoded string) {
	if eligibleProps == nil || eligibleProps.KeysLen() == 0 {
		return false, ""
	}
	encoded = eligibleProps.ToEncodedString(false)
	if encoded == "" {
		return false, ""
	}
	if len(encoded) > maxPropertyEncodedStringLength || containsPathUnsafePropertyValue(eligibleProps) {
		return true, encoded
	}
	return false, encoded
}

func (tc *TargetClient) propertyMatrixSuffix(metadata *SourceFileMetadata, options TargetDeployOptions) string {
	if metadata == nil {
		return ""
	}
	entry := tc.eligiblePropertiesEntry(metadata, options)
	return formatPropertyMatrixSuffix(entry.viaPatch, entry.encoded)
}

// propertyMatrixSuffixFromEligible re-derives the propertyDelivery decision from a Properties
// set that was not obtained via TargetClient.eligiblePropertiesEntry, so its result is not
// memoized. Production call sites go through the TargetClient method above instead.
func propertyMatrixSuffixFromEligible(eligibleProps *artifactoryutils.Properties) string {
	viaPatch, encoded := propertyDelivery(eligibleProps)
	return formatPropertyMatrixSuffix(viaPatch, encoded)
}

func formatPropertyMatrixSuffix(viaPatch bool, encoded string) string {
	if viaPatch || encoded == "" {
		return ""
	}
	return ";" + encoded
}

func isChecksumDeployHit(statusCode int) bool {
	return statusCode == http.StatusOK || statusCode == http.StatusCreated || statusCode == http.StatusAccepted
}

// Only a missing blob (404) falls through to a full-body PUT.
// HTTP 409 on checksum-deploy means the target already has a different checksum at
// this path — this is a hard failure; we do not overwrite.
func isChecksumDeployMiss(statusCode int) bool {
	return statusCode == http.StatusNotFound
}

func isSuccessfulDeployStatusCode(statusCode int) bool {
	return statusCode == http.StatusOK || statusCode == http.StatusCreated || statusCode == http.StatusAccepted
}

// isPermanentHTTPStatus reports 4xx responses other than 429 as non-retryable.
// The shared HTTP client already skips retries for these codes; this helper surfaces
// that classification at the transfer-files call site.
func isPermanentHTTPStatus(statusCode int) bool {
	return statusCode >= 400 && statusCode < 500 && statusCode != http.StatusTooManyRequests
}

func permanentOrRetryableResponseError(resp *http.Response, body []byte, expectedStatusCodes ...int) error {
	err := errorutils.CheckResponseStatusWithBody(resp, body, expectedStatusCodes...)
	if err == nil {
		return nil
	}
	if resp != nil && isPermanentHTTPStatus(resp.StatusCode) {
		return fmt.Errorf("permanent target HTTP %d: %w: %w", resp.StatusCode, ErrPermanentTarget, err)
	}
	return err
}
