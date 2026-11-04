package transferfiles

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/auth/cert"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

const proxyKeyUsageHint = "pass an HTTP or HTTPS proxy URL (for example http://proxy:3128), or configure HTTPS_PROXY and NO_PROXY on the CLI host. " +
	"Note: when --proxy-key is set, it overrides ProxyFromEnvironment for the target, so NO_PROXY is not consulted"

// streamResponseHeaderTimeout bounds only how long a streaming GET/PUT waits for the peer to
// start responding (TCP connect + send the response's status line/headers). It deliberately does
// NOT bound how long transferring a large file's body may take afterward: the streaming service
// managers are built with an overall http.Client.Timeout of 0 (see createStreamingTransferServiceManager),
// so once headers arrive, streaming can run for as long as the file takes. Without this, a peer
// that accepts a TCP connection but never writes any response (a hung reverse proxy, a stalled
// backend, ...) leaves the transfer blocked indefinitely with no way to detect or recover from it.
//
// A var (not a const) so tests can shrink it to avoid a multi-minute-long test run.
var streamResponseHeaderTimeout = 5 * time.Minute

type sourceProxyDescriptor struct {
	Key      string `json:"key"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"` // #nosec G117 -- proxy credential returned by source Artifactory
	Https    bool   `json:"https"`
}

type sourceProxiesResponse struct {
	Proxies []sourceProxyDescriptor `json:"proxies"`
}

// resolveProxyKeyURL dispatches --proxy-key by shape: a value with a URL scheme is parsed
// directly (parseProxyKeyURL); a bare name is looked up against the named proxy configured on
// the source Artifactory instance (lookupSourceProxyURL), matching the plugin-era contract.
func resolveProxyKeyURL(ctx context.Context, raw string, sourceServerDetails *config.ServerDetails) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, errorutils.CheckErrorf("--proxy-key value %q is not a valid URL. %s",
			redactProxyKey(raw), proxyKeyUsageHint)
	}
	if parsed.Scheme != "" {
		return parseProxyKeyURL(raw)
	}
	return lookupSourceProxyURL(ctx, raw, sourceServerDetails)
}

func lookupSourceProxyURL(ctx context.Context, proxyKey string, sourceServerDetails *config.ServerDetails) (*url.URL, error) {
	if sourceServerDetails == nil {
		return nil, namedProxyLookupError(proxyKey, "source Artifactory is not configured")
	}
	serviceManager, err := createTransferServiceManager(ctx, sourceServerDetails, nil)
	if err != nil {
		return nil, namedProxyLookupError(proxyKey, err.Error())
	}
	artDetails := serviceManager.GetConfig().GetServiceDetails()
	proxiesURL, err := clientutils.BuildUrl(artDetails.GetUrl(),
		path.Join("api/system/configuration/platform/proxies"), map[string]string{})
	if err != nil {
		return nil, namedProxyLookupError(proxyKey, err.Error())
	}
	httpDetails := artDetails.CreateHttpClientDetails()
	resp, body, _, err := serviceManager.Client().SendGet(proxiesURL, true, &httpDetails)
	if err != nil {
		return nil, namedProxyLookupError(proxyKey, err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		return nil, namedProxyLookupError(proxyKey, "source Artifactory returned "+resp.Status)
	}

	var proxies sourceProxiesResponse
	if err = json.Unmarshal(body, &proxies); err != nil {
		return nil, namedProxyLookupError(proxyKey, "source Artifactory returned an invalid proxy response")
	}
	for _, proxy := range proxies.Proxies {
		if strings.EqualFold(proxy.Key, proxyKey) {
			proxyURL, buildErr := proxyDescriptorURL(proxy)
			if buildErr != nil {
				return nil, namedProxyLookupError(proxyKey, buildErr.Error())
			}
			return proxyURL, nil
		}
	}
	return nil, namedProxyLookupError(proxyKey, "the key was not found")
}

func proxyDescriptorURL(proxy sourceProxyDescriptor) (*url.URL, error) {
	host := strings.TrimSpace(proxy.Host)
	if host == "" {
		return nil, fmt.Errorf("the source proxy descriptor has no host")
	}
	scheme := "http"
	if proxy.Https {
		scheme = "https"
	}
	if strings.Contains(host, "://") {
		parsedHost, err := url.Parse(host)
		if err != nil || parsedHost.Host == "" || parsedHost.Path != "" && parsedHost.Path != "/" {
			return nil, fmt.Errorf("the source proxy descriptor has an invalid host")
		}
		if parsedHost.Scheme != "http" && parsedHost.Scheme != "https" {
			return nil, fmt.Errorf("the source proxy descriptor uses unsupported scheme %q", parsedHost.Scheme)
		}
		scheme = parsedHost.Scheme
		host = parsedHost.Host
	}
	if proxy.Port > 0 {
		hostname := host
		if parsedHost, err := url.Parse(scheme + "://" + host); err == nil && parsedHost.Hostname() != "" {
			hostname = parsedHost.Hostname()
		}
		host = net.JoinHostPort(hostname, strconv.Itoa(proxy.Port))
	}
	proxyURL := &url.URL{Scheme: scheme, Host: host}
	if isEncryptedProxyPassword(proxy.Password) {
		return nil, fmt.Errorf("the source API returned an encrypted proxy password that the CLI cannot use")
	}
	if proxy.Username != "" && strings.TrimSpace(proxy.Password) == "" {
		return nil, fmt.Errorf("the source API omitted the proxy password; authenticate to source with an access token so the password can be returned")
	}
	if proxy.Username == "" && proxy.Password != "" {
		return nil, fmt.Errorf("the source proxy descriptor has a password but no username")
	}
	if proxy.Username != "" {
		proxyURL.User = url.UserPassword(proxy.Username, proxy.Password)
	}
	return parseProxyKeyURL(proxyURL.String())
}

func isEncryptedProxyPassword(password string) bool {
	upper := strings.ToUpper(strings.TrimSpace(password))
	return strings.HasPrefix(upper, "**ENC**") ||
		strings.HasPrefix(upper, "ENC(") ||
		strings.HasPrefix(upper, "{AES}")
}

func namedProxyLookupError(proxyKey, reason string) error {
	return errorutils.CheckErrorf("could not resolve --proxy-key %q: it was looked up on the source Artifactory, but %s. %s",
		proxyKey, reason, proxyKeyUsageHint)
}

func parseProxyKeyURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, errorutils.CheckErrorf("--proxy-key value %q is not a valid URL. %s",
			redactProxyKey(raw), proxyKeyUsageHint)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" || parsed.Host == "" {
		return nil, errorutils.CheckErrorf("--proxy-key value %q must be a full HTTP or HTTPS proxy URL including scheme (for example http://proxy:3128). %s",
			redactProxyKey(raw), proxyKeyUsageHint)
	}
	return parsed, nil
}

func redactProxyKey(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "<unparseable proxy key>"
	}
	if parsed.User == nil {
		return raw
	}
	return parsed.Redacted()
}

func newTargetProxyTransport(proxyURL *url.URL, serverDetails *config.ServerDetails) (http.RoundTripper, error) {
	transport, err := newStreamingTransport(serverDetails, "--proxy-key")
	if err != nil {
		return nil, err
	}
	transport.Proxy = http.ProxyURL(proxyURL)
	return transport, nil
}

// newDefaultStreamingTransport builds the same bounded-header-wait transport as
// newTargetProxyTransport, but without a --proxy-key override, for the common (no target proxy
// configured) case. Used so the source and target streaming clients always get a
// ResponseHeaderTimeout, not just when --proxy-key is set.
func newDefaultStreamingTransport(serverDetails *config.ServerDetails) (http.RoundTripper, error) {
	return newStreamingTransport(serverDetails, "streaming client")
}

func newStreamingTransport(serverDetails *config.ServerDetails, usageContext string) (*http.Transport, error) {
	insecureTls := false
	if serverDetails != nil {
		insecureTls = serverDetails.InsecureTls
	}
	transport := newDefaultTargetTransport()
	transport.ResponseHeaderTimeout = streamResponseHeaderTimeout

	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, fmt.Errorf("failed resolving JFrog certs dir for %s: %w", usageContext, err)
	}
	transport, err = cert.GetTransportWithLoadedCert(certsPath, insecureTls, transport)
	if err != nil {
		return nil, fmt.Errorf("failed loading TLS certs for %s: %w", usageContext, err)
	}
	if serverDetails != nil {
		err = httpclient.ClientBuilder().
			SetClientCertPath(serverDetails.ClientCertPath).
			SetClientCertKeyPath(serverDetails.ClientCertKeyPath).
			AddClientCertToTransport(transport)
		if err != nil {
			return nil, fmt.Errorf("failed loading client certificate for %s: %w", usageContext, err)
		}
	}
	// GetTransportWithLoadedCert replaces TLSClientConfig and drops MinVersion.
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	}
	transport.TLSClientConfig.MinVersion = tls.VersionTLS12
	return transport, nil
}

func newDefaultTargetTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   httpclient.DefaultDialTimeout,
			KeepAlive: 20 * time.Second,
		}).DialContext,
		MaxIdleConns: 100,
		// All transfer threads talk to the same host; keep enough idle connections for them to
		// be reused instead of the default 2 per host.
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
}

func httpClientWithTransport(transport http.RoundTripper, timeout time.Duration) *http.Client {
	if transport == nil {
		return nil
	}
	return &http.Client{Transport: transport, Timeout: timeout}
}

func (tdc *TransferFilesCommand) targetServiceHTTPClient() *http.Client {
	return httpClientWithTransport(tdc.targetProxyTransport, time.Minute)
}

func (tdc *TransferFilesCommand) httpClientForServer(isTarget bool) *http.Client {
	if !isTarget || tdc.targetProxyTransport == nil {
		return nil
	}
	return tdc.targetServiceHTTPClient()
}
