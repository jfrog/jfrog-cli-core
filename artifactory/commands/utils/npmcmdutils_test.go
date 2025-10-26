package utils

import (
	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestGetRegistry(t *testing.T) {
	var getRegistryTest = []struct {
		repo     string
		url      string
		expected string
	}{
		// jfrog-ignore - test URL
		{"repo", "http://url/art", "http://url/art/api/npm/repo"},
		// jfrog-ignore - test URL
		{"repo", "http://url/art/", "http://url/art/api/npm/repo"},
		{"repo", "", "/api/npm/repo"},
		// jfrog-ignore - test URL
		{"", "http://url/art", "http://url/art/api/npm/"},
	}

	for _, testCase := range getRegistryTest {
		if GetNpmRepositoryUrl(testCase.repo, testCase.url) != testCase.expected {
			t.Errorf("The expected output of getRegistry(\"%s\", \"%s\") is %s. But the actual result is:%s", testCase.repo, testCase.url, testCase.expected, GetNpmRepositoryUrl(testCase.repo, testCase.url))
		}
	}
}

type dummyArtifactoryServiceDetails struct {
	auth.CommonConfigFields
}

func (det *dummyArtifactoryServiceDetails) GetVersion() (string, error) {
	return "7.0.0", nil
}

const npmAuthResponse = "_auth = someCred\nalways-auth = true\nemail = some@gmail.com"

var getNpmAuthCases = []struct {
	testName               string
	expectedNpmAuth        string
	isNpmAuthLegacyVersion bool
	user                   string
	password               string
	accessToken            string
}{
	{"basic auth", npmAuthResponse, false, "user", "password", ""},
	{"basic auth legacy", npmAuthResponse, true, "user", "password", ""},
	{"access token", constructNpmAuthToken("token"), false, "", "", "token"},
	{"access token legacy", npmAuthResponse, true, "", "", "token"},
}

func TestGetNpmAuth(t *testing.T) {
	// Prepare mock server
	testServer := commonTests.CreateRestsMockServer(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/"+npmAuthRestApi {
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(npmAuthResponse))
			assert.NoError(t, err)
		}
	})
	defer testServer.Close()

	for _, testCase := range getNpmAuthCases {
		t.Run(testCase.testName, func(t *testing.T) {
			authDetails := dummyArtifactoryServiceDetails{CommonConfigFields: auth.CommonConfigFields{Url: testServer.URL + "/", User: testCase.user, Password: testCase.password, AccessToken: testCase.accessToken}}
			actualNpmAuth, err := getNpmAuth(&authDetails, testCase.isNpmAuthLegacyVersion)
			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedNpmAuth, actualNpmAuth)
		})
	}
}
