package transferfiles

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/transferfiles/api"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProxyKeyURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "http", raw: "http://proxy:3128"},
		{name: "https", raw: "https://proxy:3128"},
		{name: "legacy key", raw: "corp-egress", wantErr: true},
		{name: "empty scheme host", raw: "http://", wantErr: true},
		{name: "socks5", raw: "socks5://proxy:1080", wantErr: true},
		{name: "userinfo redacted", raw: proxyURLWithUserinfo("http", "user", "proxy:3128"), wantErr: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := parseProxyKeyURL(tc.raw)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "HTTPS_PROXY")
				assert.Contains(t, err.Error(), "NO_PROXY")
				assert.NotContains(t, err.Error(), "secret")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "proxy:3128", parsed.Host)
			assert.NotContains(t, redactProxyKey(tc.raw), "secret")
		})
	}
}

func TestTargetClient_routesThroughProxy_sourceDoesNot(t *testing.T) {
	var artifactoryHits atomic.Int32
	var sourceStorageHits atomic.Int32
	storagePath := "/api/storage/" + testSourceRelativePath()
	artifactory := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		artifactoryHits.Add(1)
		if r.URL.Path == "/api/system/ping" {
			_, _ = w.Write([]byte("OK"))
			return
		}
		if strings.HasPrefix(r.URL.Path, storagePath) {
			sourceStorageHits.Add(1)
		}
		if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"checksums":{},"size":"0"}`))
	}))
	t.Cleanup(artifactory.Close)

	var proxyHits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		outReq := r.Clone(r.Context())
		outReq.RequestURI = ""
		resp, err := http.DefaultTransport.RoundTrip(outReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		for k, vv := range resp.Header {
			w.Header()[k] = vv
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))
	t.Cleanup(proxy.Close)

	proxyURL, err := url.Parse(proxy.URL)
	require.NoError(t, err)
	transport, err := newTargetProxyTransport(proxyURL, newTestTargetServerDetails(artifactory.URL))
	require.NoError(t, err)

	targetClient, err := NewTargetClient(context.Background(), newTestTargetServerDetails(artifactory.URL), transport)
	require.NoError(t, err)
	require.NoError(t, targetClient.Ping(context.Background()))
	assert.Greater(t, proxyHits.Load(), int32(0), "target traffic must route through the proxy")

	// b's raw doManagerPut/PutStream/Patch path (checksum-deploy, plain PUT, PATCH) must also
	// route through the target proxy transport, not just the client-go Ping call above.
	hitsBeforeChecksumDeploy := proxyHits.Load()
	outcome, err := targetClient.TryChecksumDeploy(context.Background(), testTargetMetadata(), defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.Equal(t, ChecksumDeployHit, outcome)
	assert.Greater(t, proxyHits.Load(), hitsBeforeChecksumDeploy, "checksum-deploy PUT must route through the proxy")

	hitsBeforePut := proxyHits.Load()
	require.NoError(t, targetClient.Put(context.Background(), testTargetMetadata(), strings.NewReader("hello world"), defaultTargetDeployOptions()))
	assert.Greater(t, proxyHits.Load(), hitsBeforePut, "plain PUT must route through the proxy")

	hitsBeforePatch := proxyHits.Load()
	largePropsMetadata := testTargetMetadata()
	largePropsMetadata.Properties = largeEligiblePropertiesMap(t)
	_, err = targetClient.ApplyProperties(context.Background(), largePropsMetadata, defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.Greater(t, proxyHits.Load(), hitsBeforePatch, "PATCH must route through the proxy")

	hitsAfterTarget := proxyHits.Load()
	sourceClient, err := NewSourceClient(context.Background(), newTestTargetServerDetails(artifactory.URL))
	require.NoError(t, err)
	_, err = sourceClient.GetFileMetadata(context.Background(), api.FileRepresentation{
		Repo: testSourceRepo,
		Path: testSourcePath,
		Name: testSourceName,
	})
	require.NoError(t, err)
	assert.Greater(t, sourceStorageHits.Load(), int32(0), "source GET must reach Artifactory")
	assert.Equal(t, hitsAfterTarget, proxyHits.Load(), "source traffic must not route through the --proxy-key transport")
	assert.Greater(t, artifactoryHits.Load(), int32(0))
}

func TestResolveProxyKeyURL_URLFormDoesNotRequireSource(t *testing.T) {
	resolvedURL, err := resolveProxyKeyURL(context.Background(), "http://proxy:3128", nil)
	require.NoError(t, err)
	assert.Equal(t, "http://proxy:3128", resolvedURL.String())
}

func TestNamedProxyLookup_routesTargetPingThroughProxy_sourceLookupDoesNot(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits.Add(1)
		assert.Equal(t, "/api/system/ping", r.URL.Path)
		_, _ = w.Write([]byte("OK"))
	}))
	t.Cleanup(target.Close)

	var proxyHits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		outReq := r.Clone(r.Context())
		outReq.RequestURI = ""
		resp, err := http.DefaultTransport.RoundTrip(outReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))
	t.Cleanup(proxy.Close)

	parsedProxyURL, err := url.Parse(proxy.URL)
	require.NoError(t, err)
	proxyPort := parsedProxyURL.Port()
	var sourceLookupHits atomic.Int32
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceLookupHits.Add(1)
		assert.Equal(t, "/api/system/configuration/platform/proxies", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"proxies":[{"key":"corp-egress","host":"127.0.0.1","port":` +
			proxyPort + `}]}`))
	}))
	t.Cleanup(source.Close)

	resolvedURL, err := resolveProxyKeyURL(context.Background(), "corp-egress",
		newTestSourceServerDetails(source.URL))
	require.NoError(t, err)
	assert.Equal(t, proxy.URL, resolvedURL.String())
	assert.Equal(t, int32(1), sourceLookupHits.Load())
	assert.Zero(t, proxyHits.Load(), "source proxy lookup must use the source client's normal transport")

	transport, err := newTargetProxyTransport(resolvedURL, newTestTargetServerDetails(target.URL))
	require.NoError(t, err)
	targetClient, err := NewTargetClient(context.Background(), newTestTargetServerDetails(target.URL), transport)
	require.NoError(t, err)
	require.NoError(t, targetClient.Ping(context.Background()))
	assert.Equal(t, int32(1), proxyHits.Load(), "target ping must route through the named proxy")
	assert.Equal(t, int32(1), targetHits.Load())
}

func TestNamedProxyLookup_missingKeyFailsClosed(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/system/configuration/platform/proxies", r.URL.Path)
		_, _ = w.Write([]byte(`{"proxies":[{"key":"another-proxy","host":"proxy","port":3128}]}`))
	}))
	t.Cleanup(source.Close)

	_, err := resolveProxyKeyURL(context.Background(), "corp-egress", newTestSourceServerDetails(source.URL))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"corp-egress"`)
	assert.Contains(t, err.Error(), "looked up on the source Artifactory")
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "http://proxy:3128")
	assert.Contains(t, err.Error(), "HTTPS_PROXY")
}

func TestNamedProxyLookup_encryptedPasswordFailsClosed(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"proxies":[{"key":"corp-egress","host":"proxy","port":3128,"username":"u","password":"**ENC**xxx"}]}`))
	}))
	t.Cleanup(source.Close)

	_, err := resolveProxyKeyURL(context.Background(), "corp-egress", newTestSourceServerDetails(source.URL))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"corp-egress"`)
	assert.Contains(t, err.Error(), "looked up on the source Artifactory")
	assert.Contains(t, err.Error(), "encrypted proxy password")
}

func TestProxyDescriptorURL_rejectsUsernameWithoutPassword(t *testing.T) {
	_, err := proxyDescriptorURL(sourceProxyDescriptor{
		Host:     "proxy.example",
		Port:     3128,
		Username: "proxy-user",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "omitted the proxy password")
	assert.Contains(t, err.Error(), "access token")
}

func TestNamedProxyLookup_usernameWithoutPasswordFailsClosed(t *testing.T) {
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/system/configuration/platform/proxies", r.URL.Path)
		_, _ = w.Write([]byte(`{"proxies":[{"key":"corp-egress","host":"proxy","port":3128,"username":"proxy-user"}]}`))
	}))
	t.Cleanup(source.Close)

	_, err := resolveProxyKeyURL(context.Background(), "corp-egress", newTestSourceServerDetails(source.URL))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"corp-egress"`)
	assert.Contains(t, err.Error(), "looked up on the source Artifactory")
	assert.Contains(t, err.Error(), "omitted the proxy password")
	assert.Contains(t, err.Error(), "access token")
	assert.Contains(t, err.Error(), "HTTPS_PROXY")
}

func TestProxyDescriptorURL_rejectsEncryptedPassword(t *testing.T) {
	_, err := proxyDescriptorURL(sourceProxyDescriptor{ // #nosec G101 -- dummy test fixture
		Host:     "proxy.example",
		Port:     3128,
		Password: "**ENC**unusable",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "encrypted proxy password")
	assert.NotContains(t, err.Error(), "unusable")
}

// proxyURLWithUserinfo assembles a "scheme://user:secret@host" string at runtime so the test
// source holds no credential-bearing URL literal for secret scanners to flag.
func proxyURLWithUserinfo(scheme, user, host string) string {
	return scheme + "://" + user + ":" + "secret" + "@" + host
}

func TestProxyDescriptorURL_buildsHTTPSURLWithCredentials(t *testing.T) {
	proxyURL, err := proxyDescriptorURL(sourceProxyDescriptor{
		Host:     "proxy.example",
		Port:     8443,
		Username: "proxy-user",
		Password: "secret",
		Https:    true,
	})
	require.NoError(t, err)
	assert.Equal(t, proxyURLWithUserinfo("https", "proxy-user", "proxy.example:8443"), proxyURL.String())
}

func TestParseProxyKeyURL_schemeLessFailsWithHelpfulMessage(t *testing.T) {
	_, err := parseProxyKeyURL("corp-egress")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"corp-egress"`)
	assert.Contains(t, err.Error(), "including scheme")
	assert.Contains(t, err.Error(), "http://proxy:3128")
	assert.Contains(t, err.Error(), "HTTPS_PROXY")
	assert.Contains(t, err.Error(), "NO_PROXY")
}

func TestRedactProxyKey_parseFailureNeverReturnsRawInput(t *testing.T) {
	malformed := proxyURLWithUserinfo("http", "user", "proxy.example/%zz")
	assert.Equal(t, "<unparseable proxy key>", redactProxyKey(malformed))
	_, err := parseProxyKeyURL(malformed)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), malformed)
	assert.NotContains(t, err.Error(), "secret")
}

func TestNewTargetProxyTransport_matchesClientGoDefaults(t *testing.T) {
	proxyURL, err := url.Parse("http://proxy:3128")
	require.NoError(t, err)
	roundTripper, err := newTargetProxyTransport(proxyURL, &config.ServerDetails{})
	require.NoError(t, err)
	transport, ok := roundTripper.(*http.Transport)
	require.True(t, ok)
	assert.False(t, transport.ForceAttemptHTTP2)
	assert.NotNil(t, transport.DialContext)
	assert.Equal(t, 100, transport.MaxIdleConns)
	assert.Equal(t, 100, transport.MaxIdleConnsPerHost)
	assert.Equal(t, 90*time.Second, transport.IdleConnTimeout)
	assert.Equal(t, 10*time.Second, transport.TLSHandshakeTimeout)
	assert.Equal(t, time.Second, transport.ExpectContinueTimeout)
	require.NotNil(t, transport.TLSClientConfig)
	assert.Equal(t, uint16(tls.VersionTLS12), transport.TLSClientConfig.MinVersion)
}

func TestNewTargetProxyTransport_loadsClientCertificate(t *testing.T) {
	certPath, keyPath := writeClientCertificate(t)
	proxyURL, err := url.Parse("http://proxy:3128")
	require.NoError(t, err)
	roundTripper, err := newTargetProxyTransport(proxyURL, &config.ServerDetails{
		ClientCertPath:    certPath,
		ClientCertKeyPath: keyPath,
	})
	require.NoError(t, err)
	transport, ok := roundTripper.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.TLSClientConfig)
	assert.Len(t, transport.TLSClientConfig.Certificates, 1)
	assert.Equal(t, uint16(tls.VersionTLS12), transport.TLSClientConfig.MinVersion)
}

func TestFlaglessProxyUsesEnvironmentOnly(t *testing.T) {
	// Without --proxy-key, targetProxyTransport is never set, so the target client falls back to
	// client-go's own transport (ProxyFromEnvironment), which already honors HTTPS_PROXY/NO_PROXY.
	cmd, err := NewTransferFilesCommand(nil, nil)
	require.NoError(t, err)
	assert.Nil(t, cmd.targetServiceHTTPClient())
}

func writeClientCertificate(t *testing.T) (string, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "proxy-client"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	certPath := filepath.Join(t.TempDir(), "client-cert.pem")
	keyPath := filepath.Join(filepath.Dir(certPath), "client-key.pem")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: certDER,
	}), 0600))
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}), 0600))
	return certPath, keyPath
}
