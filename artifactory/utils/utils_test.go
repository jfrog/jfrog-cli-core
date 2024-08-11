package utils

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
)

func TestGetHomeDir(t *testing.T) {
	homeDir, err := coreutils.GetJfrogHomeDir()
	assert.NoError(t, err)
	secPath, err := coreutils.GetJfrogSecurityDir()
	assert.NoError(t, err)
	secFile, err := coreutils.GetJfrogSecurityConfFilePath()
	assert.NoError(t, err)
	certsPath, err := coreutils.GetJfrogCertsDir()
	assert.NoError(t, err)

	assert.Equal(t, secPath, filepath.Join(homeDir, coreutils.JfrogSecurityDirName))
	assert.Equal(t, secFile, filepath.Join(secPath, coreutils.JfrogSecurityConfFile))
	assert.Equal(t, certsPath, filepath.Join(secPath, coreutils.JfrogCertsDirName))
}

var versionsCases = []struct {
	version          string
	expectedMajorVer int
}{
	{"7.6.5", 7},
	{"7.0.0", 7},
	{"6.7.8", 6},
	{"5.9.9", 5},
}

func TestGetRtMajorVersion(t *testing.T) {
	for _, testCase := range versionsCases {
		t.Run("", func(t *testing.T) {
			testGetRtMajorVersion(t, testCase.version, testCase.expectedMajorVer)
		})
	}
}

func testGetRtMajorVersion(t *testing.T, version string, expected int) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/system/version" {
			content := []byte(fmt.Sprintf("{\"version\": \"%s\"}", version))
			_, err := w.Write(content)
			assert.NoError(t, err)
		}
	}))
	defer testServer.Close()
	serviceManager, err := CreateServiceManager(&config.ServerDetails{ArtifactoryUrl: testServer.URL + "/"}, -1, 0, false)
	assert.NoError(t, err)

	major, err := GetRtMajorVersion(serviceManager)
	assert.NoError(t, err)
	assert.Equal(t, expected, major)
}

func TestCreateMetadataServiceManager(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "api/v1/query" {
			content := []byte(fmt.Sprintf("{\"query\": \"queryBody\"\"}"))
			_, err := w.Write(content)
			assert.NoError(t, err)
		}
	}))
	defer testServer.Close()
	serviceManager, err := CreateMetadataServiceManager(&config.ServerDetails{MetadataUrl: testServer.URL + "/"}, false)
	assert.NoError(t, err)
	assert.NotEmpty(t, serviceManager)
}
