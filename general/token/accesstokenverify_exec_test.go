package token_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/common/commands"
	"github.com/jfrog/jfrog-cli-core/v2/general/token"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerify_Exec_UsageReportingNeverRefreshes(t *testing.T) {
	t.Setenv(coreutils.HomeDir, t.TempDir())
	t.Setenv(coreutils.ReportUsage, "true")
	certB64, key := token.GenerateTestCert(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/access/api/v1/cert/root":
			_, _ = w.Write([]byte(certB64))
		case "/access/api/v1/system/readiness":
		case "/artifactory/api/system/version":
			_, _ = w.Write([]byte(`{"version":"6.0.0"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	}))
	defer server.Close()

	now := time.Now()
	nearExpiry := func() string {
		claims := token.ValidClaims(now)
		claims["iat"] = now.Add(-5 * time.Minute).Unix()
		claims["exp"] = now.Add(5 * time.Minute).Unix()
		return token.SignToken(t, key, token.ValidHeader(), claims)
	}
	profileToken := nearExpiry()
	require.NoError(t, config.SaveServersConf([]*config.ServerDetails{{
		ServerId:       "login",
		Url:            server.URL + "/",
		ArtifactoryUrl: server.URL + "/artifactory/",
		AccessToken:    profileToken,
		RefreshToken:   "refresh",
		WebLogin:       true,
		IsDefault:      true,
	}}))

	for name, tokenUnderTest := range map[string]string{"another token": nearExpiry(), "profile's own token": profileToken} {
		t.Run(name, func(t *testing.T) {
			profile, err := config.GetSpecificConfig("login", true, true)
			require.NoError(t, err)

			start := time.Now()
			require.NoError(t, commands.Exec(token.NewAccessTokenVerifyCommand().SetServerDetails(profile).SetToken(tokenUnderTest)))
			assert.Less(t, time.Since(start), 5*time.Second)

			saved, err := config.GetSpecificConfig("login", true, false)
			require.NoError(t, err)
			assert.Equal(t, profileToken, saved.AccessToken)
			assert.Equal(t, "refresh", saved.RefreshToken)
		})
	}
}
