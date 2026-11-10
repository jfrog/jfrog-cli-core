package token

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testNow = time.Unix(1_800_000_000, 0)

func generateTestCert(t *testing.T) (string, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(certDER), key
}

func signToken(t *testing.T, key *rsa.PrivateKey, header, claims map[string]interface{}) string {
	t.Helper()
	encode := func(v interface{}) string {
		data, err := json.Marshal(v)
		require.NoError(t, err)
		return base64.RawURLEncoding.EncodeToString(data)
	}
	signingInput := encode(header) + "." + encode(claims)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func validHeader() map[string]interface{} {
	return map[string]interface{}{"alg": "RS256", "typ": "JWT", "ver": "2", "kid": "test-kid"}
}

func validClaims(now time.Time) map[string]interface{} {
	return map[string]interface{}{
		"jti": "test-jti",
		"sub": testUserSub,
		"aud": "*@*",
		"scp": "applied-permissions/user",
		"iss": "jfac@01ftqhpvg3kn760kg8qbt411qy",
		"iat": now.Add(-time.Minute).Unix(),
		"exp": now.Add(time.Hour).Unix(),
		"ext": `{"revocable":"true"}`,
	}
}

type fakeAccessClient struct {
	cert           string
	rootErr        error
	status         int
	readinessErr   error
	rootCalls      int
	readinessCalls int
}

func (f *fakeAccessClient) GetRootCertificate() (string, error) {
	f.rootCalls++
	return f.cert, f.rootErr
}

func (f *fakeAccessClient) CheckReadiness() (int, error) {
	f.readinessCalls++
	return f.status, f.readinessErr
}

func newTestCommand(token string, fake *fakeAccessClient) *AccessTokenVerifyCommand {
	cmd := NewAccessTokenVerifyCommand().SetServerDetails(&config.ServerDetails{Url: "https://jpd.example/"}).SetToken(token)
	cmd.now = func() time.Time { return testNow }
	cmd.newAccessClient = func(*config.ServerDetails) (accessClient, error) { return fake, nil }
	return cmd
}

func validSetup(t *testing.T) (*rsa.PrivateKey, *fakeAccessClient) {
	certB64, key := generateTestCert(t)
	return key, &fakeAccessClient{cert: certB64, status: http.StatusOK}
}

func TestVerify_ValidIdentityToken(t *testing.T) {
	key, fake := validSetup(t)
	cmd := newTestCommand(signToken(t, key, validHeader(), validClaims(testNow)), fake)
	require.NoError(t, cmd.Run())
	assert.True(t, cmd.Result().Valid)
	assert.Equal(t, []TokenType{TokenTypeIdentity}, cmd.Result().Types)
	assert.Equal(t, 1, fake.readinessCalls)
}

func TestVerify_InvalidHeader_NoNetworkCalls(t *testing.T) {
	key, fake := validSetup(t)
	header := validHeader()
	header["ver"] = "1"
	cmd := newTestCommand(signToken(t, key, header, validClaims(testNow)), fake)
	assert.ErrorContains(t, cmd.Run(), `expected ver "2"`)
	assert.Zero(t, fake.rootCalls+fake.readinessCalls)
	assert.NotNil(t, cmd.Result().Payload)
}

func TestVerify_NoConfiguredServer(t *testing.T) {
	key, fake := validSetup(t)
	cmd := newTestCommand(signToken(t, key, validHeader(), validClaims(testNow)), fake)
	cmd.SetServerDetails(&config.ServerDetails{})
	assert.ErrorContains(t, cmd.Run(), "no JFrog server")
	assert.NotNil(t, cmd.Result().Payload)
	assert.Zero(t, fake.rootCalls)
}

func TestVerify_BadSignature_TokenNotSentToServer(t *testing.T) {
	_, fake := validSetup(t)
	_, otherKey := generateTestCert(t)
	cmd := newTestCommand(signToken(t, otherKey, validHeader(), validClaims(testNow)), fake)
	assert.ErrorContains(t, cmd.Run(), "invalid signature")
	assert.Zero(t, fake.readinessCalls)
}

func TestVerify_MissingSub_BeforeReadiness(t *testing.T) {
	key, fake := validSetup(t)
	claims := validClaims(testNow)
	delete(claims, "sub")
	cmd := newTestCommand(signToken(t, key, validHeader(), claims), fake)
	assert.ErrorContains(t, cmd.Run(), "missing required field: sub")
	assert.Zero(t, fake.readinessCalls)
}

func TestVerify_Expired_BeforeReadiness(t *testing.T) {
	key, fake := validSetup(t)
	cmd := newTestCommand(signToken(t, key, validHeader(), validClaims(testNow.Add(-2*time.Hour))), fake)
	assert.ErrorContains(t, cmd.Run(), "token expired at")
	assert.Zero(t, fake.readinessCalls)
	assert.Nil(t, cmd.Result().Types)
}

func TestVerify_NoExpiry_Valid(t *testing.T) {
	key, fake := validSetup(t)
	claims := validClaims(testNow)
	delete(claims, "exp")
	assert.NoError(t, newTestCommand(signToken(t, key, validHeader(), claims), fake).Run())
}

func TestVerify_RootCertFetchFails(t *testing.T) {
	key, fake := validSetup(t)
	fake.rootErr = errors.New("boom")
	cmd := newTestCommand(signToken(t, key, validHeader(), validClaims(testNow)), fake)
	assert.ErrorContains(t, cmd.Run(), "GET /access/api/v1/cert/root")
}

func TestVerify_ReadinessRejected(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		key, fake := validSetup(t)
		fake.status = status
		cmd := newTestCommand(signToken(t, key, validHeader(), validClaims(testNow)), fake)
		assert.EqualError(t, cmd.Run(), fmt.Sprintf("token rejected by Access server: status %d from GET /access/api/v1/system/readiness", status))
		assert.False(t, cmd.Result().Valid)
	}
}

func TestVerify_ReadinessUnexpectedStatus(t *testing.T) {
	key, fake := validSetup(t)
	fake.status = http.StatusServiceUnavailable
	cmd := newTestCommand(signToken(t, key, validHeader(), validClaims(testNow)), fake)
	assert.ErrorContains(t, cmd.Run(), "unexpected response from Access server: status 503")
}

func TestVerify_ReadinessTransportError(t *testing.T) {
	key, fake := validSetup(t)
	fake.readinessErr = errors.New("connection refused")
	cmd := newTestCommand(signToken(t, key, validHeader(), validClaims(testNow)), fake)
	assert.ErrorContains(t, cmd.Run(), "failed to reach Access server")
}

func TestVerify_ReferenceToken(t *testing.T) {
	_, fake := validSetup(t)
	cmd := newTestCommand("cmVmdGtuOjAxOjA6YWJj", fake)
	assert.ErrorContains(t, cmd.Run(), "reference token")
	assert.Zero(t, fake.rootCalls)
}

func TestVerify_ApiKey(t *testing.T) {
	_, fake := validSetup(t)
	cmd := newTestCommand("AKCp8"+strings.Repeat("x", 68), fake)
	assert.ErrorContains(t, cmd.Run(), "API key")
	assert.Zero(t, fake.rootCalls)
}

func TestVerify_RefreshableProfile_NeverRefreshes(t *testing.T) {
	t.Setenv(coreutils.HomeDir, t.TempDir())
	certB64, key := generateTestCert(t)
	var readinessAuth []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/access/api/v1/cert/root":
			_, _ = w.Write([]byte(certB64))
		case "/access/api/v1/system/readiness":
			readinessAuth = append(readinessAuth, r.Header.Get("Authorization"))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	}))
	defer server.Close()

	now := time.Now()
	nearExpiry := func(lifetime, left time.Duration) string {
		claims := validClaims(now)
		claims["iat"] = now.Add(left - lifetime).Unix()
		claims["exp"] = now.Add(left).Unix()
		return signToken(t, key, validHeader(), claims)
	}
	noExpiry := validClaims(now)
	delete(noExpiry, "exp")
	profileToken := nearExpiry(10*time.Minute, 5*time.Minute)
	require.NoError(t, config.SaveServersConf([]*config.ServerDetails{{
		ServerId:     "login",
		Url:          server.URL + "/",
		AccessToken:  profileToken,
		RefreshToken: "refresh",
		WebLogin:     true,
		IsDefault:    true,
	}}))

	for name, token := range map[string]string{
		"10m lifetime, 5m left": nearExpiry(10*time.Minute, 5*time.Minute),
		"1h lifetime, 20m left": nearExpiry(time.Hour, 20*time.Minute),
		"no exp":                signToken(t, key, validHeader(), noExpiry),
		"profile's own token":   profileToken,
	} {
		t.Run(name, func(t *testing.T) {
			readinessAuth = nil
			profile, err := config.GetSpecificConfig("login", true, true)
			require.NoError(t, err)

			start := time.Now()
			require.NoError(t, NewAccessTokenVerifyCommand().SetServerDetails(profile).SetToken(token).Run())
			assert.Less(t, time.Since(start), 5*time.Second)
			assert.Equal(t, []string{"Bearer " + token}, readinessAuth)

			saved, err := config.GetSpecificConfig("login", true, false)
			require.NoError(t, err)
			assert.Equal(t, profileToken, saved.AccessToken)
			assert.Equal(t, "refresh", saved.RefreshToken)
		})
	}
}

func TestTokenUnderTestDetails_KeepsOnlyUrlAndClientCert(t *testing.T) {
	profile := &config.ServerDetails{
		ServerId:                "p",
		Url:                     "https://jpd.example/",
		User:                    "u",
		Password:                "p",
		AccessToken:             "profile-token",
		RefreshToken:            "r",
		ArtifactoryRefreshToken: "ar",
		SshKeyPath:              "k",
		SshPassphrase:           "s",
		ClientCertPath:          "c",
		ClientCertKeyPath:       "ck",
		InsecureTls:             true,
		WebLogin:                true,
	}
	assert.Equal(t, &config.ServerDetails{
		Url:                 "https://jpd.example/",
		ClientCertPath:      "c",
		ClientCertKeyPath:   "ck",
		AccessToken:         "token",
		DisableTokenRefresh: true,
	}, tokenUnderTestDetails(profile, "token"))
}

func TestInsecureConnectionWarning(t *testing.T) {
	assert.Empty(t, insecureConnectionWarning(&config.ServerDetails{Url: "https://x/"}))
	warning := insecureConnectionWarning(&config.ServerDetails{Url: "http://x/"})
	assert.Contains(t, warning, "http://")
	assert.Contains(t, warning, "can't prove the root certificate")
}

func TestVerify_ServerDetailsIsProfile(t *testing.T) {
	key, fake := validSetup(t)
	cmd := newTestCommand(signToken(t, key, validHeader(), validClaims(testNow)), fake)
	profile := &config.ServerDetails{Url: "https://jpd.example/", AccessToken: "profile-token"}
	cmd.SetServerDetails(profile)
	require.NoError(t, cmd.Run())
	details, err := cmd.ServerDetails()
	require.NoError(t, err)
	assert.Same(t, profile, details)
	assert.Equal(t, "profile-token", profile.AccessToken)
}
