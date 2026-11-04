package transferfiles

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

	transferutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	artifactoryutils "github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTargetRepo             = testSourceRepo
	canonicalUbuntuDescription = "The Ubuntu container image maintained by Canonical\n\n" +
		"Ubuntu is a Debian-based Linux operating system that runs from the desktop to the cloud, " +
		"to all your internet connected things.\n" +
		"It is the world's most popular operating system across public clouds and OpenStack clouds.\n" +
		"It is the number one platform for containers; from Docker to Kubernetes to LXD, " +
		"Ubuntu can run your containers at scale.\n" +
		"Fast, secure and simple, Ubuntu powers millions of PCs worldwide.\n"
)

func propertyMatrixSuffix(metadata *SourceFileMetadata, options TargetDeployOptions) string {
	eligibleProps, _ := filterEligibleProperties(metadata.Properties, options)
	return propertyMatrixSuffixFromEligible(eligibleProps)
}

func testTargetMetadata() *SourceFileMetadata {
	return &SourceFileMetadata{
		Repo:         testTargetRepo,
		Path:         testSourcePath,
		Name:         testSourceName,
		Size:         11,
		Sha1:         "sha1-value",
		Sha256:       "sha256-value",
		Md5:          "md5-value",
		Created:      "2020-01-01T00:00:00.000Z",
		CreatedBy:    "admin",
		LastModified: "2020-01-02T00:00:00.000Z",
		ModifiedBy:   "deployer",
		Properties: map[string][]string{
			"build.name": {"app"},
			"env":        {"prod", "release"},
		},
	}
}

func testTargetRelativePath() string {
	return testSourceRelativePath()
}

func defaultTargetDeployOptions() TargetDeployOptions {
	return TargetDeployOptions{
		MinChecksumDeploySize: 10,
	}
}

func newTestTargetServerDetails(serverURL string) *config.ServerDetails {
	return &config.ServerDetails{
		Url:            serverURL + "/",
		ArtifactoryUrl: serverURL + "/",
	}
}

func TestTargetClient_Ping_usesSystemPing(t *testing.T) {
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	err = client.Ping(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "/api/system/ping", requestPath)
}

func TestTargetClient_TryChecksumDeploy_hit_sendsNoBody(t *testing.T) {
	deployPath := "/" + testTargetRelativePath()
	var requestMethod string
	var requestPath string
	var checksumDeployHeader string
	var sha1Header string
	var bodyBytes []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMethod = r.Method
		requestPath = r.URL.Path
		checksumDeployHeader = r.Header.Get("X-Checksum-Deploy")
		sha1Header = r.Header.Get("X-Checksum-Sha1")
		if r.Body != nil {
			bodyBytes, _ = io.ReadAll(r.Body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	outcome, err := client.TryChecksumDeploy(context.Background(), testTargetMetadata(), defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.Equal(t, ChecksumDeployHit, outcome)
	assert.Equal(t, http.MethodPut, requestMethod)
	assert.True(t, strings.HasPrefix(requestPath, deployPath))
	assert.Contains(t, requestPath, ";build.name=")
	assert.Equal(t, "true", checksumDeployHeader)
	assert.Empty(t, bodyBytes)
	assert.Equal(t, "sha1-value", sha1Header)
}

func TestTargetClient_TryChecksumDeploy_missOn404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	outcome, err := client.TryChecksumDeploy(context.Background(), testTargetMetadata(), defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.Equal(t, ChecksumDeployMiss, outcome)
}

func TestTargetClient_TryChecksumDeploy_conflictDoesNotFallThrough(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"errors":[{"message":"checksum conflict"}]}`))
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	outcome, err := client.TryChecksumDeploy(context.Background(), testTargetMetadata(), defaultTargetDeployOptions())
	require.Error(t, err)
	assert.Equal(t, ChecksumDeployMiss, outcome)
}

func TestTargetClient_TryChecksumDeploy_skipsBuildInfoRepo(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		requestCount++
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	options := defaultTargetDeployOptions()
	options.BuildInfoRepo = true
	outcome, err := client.TryChecksumDeploy(context.Background(), testTargetMetadata(), options)
	require.NoError(t, err)
	assert.Equal(t, ChecksumDeployMiss, outcome)
	assert.Zero(t, requestCount)
}

func TestTargetClient_TryChecksumDeploy_filestoreOptionHeaders(t *testing.T) {
	var binaryExistenceHeader string
	var md5Header string
	var md5Present bool
	var binarySizeHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		binaryExistenceHeader = r.Header.Get("X-Check-Binary-Existence-In-Filestore")
		_, md5Present = r.Header["X-Checksum-Md5"]
		md5Header = r.Header.Get("X-Checksum-Md5")
		binarySizeHeader = r.Header.Get("X-Binary-Size")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	options := defaultTargetDeployOptions()
	options.CheckExistenceInFilestore = true
	outcome, err := client.TryChecksumDeploy(context.Background(), testTargetMetadata(), options)
	require.NoError(t, err)
	assert.Equal(t, ChecksumDeployHit, outcome)
	assert.Equal(t, "true", binaryExistenceHeader)
	assert.True(t, md5Present, "X-Checksum-Md5 must be present on filestore-existence checksum-deploy")
	assert.Equal(t, "md5-value", md5Header)
	assert.Equal(t, strconv.FormatInt(testTargetMetadata().Size, 10), binarySizeHeader)
}

func TestTargetClient_Put_setsContentLengthAndChecksumHeaders(t *testing.T) {
	deployPath := "/" + testTargetRelativePath()
	payload := "hello world"
	var contentLength string
	var sha1Header string
	var sha256Header string
	var md5Present bool
	var bodyBytes []byte
	var requestPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		contentLength = r.Header.Get("Content-Length")
		sha1Header = r.Header.Get("X-Checksum-Sha1")
		sha256Header = r.Header.Get("X-Checksum")
		_, md5Present = r.Header["X-Checksum-Md5"]
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	metadata := testTargetMetadata()
	err = client.Put(context.Background(), metadata, strings.NewReader(payload), defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(requestPath, deployPath))
	assert.Contains(t, requestPath, ";build.name=")
	assert.Equal(t, strconv.FormatInt(metadata.Size, 10), contentLength)
	assert.Equal(t, metadata.Sha1, sha1Header)
	assert.Equal(t, metadata.Sha256, sha256Header)
	assert.False(t, md5Present, "X-Checksum-Md5 must be absent on plain Put")
	assert.Equal(t, payload, string(bodyBytes))
}

func TestTargetClient_ApplyProperties_setsEligibleProperties(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	skippedLargeProps, err := client.ApplyProperties(context.Background(), testTargetMetadata(), defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.False(t, skippedLargeProps)
	assert.Zero(t, requestCount, "short property maps are applied as deploy matrix params, not a follow-up properties PUT")
}

func TestTargetClient_ApplyProperties_omitsValuesLongerThan2400(t *testing.T) {
	longValue := strings.Repeat("x", 2401)
	metadata := testTargetMetadata()
	metadata.Properties = map[string][]string{
		"short": {"ok"},
		"long":  {longValue},
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	skippedLargeProps, err := client.ApplyProperties(context.Background(), metadata, defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.True(t, skippedLargeProps)
	assert.Zero(t, requestCount)
	assert.Contains(t, client.propertyMatrixSuffix(metadata, defaultTargetDeployOptions()), ";short=")
	assert.NotContains(t, client.propertyMatrixSuffix(metadata, defaultTargetDeployOptions()), "long")
}

func TestPropertyMatrixSuffix_omitsPathUnsafeValues(t *testing.T) {
	metadata := testTargetMetadata()
	metadata.Properties = map[string][]string{
		"docker.label.org.opencontainers.image.description": {
			canonicalUbuntuDescription,
		},
		"note":      {"see docs#install"},
		"docker.os": {"linux"},
	}

	suffix := propertyMatrixSuffix(metadata, defaultTargetDeployOptions())
	assert.Empty(t, suffix, "one unsafe value routes the entire item through PATCH")
	assert.NotContains(t, suffix, "docker.label.org.opencontainers.image.description")
	assert.NotContains(t, suffix, "note=")
	assert.NotContains(t, suffix, "%0A")
}

func Test_propertyDelivery_agreesForMatrixAndPatch(t *testing.T) {
	tests := []struct {
		name     string
		props    map[string][]string
		viaPatch bool
	}{
		{
			name:     "path-safe short map uses matrix",
			props:    map[string][]string{"docker.os": {"linux"}},
			viaPatch: false,
		},
		{
			name: "unsafe value forces PATCH",
			props: map[string][]string{
				"note":      {"docs#install"},
				"docker.os": {"linux"},
			},
			viaPatch: true,
		},
		{
			name:     "oversized encoded payload forces PATCH",
			props:    largeEligiblePropertiesMap(t),
			viaPatch: true,
		},
		{
			// ";" and "=" are matrix-param separators, but ToEncodedString escapes them via
			// url.QueryEscape like any other byte, so they round-trip through the matrix path
			// instead of forcing PATCH. See isPathUnsafePropertyValue's doc comment.
			name:     "semicolon and equals values still use matrix",
			props:    map[string][]string{"note": {"a;b=c"}},
			viaPatch: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eligible, _ := filterEligibleProperties(tt.props, TargetDeployOptions{})
			viaPatch, encoded := propertyDelivery(eligible)
			assert.Equal(t, tt.viaPatch, viaPatch)
			suffix := propertyMatrixSuffixFromEligible(eligible)
			if viaPatch {
				assert.Empty(t, suffix)
				assert.NotEmpty(t, encoded)
				return
			}
			assert.Equal(t, ";"+encoded, suffix)
			if tt.name == "semicolon and equals values still use matrix" {
				assert.NotContains(t, encoded, "a;b=c", "raw ; and = must not appear unescaped in the matrix suffix")
				assert.Contains(t, encoded, "a%3Bb%3Dc")
			}
		})
	}
}

func Test_isPathUnsafePropertyValue(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		unsafe bool
	}{
		{name: "newline", value: "first\nsecond", unsafe: true},
		{name: "carriage return", value: "first\rsecond", unsafe: true},
		{name: "hash", value: "docs#install", unsafe: true},
		{name: "comma", value: "one,two", unsafe: true},
		{name: "backslash", value: `one\two`, unsafe: true},
		{name: "pipe", value: "one|two", unsafe: true},
		{name: "semicolon", value: "one;two", unsafe: false},
		{name: "equals", value: "one=two", unsafe: false},
		{name: "safe", value: "letters numbers 123-_./:", unsafe: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.unsafe, isPathUnsafePropertyValue(test.value))
		})
	}
}

func TestTargetClient_ApplyProperties_patchesPathUnsafeValues(t *testing.T) {
	metadata := testTargetMetadata()
	metadata.Properties = map[string][]string{
		"docker.label.org.opencontainers.image.description": {canonicalUbuntuDescription},
		"docker.os": {"linux"},
	}

	var requestMethod string
	var bodyBytes []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMethod = r.Method
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	skippedLargeProps, err := client.ApplyProperties(context.Background(), metadata, defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.False(t, skippedLargeProps)
	assert.Equal(t, http.MethodPatch, requestMethod)

	var patchBody updateItemPropertiesBody
	require.NoError(t, json.Unmarshal(bodyBytes, &patchBody))
	assert.Equal(t, map[string][]string{
		"docker.label.org.opencontainers.image.description": {canonicalUbuntuDescription},
		"docker.os": {"linux"},
	}, patchBody.Props)
	assert.Empty(t, propertyMatrixSuffix(metadata, defaultTargetDeployOptions()))
}

func TestTargetClient_ApplyProperties_patchesPathUnsafeKeys(t *testing.T) {
	metadata := testTargetMetadata()
	metadata.Properties = map[string][]string{
		"safe":       {"value"},
		"unsafe|key": {"preserved"},
	}

	var requestMethod string
	var requestPath string
	var bodyBytes []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMethod = r.Method
		requestPath = r.URL.Path
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	skippedLargeProps, err := client.ApplyProperties(context.Background(), metadata, defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.False(t, skippedLargeProps)
	assert.Equal(t, http.MethodPatch, requestMethod)
	assert.Equal(t, "/api/metadata/"+testTargetRelativePath(), requestPath)
	assert.Empty(t, propertyMatrixSuffix(metadata, defaultTargetDeployOptions()),
		"unsafe property keys must not be encoded as deploy matrix parameters")

	var patchBody updateItemPropertiesBody
	require.NoError(t, json.Unmarshal(bodyBytes, &patchBody))
	assert.Equal(t, metadata.Properties, patchBody.Props)
}

func TestTargetClient_ApplyProperties_usesPatchWhenEncodedPayloadExceeds4000(t *testing.T) {
	metadata := testTargetMetadata()
	metadata.Properties = largeEligiblePropertiesMap(t)

	var requestMethod string
	var requestPath string
	var rawQuery string
	var contentType string
	var bodyBytes []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMethod = r.Method
		requestPath = r.URL.Path
		rawQuery = r.URL.RawQuery
		contentType = r.Header.Get("Content-Type")
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	skippedLargeProps, err := client.ApplyProperties(context.Background(), metadata, defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.False(t, skippedLargeProps)
	assert.Equal(t, http.MethodPatch, requestMethod)
	assert.Equal(t, "/api/metadata/"+testTargetRelativePath(), requestPath)
	assert.Empty(t, rawQuery)
	assert.Contains(t, contentType, "application/json")

	var patchBody updateItemPropertiesBody
	require.NoError(t, json.Unmarshal(bodyBytes, &patchBody))
	assert.NotEmpty(t, patchBody.Props)
	assert.Contains(t, patchBody.Props, "transfer.prop.0000")
}

func TestTargetClient_ApplyProperties_usesPutQueryWhenEncodedPayloadAtMost4000(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	metadata := testTargetMetadata()
	suffix := client.propertyMatrixSuffix(metadata, defaultTargetDeployOptions())
	assert.True(t, strings.HasPrefix(suffix, ";"))
	assert.Contains(t, suffix, "build.name=")
	assert.LessOrEqual(t, len(strings.TrimPrefix(suffix, ";")), maxPropertyEncodedStringLength)

	_, err = client.ApplyProperties(context.Background(), metadata, defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.Zero(t, requestCount)
}

func TestTargetClient_ApplyProperties_stripsOnlyPackageGeneratedPropertyKeys(t *testing.T) {
	metadata := testTargetMetadata()
	metadata.Properties = map[string][]string{
		"build.name":                   {"app"},
		"npm.name":                     {"package"},
		"npm.version":                  {"1.0.0"},
		"artifactory.licenses":         {"MIT"},
		"artifactory.metadata.exclude": {"*"},
		"package.lowercase":            {"true"},
		"ruby":                         {"x86_64-linux"},
		"baseUrl":                      {"http://example.com"},
		"conan.settings.os":            {"Linux"},
		"conan.settings.compiler":      {"gcc"},
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount++
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	options := defaultTargetDeployOptions()
	options.PackageType = "npm"
	suffix := client.propertyMatrixSuffix(metadata, options)
	assert.Contains(t, suffix, "build.name=")
	assert.NotContains(t, suffix, "npm.name")
	assert.NotContains(t, suffix, "npm.version")
	assert.NotContains(t, suffix, "artifactory.metadata.exclude")
	assert.Contains(t, suffix, "artifactory.licenses")
	assert.Contains(t, suffix, "package.lowercase")
	assert.Contains(t, suffix, "baseUrl")
	assert.Contains(t, suffix, "conan.settings.os")

	skippedLargeProps, err := client.ApplyProperties(context.Background(), metadata, options)
	require.NoError(t, err)
	assert.False(t, skippedLargeProps)
	assert.Zero(t, requestCount)
}

func TestTargetClient_ApplyProperties_stripsPackageScopedGeneratedKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	gemsMetadata := testTargetMetadata()
	gemsMetadata.Properties = map[string][]string{
		"build.name": {"app"},
		"ruby":       {"x86_64-linux"},
	}
	gemsOptions := defaultTargetDeployOptions()
	gemsOptions.PackageType = "gems"
	suffix := client.propertyMatrixSuffix(gemsMetadata, gemsOptions)
	assert.Contains(t, suffix, "build.name=")
	assert.NotContains(t, suffix, "ruby=")

	pubMetadata := testTargetMetadata()
	pubMetadata.Properties = map[string][]string{
		"build.name": {"app"},
		"baseUrl":    {"http://example.com"},
	}
	pubOptions := defaultTargetDeployOptions()
	pubOptions.PackageType = "pub"
	suffix = client.propertyMatrixSuffix(pubMetadata, pubOptions)
	assert.Contains(t, suffix, "build.name=")
	assert.NotContains(t, suffix, "baseUrl=")
}

func TestTargetClient_ApplyProperties_preservesMultiValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	metadata := testTargetMetadata()
	metadata.Properties = map[string][]string{
		"env": {"prod", "release"},
	}
	suffix := client.propertyMatrixSuffix(metadata, defaultTargetDeployOptions())
	assert.Contains(t, suffix, "env=prod")
	assert.Contains(t, suffix, "env=release")
}

func TestTargetClient_CreateFolder_putsZeroByteBody(t *testing.T) {
	folderMetadata := &SourceFileMetadata{
		Repo:         testTargetRepo,
		Path:         "empty-dir",
		Name:         "",
		Created:      "2020-01-01T00:00:00.000Z",
		CreatedBy:    "admin",
		LastModified: "2020-01-02T00:00:00.000Z",
		ModifiedBy:   "deployer",
	}
	deployPath := "/" + testTargetRepo + "/empty-dir/"
	var contentLength string
	var bodyBytes []byte
	var requestPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		contentLength = r.Header.Get("Content-Length")
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	err = client.CreateFolder(context.Background(), folderMetadata, defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.Equal(t, deployPath, requestPath)
	assert.Equal(t, "0", contentLength)
	assert.Empty(t, bodyBytes)
}

func TestTargetClient_noPluginExecuteURLs(t *testing.T) {
	var seenPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPaths = append(seenPaths, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	require.NoError(t, client.Ping(context.Background()))
	_, err = client.TryChecksumDeploy(context.Background(), testTargetMetadata(), defaultTargetDeployOptions())
	require.NoError(t, err)

	for _, path := range seenPaths {
		assert.NotContains(t, path, "/api/plugins/execute")
	}
}

func TestTargetClient_checksumDeploy409IsHardFailure(t *testing.T) {
	deployPath := "/" + testTargetRelativePath()
	checksumDeployCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, deployPath) || r.Method != http.MethodPut {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("X-Checksum-Deploy") == "true" {
			checksumDeployCalls++
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"errors":[{"message":"checksum conflict"}]}`))
			return
		}
		t.Errorf("unexpected non-checksum-deploy PUT to %s", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	outcome, err := client.TryChecksumDeploy(context.Background(), testTargetMetadata(), defaultTargetDeployOptions())
	assert.Equal(t, ChecksumDeployMiss, outcome)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permanent target HTTP 409")
	assert.Equal(t, 1, checksumDeployCalls)
}

func largeEligiblePropertiesMap(t *testing.T) map[string][]string {
	t.Helper()
	props := make(map[string][]string)
	for i := 0; ; i++ {
		key := fmt.Sprintf("transfer.prop.%04d", i)
		props[key] = []string{"value"}
		if encodedPropertiesLength(props) > maxPropertyEncodedStringLength {
			break
		}
	}
	require.Greater(t, encodedPropertiesLength(props), maxPropertyEncodedStringLength)
	return props
}

func encodedPropertiesLength(props map[string][]string) int {
	eligibleProps, _ := filterEligibleProperties(props, TargetDeployOptions{})
	return len(eligibleProps.ToEncodedString(false))
}

func TestNewTargetClient_streamServiceManagerHasNoOverallTimeoutOrRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	streamConfig := client.streamServiceManager.GetConfig()
	assert.Zero(t, streamConfig.GetOverallRequestTimeout())
	assert.Zero(t, streamConfig.GetHttpRetries())

	metadataConfig := client.metadataServiceManager.GetConfig()
	assert.Equal(t, time.Minute, metadataConfig.GetOverallRequestTimeout())
	assert.Equal(t, metadataTransferRetries, metadataConfig.GetHttpRetries())
}

func TestTargetClient_TryChecksumDeploy_skipsWhenSha1Empty(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		requestCount++
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	metadata := testTargetMetadata()
	metadata.Sha1 = ""
	outcome, err := client.TryChecksumDeploy(context.Background(), metadata, defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.Equal(t, ChecksumDeployMiss, outcome)
	assert.Zero(t, requestCount)
}

func TestTargetClient_emptySha1UsesPutStreamPath(t *testing.T) {
	deployPath := "/" + testTargetRelativePath()
	payload := "hello world"
	checksumDeployCalls := 0
	fullPutCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, deployPath) || r.Method != http.MethodPut {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("X-Checksum-Deploy") == "true" {
			checksumDeployCalls++
			w.WriteHeader(http.StatusOK)
			return
		}
		fullPutCalls++
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	metadata := testTargetMetadata()
	metadata.Sha1 = ""
	outcome, err := client.TryChecksumDeploy(context.Background(), metadata, defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.Equal(t, ChecksumDeployMiss, outcome)
	assert.Zero(t, checksumDeployCalls)

	err = client.Put(context.Background(), metadata, strings.NewReader(payload), defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.Equal(t, 1, fullPutCalls)
}

func Test_addIdentityHeaders_convertsIsoTimestampsToEpochMilliseconds(t *testing.T) {
	var createdHeader string
	var lastModifiedHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		createdHeader = r.Header.Get("X-Artifactory-Created")
		lastModifiedHeader = r.Header.Get("X-Artifactory-Last-Modified")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	err = client.Put(context.Background(), testTargetMetadata(), strings.NewReader("hello world"), defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.Equal(t, "1577836800000", createdHeader)
	assert.Equal(t, "1577923200000", lastModifiedHeader)
}

func Test_addIdentityHeaders_preservesNumericTimestamps(t *testing.T) {
	var createdHeader string
	var lastModifiedHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		createdHeader = r.Header.Get("X-Artifactory-Created")
		lastModifiedHeader = r.Header.Get("X-Artifactory-Last-Modified")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	metadata := testTargetMetadata()
	metadata.Created = "1670000000000"
	metadata.LastModified = "1680000000000"
	err = client.Put(context.Background(), metadata, strings.NewReader("hello world"), defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.Equal(t, "1670000000000", createdHeader)
	assert.Equal(t, "1680000000000", lastModifiedHeader)
}

func Test_addIdentityHeaders_omitsInvalidTimestamps(t *testing.T) {
	var createdHeader string
	var lastModifiedHeader string
	var createdByHeader string
	var modifiedByHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		createdHeader = r.Header.Get("X-Artifactory-Created")
		lastModifiedHeader = r.Header.Get("X-Artifactory-Last-Modified")
		createdByHeader = r.Header.Get("X-Artifactory-Created-By")
		modifiedByHeader = r.Header.Get("X-Artifactory-Modified-By")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	metadata := testTargetMetadata()
	metadata.Created = "not-a-timestamp"
	metadata.LastModified = "also-invalid"
	err = client.Put(context.Background(), metadata, strings.NewReader("hello world"), defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.Empty(t, createdHeader)
	assert.Empty(t, lastModifiedHeader)
	assert.Equal(t, "admin", createdByHeader)
	assert.Equal(t, "deployer", modifiedByHeader)
}

func TestTargetClient_ApplyStatistics_putsItemStatisticsXML(t *testing.T) {
	var (
		requestMethod string
		requestPath   string
		requestBody   string
		contentType   string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMethod = r.Method
		requestPath = r.URL.Path
		contentType = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		requestBody = string(body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	metadata := testTargetMetadata()
	metadata.DownloadCount = 5
	metadata.LastDownloaded = 1788945284685
	metadata.LastDownloadedBy = "admin"
	err = client.ApplyStatistics(context.Background(), metadata, defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.Equal(t, http.MethodPut, requestMethod)
	assert.Equal(t, "/"+testTargetRelativePath()+":statistics", requestPath)
	assert.Equal(t, "application/xml", contentType)
	assert.Contains(t, requestBody, "<artifactory.stats>")
	assert.Contains(t, requestBody, "<downloadCount>5</downloadCount>")
	assert.Contains(t, requestBody, "<lastDownloaded>1788945284685</lastDownloaded>")
	assert.Contains(t, requestBody, "<lastDownloadedBy>admin</lastDownloadedBy>")
}

func TestTargetClient_ApplyStatistics_rejectedStatus_doesNotFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.HasSuffix(r.URL.Path, ":statistics"))
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"errors":[{"message":"statistics rejected"}]}`))
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	metadata := testTargetMetadata()
	metadata.DownloadCount = 5
	metadata.LastDownloadedBy = "admin"
	err = client.ApplyStatistics(context.Background(), metadata, defaultTargetDeployOptions())
	require.NoError(t, err)
}

func TestTargetClient_ApplyStatistics_skipsWhenEmpty(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		requestCount++
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	err = client.ApplyStatistics(context.Background(), testTargetMetadata(), defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.Zero(t, requestCount)
}

func TestTargetClient_ApplyStatistics_skipsFolders(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		requestCount++
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	err = client.ApplyStatistics(context.Background(), &SourceFileMetadata{
		Repo:             testTargetRepo,
		Path:             "empty-dir",
		DownloadCount:    3,
		LastDownloadedBy: "admin",
	}, defaultTargetDeployOptions())
	require.NoError(t, err)
	assert.Zero(t, requestCount)
}

func TestTargetClient_sendsTargetCredentials(t *testing.T) {
	var receivedAuth []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = append(receivedAuth, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	details := newTestTargetServerDetails(server.URL)
	details.AccessToken = "target-secret-token"
	client, err := NewTargetClient(context.Background(), details, nil)
	require.NoError(t, err)

	require.NoError(t, client.Ping(context.Background()))
	assert.NotEmpty(t, receivedAuth)
	for i, auth := range receivedAuth {
		assert.Equal(t, "Bearer target-secret-token", auth, "request %d", i)
	}
}

// TestTargetClient_Put_expiredTokenRejection_returnsPromptlyNoHang exercises the real HTTP path
// (doManagerPutStream's direct manager.Client().GetHttpClient().GetClient().Do(req) call, which
// bypasses client-go's JfrogHttpClient.Send* wrapper and therefore its token-refresh
// pre-request interceptor) to settle the B-22 "expiring token mid-stream deadlocks the copy/PUT
// goroutines" theory. The target here simulates an Artifactory rejecting a stale, un-refreshed
// bearer token: it reads none of the request body and immediately responds 401. The reader fed
// to Put blocks forever on all but its first Read (as a real in-flight GET-stream reader would
// while more file content is still arriving), so if the PUT path ever waited for the full body to
// be written before observing the response, this test would hang until its own timeout.
func TestTargetClient_Put_expiredTokenRejection_returnsPromptlyNoHang(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Deliberately do not read r.Body: an expired/rejected token fails auth before content
		// processing. Respond immediately, as a real Artifactory 401 would.
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errors":[{"status":401,"message":"Bad credentials"}]}`))
	}))
	defer server.Close()

	details := newTestTargetServerDetails(server.URL)
	details.AccessToken = "stale-expired-token"
	client, err := NewTargetClient(context.Background(), details, nil)
	require.NoError(t, err)

	blockingReader := newBlockingAfterFirstByteReader()
	metadata := testTargetMetadata()
	metadata.Size = 10 * 1024 * 1024 // large enough that a body-first client would never finish writing

	done := make(chan error, 1)
	go func() {
		done <- client.Put(context.Background(), metadata, blockingReader, defaultTargetDeployOptions())
	}()

	select {
	case err := <-done:
		require.Error(t, err, "expired-token PUT must fail cleanly, not succeed")
		assert.Contains(t, err.Error(), "401")
	case <-time.After(3 * time.Second):
		t.Fatal("TargetClient.Put hung instead of returning promptly on a 401 rejection")
	}
}

// TestTargetClient_Put_runsPreRequestInterceptors_tokenRefresh verifies the B-22 fix: Put's
// streaming PUT path (doManagerPutStream) now runs the target's registered pre-request
// interceptors (the same mechanism client-go's JfrogHttpClient.Send* uses for proactive
// access/refresh-token renewal) before issuing the raw HTTP request, instead of bypassing them
// entirely. A custom interceptor rewrites the Authorization header; the test asserts the target
// server actually observes the rewritten header, not the stale one the client was constructed
// with.
func TestTargetClient_Put_runsPreRequestInterceptors_tokenRefresh(t *testing.T) {
	var observedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	details := newTestTargetServerDetails(server.URL)
	details.AccessToken = "stale-expired-token"
	client, err := NewTargetClient(context.Background(), details, nil)
	require.NoError(t, err)

	serviceDetails := client.streamServiceManager.GetConfig().GetServiceDetails()
	serviceDetails.AppendPreRequestFunction(func(_ *auth.CommonConfigFields, httpClientDetails *httputils.HttpClientDetails) error {
		if httpClientDetails.Headers == nil {
			httpClientDetails.Headers = map[string]string{}
		}
		httpClientDetails.Headers["Authorization"] = "Bearer refreshed-token"
		return nil
	})

	metadata := testTargetMetadata()
	metadata.Size = int64(len("hello world"))
	err = client.Put(context.Background(), metadata, bytes.NewReader([]byte("hello world")), defaultTargetDeployOptions())
	require.NoError(t, err)

	assert.Equal(t, "Bearer refreshed-token", observedAuth, "the pre-request interceptor must run before the streaming PUT is sent")
}

// blockingAfterFirstByteReader yields one byte, then blocks on every subsequent Read until the
// test process exits, modeling a GET-stream reader that still has much more content pending.
type blockingAfterFirstByteReader struct {
	yielded bool
}

func newBlockingAfterFirstByteReader() *blockingAfterFirstByteReader {
	return &blockingAfterFirstByteReader{}
}

func (r *blockingAfterFirstByteReader) Read(p []byte) (int, error) {
	if !r.yielded && len(p) > 0 {
		r.yielded = true
		p[0] = 'x'
		return 1, nil
	}
	select {} // block forever; only a closed underlying connection/context should ever stop the caller
}

// TestTargetClient_sendsTargetCredentials_putAndPatch complements
// TestTargetClient_sendsTargetCredentials, which only exercises Ping (routed through
// doManagerGet). Put and applyPropertiesViaPatch instead build their requests through
// applyHttpClientDetails directly, a separate code path that must apply the same auth.
func TestTargetClient_sendsTargetCredentials_putAndPatch(t *testing.T) {
	receivedAuthByMethod := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthByMethod[r.Method] = r.Header.Get("Authorization")
		if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	details := newTestTargetServerDetails(server.URL)
	details.AccessToken = "target-secret-token"
	client, err := NewTargetClient(context.Background(), details, nil)
	require.NoError(t, err)

	require.NoError(t, client.Put(context.Background(), testTargetMetadata(), strings.NewReader("hello world"), defaultTargetDeployOptions()))

	metadata := testTargetMetadata()
	metadata.Properties = largeEligiblePropertiesMap(t)
	_, err = client.ApplyProperties(context.Background(), metadata, defaultTargetDeployOptions())
	require.NoError(t, err)

	assert.Equal(t, "Bearer target-secret-token", receivedAuthByMethod[http.MethodPut])
	assert.Equal(t, "Bearer target-secret-token", receivedAuthByMethod[http.MethodPatch])
}

func TestTargetClient_TryChecksumDeploy_permanent4xxFailsFast(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"message":"forbidden"}]}`))
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	outcome, err := client.TryChecksumDeploy(context.Background(), testTargetMetadata(), defaultTargetDeployOptions())
	assert.Equal(t, ChecksumDeployMiss, outcome)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permanent target HTTP 403")
	assert.Equal(t, 1, requestCount, "4xx other than 429 must not be retried")
}

func TestTargetClient_TryChecksumDeploy_retryableStatusUsesMetadataRetryPolicy(t *testing.T) {
	const metadataRetries = 2
	for _, testCase := range []struct {
		name       string
		statusCode int
	}{
		{name: "too many requests", statusCode: http.StatusTooManyRequests},
		{name: "server error", statusCode: http.StatusServiceUnavailable},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requestCount++
				w.WriteHeader(testCase.statusCode)
			}))
			defer server.Close()

			details := newTestTargetServerDetails(server.URL)
			client, err := NewTargetClient(context.Background(), details, nil)
			require.NoError(t, err)
			client.metadataServiceManager, err = transferutils.CreateServiceManagerWithContext(
				context.Background(), details, false, 0, metadataRetries, 0, time.Minute,
			)
			require.NoError(t, err)

			outcome, err := client.TryChecksumDeploy(
				context.Background(), testTargetMetadata(), defaultTargetDeployOptions(),
			)
			assert.Equal(t, ChecksumDeployMiss, outcome)
			require.Error(t, err)
			assert.Equal(t, metadataRetries, client.metadataServiceManager.GetConfig().GetHttpRetries())
			assert.Equal(t, metadataRetries+1, requestCount,
				"retryable responses must use the configured metadata retries plus the initial attempt")
		})
	}
}

// TestNewTargetClient_metadataServiceManagerUsesProductionRetryPolicy complements
// TestTargetClient_TryChecksumDeploy_retryableStatusUsesMetadataRetryPolicy, which swaps in a
// manager with a small retry count to keep the test fast. This asserts the manager NewTargetClient
// actually builds (without any override) carries the dedicated metadata retry budget.
func TestNewTargetClient_metadataServiceManagerUsesProductionRetryPolicy(t *testing.T) {
	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails("http://127.0.0.1:0"), nil)
	require.NoError(t, err)
	assert.Equal(t, metadataTransferRetries, client.metadataServiceManager.GetConfig().GetHttpRetries())
	assert.Equal(t, metadataRetryWaitMilliSecs, client.metadataServiceManager.GetConfig().GetHttpRetryWaitMilliSecs())
}

func TestTargetClient_eligiblePropertiesComputedOnceAcrossFileAndFolderOperations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Header.Get("X-Checksum-Deploy") == "true":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)
	var filterCalls atomic.Int32
	client.filterEligibleProperties = func(
		properties map[string][]string, options TargetDeployOptions,
	) (*artifactoryutils.Properties, bool) {
		filterCalls.Add(1)
		return filterEligibleProperties(properties, options)
	}

	fileMetadata := testTargetMetadata()
	fileMetadata.Properties = largeEligiblePropertiesMap(t)
	options := defaultTargetDeployOptions()
	outcome, err := client.TryChecksumDeploy(context.Background(), fileMetadata, options)
	require.NoError(t, err)
	assert.Equal(t, ChecksumDeployMiss, outcome)
	require.NoError(t, client.Put(context.Background(), fileMetadata, strings.NewReader("hello world"), options))
	_, err = client.ApplyProperties(context.Background(), fileMetadata, options)
	require.NoError(t, err)

	folderMetadata := &SourceFileMetadata{
		Repo:       testTargetRepo,
		Path:       "empty-dir",
		Properties: largeEligiblePropertiesMap(t),
	}
	require.NoError(t, client.CreateFolder(context.Background(), folderMetadata, options))
	_, err = client.ApplyProperties(context.Background(), folderMetadata, options)
	require.NoError(t, err)

	assert.Equal(t, int32(2), filterCalls.Load(),
		"each file or folder transfer must compute eligible properties once across all target operations")
}

func TestTargetClient_concurrentTransfersDoNotEvictEligibleProperties(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)
	var filterCalls atomic.Int32
	client.filterEligibleProperties = func(
		properties map[string][]string, options TargetDeployOptions,
	) (*artifactoryutils.Properties, bool) {
		filterCalls.Add(1)
		return filterEligibleProperties(properties, options)
	}

	const transfers = 8
	metadata := make([]*SourceFileMetadata, transfers)
	for i := range metadata {
		metadata[i] = &SourceFileMetadata{
			Repo:       testTargetRepo,
			Path:       fmt.Sprintf("folder-%d", i),
			Properties: largeEligiblePropertiesMap(t),
		}
	}
	options := defaultTargetDeployOptions()
	runConcurrently := func(operation func(*SourceFileMetadata) error) {
		var waitGroup sync.WaitGroup
		errors := make(chan error, len(metadata))
		for _, item := range metadata {
			waitGroup.Add(1)
			go func() {
				defer waitGroup.Done()
				errors <- operation(item)
			}()
		}
		waitGroup.Wait()
		close(errors)
		for operationErr := range errors {
			require.NoError(t, operationErr)
		}
	}

	runConcurrently(func(item *SourceFileMetadata) error {
		return client.CreateFolder(context.Background(), item, options)
	})
	runConcurrently(func(item *SourceFileMetadata) error {
		_, applyErr := client.ApplyProperties(context.Background(), item, options)
		return applyErr
	})

	assert.Equal(t, int32(transfers), filterCalls.Load(),
		"concurrent transfers must retain each transfer's cached eligible properties")
}

func TestTargetClient_Put_errorReleasesEligibleProperties(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"message":"forbidden"}]}`))
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	metadata := testTargetMetadata()
	options := defaultTargetDeployOptions()
	err = client.Put(context.Background(), metadata, strings.NewReader("hello world"), options)
	require.Error(t, err)
	assert.False(t, eligibleCacheHas(client, metadata, options),
		"failed Put must release eligibleCache without ApplyProperties")
}

func TestTargetClient_Put_classifiesPermanentStatusAndKeepsBodyText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"message":"forbidden by target"}]}`))
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	metadata := testTargetMetadata()
	err = client.Put(context.Background(), metadata, strings.NewReader("hello world"), defaultTargetDeployOptions())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permanent target HTTP 403",
		"plain PUT must reach the permanent/retryable classification, not bail out on UploadFileFromReader's status-only error")
	assert.Contains(t, err.Error(), "forbidden by target",
		"the classified error must still carry the response body text")
}

func TestTargetClient_Put_classifiesRetryableStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"errors":[{"message":"slow down"}]}`))
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	metadata := testTargetMetadata()
	err = client.Put(context.Background(), metadata, strings.NewReader("hello world"), defaultTargetDeployOptions())
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "permanent target HTTP",
		"429 must remain retryable, not be classified as permanent")
	assert.Contains(t, err.Error(), "slow down")
}

func TestTargetClient_Put_ctxCancelAbortsInFlightRequest(t *testing.T) {
	unblock := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-unblock
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	defer close(unblock)

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err = client.Put(ctx, testTargetMetadata(), strings.NewReader("hello world"), defaultTargetDeployOptions())
	elapsed := time.Since(start)
	require.Error(t, err)
	assert.Less(t, elapsed, 2*time.Second,
		"Put must be bound to the per-call ctx and abort mid-flight on cancellation, not block until the (never-responding) server replies")
}

// TestTargetClient_Put_ctxCancelAbortsMidBodyWrite complements
// TestTargetClient_Put_ctxCancelAbortsInFlightRequest, which cancels while still waiting for
// response headers (before any body byte reaches the server). This exercises cancellation once
// the target is already reading the streamed body. The reader is a real network response body
// (as production streams: the source GET's resp.Body feeds the target PUT), not an io.Pipe --
// an io.Pipe's blocking Read isn't network I/O and isn't unblocked by the transport closing the
// connection on ctx cancellation, which would make that variant of this test hang forever.
func TestTargetClient_Put_ctxCancelAbortsMidBodyWrite(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("a"), 4096))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer sourceServer.Close()

	targetRequestReceived := make(chan struct{})
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(targetRequestReceived)
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer targetServer.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(targetServer.URL), nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	sourceReq, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceServer.URL, nil)
	require.NoError(t, err)
	sourceResp, err := http.DefaultClient.Do(sourceReq)
	require.NoError(t, err)
	defer func() { _ = sourceResp.Body.Close() }()

	go func() {
		<-targetRequestReceived
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	metadata := testTargetMetadata()
	metadata.Size = 1024 * 1024 // larger than what the source ever supplies, so the body never completes on its own

	start := time.Now()
	err = client.Put(ctx, metadata, sourceResp.Body, defaultTargetDeployOptions())
	elapsed := time.Since(start)
	require.Error(t, err)
	assert.Less(t, elapsed, 2*time.Second,
		"Put must abort a stalled mid-body write on ctx cancellation, not block forever on the source reader")
}

func TestTargetClient_ApplyProperties_ctxCancelAbortsInFlightPatch(t *testing.T) {
	unblock := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-unblock
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	defer close(unblock)

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	metadata := testTargetMetadata()
	metadata.Properties = largeEligiblePropertiesMap(t)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err = client.ApplyProperties(ctx, metadata, defaultTargetDeployOptions())
	elapsed := time.Since(start)
	require.Error(t, err)
	assert.Less(t, elapsed, 2*time.Second,
		"ApplyProperties' PATCH must be bound to the per-call ctx and abort mid-flight on cancellation")
}

func TestTargetClient_CreateFolder_errorReleasesEligibleProperties(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"message":"forbidden"}]}`))
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	metadata := &SourceFileMetadata{
		Repo:       testTargetRepo,
		Path:       "empty-dir",
		Properties: map[string][]string{"build.name": {"app"}},
	}
	options := defaultTargetDeployOptions()
	err = client.CreateFolder(context.Background(), metadata, options)
	require.Error(t, err)
	assert.False(t, eligibleCacheHas(client, metadata, options),
		"failed CreateFolder must release eligibleCache without ApplyProperties")
}

func eligibleCacheHas(tc *TargetClient, metadata *SourceFileMetadata, options TargetDeployOptions) bool {
	tc.eligibleMu.Lock()
	defer tc.eligibleMu.Unlock()
	_, ok := tc.eligibleCache[eligiblePropertiesCacheKey{metadata: metadata, packageType: options.PackageType}]
	return ok
}

func Test_filterEligibleProperties_usesPackageType(t *testing.T) {
	properties := map[string][]string{
		"build.name":  {"app"},
		"npm.name":    {"pkg"},
		"npm.version": {"1.0.0"},
	}
	options := TargetDeployOptions{PackageType: "npm"}
	eligibleProps, skippedLargeProps := filterEligibleProperties(properties, options)
	assert.False(t, skippedLargeProps)
	assert.Equal(t, 1, eligibleProps.KeysLen())
	assert.Equal(t, map[string][]string{"build.name": {"app"}}, eligibleProps.ToMap())
}

func testStatsMetadata() *SourceFileMetadata {
	metadata := testTargetMetadata()
	metadata.DownloadCount = 5
	metadata.LastDownloaded = 1788945284685
	metadata.LastDownloadedBy = "admin"
	return metadata
}

func Test_skipsDownloadStatistics(t *testing.T) {
	tests := []struct {
		name    string
		options TargetDeployOptions
		want    bool
	}{
		{name: "generic", options: defaultTargetDeployOptions(), want: false},
		{name: "distribution", options: TargetDeployOptions{PackageType: "distribution"}, want: false},
		{name: "support", options: TargetDeployOptions{PackageType: "support"}, want: false},
		{name: "releasebundles", options: TargetDeployOptions{PackageType: "releasebundles"}, want: false},
		{name: "buildInfoRepoFlag", options: TargetDeployOptions{BuildInfoRepo: true}, want: true},
		{name: "buildinfoPackageType", options: TargetDeployOptions{PackageType: "BuildInfo"}, want: true},
		{name: "pipeinfoPackageType", options: TargetDeployOptions{PackageType: "PipeInfo"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, skipsDownloadStatistics(tt.options))
		})
	}
}

// slowReader trickles data out with a delay between chunks, simulating a large upload over a
// slow network: the total time to write the whole body can vastly exceed streamResponseHeaderTimeout.
type slowReader struct {
	data      []byte
	pos       int
	chunkSize int
	delay     time.Duration
}

func (r *slowReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	time.Sleep(r.delay)
	n := r.chunkSize
	if remaining := len(r.data) - r.pos; n > remaining {
		n = remaining
	}
	n = copy(p, r.data[r.pos:r.pos+n])
	r.pos += n
	return n, nil
}

// TestTargetClient_Put_slowLargeUpload_notBoundedByResponseHeaderTimeout answers a direct
// question about the B-17(b) fix: does bounding ResponseHeaderTimeout break large, slow uploads?
// Per net/http's own doc for Transport.ResponseHeaderTimeout, the timer only starts "after fully
// writing the request (including its body, if any)" - so a slow but steadily-progressing upload,
// whose total wall-clock time is many multiples of streamResponseHeaderTimeout, must still
// succeed as long as the target responds promptly once the body is fully received. Only a peer
// that stalls AFTER receiving the full body (or before starting to respond at all) should trip
// the timeout - that scenario is covered separately by
// TestTargetClient_Put_nonRespondingPeer_boundedByResponseHeaderTimeout.
func TestTargetClient_Put_slowLargeUpload_notBoundedByResponseHeaderTimeout(t *testing.T) {
	origTimeout := streamResponseHeaderTimeout
	streamResponseHeaderTimeout = 150 * time.Millisecond
	defer func() { streamResponseHeaderTimeout = origTimeout }()

	deployPath := "/" + testTargetRelativePath()
	var receivedLen int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, deployPath) {
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		receivedLen = len(body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	// 20 chunks x 40ms = ~800ms total upload time - more than 5x streamResponseHeaderTimeout
	// (150ms), yet the upload never stalls, so it must succeed.
	payload := bytes.Repeat([]byte("x"), 200)
	reader := &slowReader{data: payload, chunkSize: 10, delay: 40 * time.Millisecond}

	start := time.Now()
	metadata := testTargetMetadata()
	metadata.Properties = nil // avoid a matrix-param suffix on the deploy path
	metadata.Size = int64(len(payload))
	err = client.Put(context.Background(), metadata, reader, defaultTargetDeployOptions())
	elapsed := time.Since(start)

	require.NoError(t, err, "a slow-but-steady large upload must not be cut short by streamResponseHeaderTimeout")
	assert.Equal(t, len(payload), receivedLen)
	assert.Greater(t, elapsed, 5*streamResponseHeaderTimeout, "sanity check: the upload should genuinely have taken longer than the response-header timeout")
}

// TestTargetClient_Put_nonRespondingPeer_boundedByResponseHeaderTimeout is the target-side
// counterpart of B-17(b) (see sourceclient_test.go's
// TestGetFileReader_nonRespondingPeer_boundedByResponseHeaderTimeout): without --proxy-key,
// NewTargetClient used to fall through to client-go's default transport for the streaming PUT
// path, which has no ResponseHeaderTimeout. A target peer that accepts the connection but never
// writes a response would hang the PUT indefinitely.
func TestTargetClient_Put_nonRespondingPeer_boundedByResponseHeaderTimeout(t *testing.T) {
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

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails("http://"+ln.Addr().String()), nil)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		done <- client.Put(context.Background(), testTargetMetadata(), strings.NewReader("hello world"), defaultTargetDeployOptions())
	}()

	select {
	case err := <-done:
		assert.Error(t, err, "a non-responding target peer must fail once the response-header timeout elapses, not hang or succeed")
	case <-time.After(5 * time.Second):
		t.Fatal("Put hung indefinitely against a non-responding target peer instead of being bounded by streamResponseHeaderTimeout")
	}
}

func TestTargetClient_ApplyStatistics_skipsInfoRepos(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		requestCount++
	}))
	defer server.Close()

	client, err := NewTargetClient(context.Background(), newTestTargetServerDetails(server.URL), nil)
	require.NoError(t, err)

	for _, options := range []TargetDeployOptions{
		{BuildInfoRepo: true},
		{PackageType: "buildinfo"},
		{PackageType: "pipeinfo"},
	} {
		requestCount = 0
		err = client.ApplyStatistics(context.Background(), testStatsMetadata(), options)
		require.NoError(t, err)
		assert.Zero(t, requestCount, "options=%+v", options)
	}
}
