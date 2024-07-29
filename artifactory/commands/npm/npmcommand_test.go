package npm

import (
	"fmt"
	biutils "github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/gofrog/version"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/commands/utils"
	commonTests "github.com/jfrog/jfrog-cli-core/v2/common/tests"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	testsUtils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// #nosec G101 - Dummy token for tests.
const authToken = "YWRtaW46QVBCN1ZkZFMzN3NCakJiaHRGZThVb0JlZzFl"

func TestPrepareConfigData(t *testing.T) {
	configBefore := []byte(
		"json=true\n" +
			"user-agent=npm/5.5.1 node/v8.9.1 darwin x64\n" +
			"metrics-registry=http://somebadregistry\nscope=\n" +
			"//reg=ddddd\n" +
			"@jfrog:registry=http://somebadregistry\n" +
			"registry=http://somebadregistry\n" +
			"email=ddd@dd.dd\n" +
			"allow-same-version=false\n" +
			"cache-lock-retries=10")

	expectedConfig :=
		[]string{
			"json = true",
			"allow-same-version=false",
			"user-agent=npm/5.5.1 node/v8.9.1 darwin x64",
			"@jfrog:registry = http://goodRegistry",
			"email=ddd@dd.dd",
			"cache-lock-retries=10",
			"registry = http://goodRegistry",
		}

	npmi := NpmCommand{registry: "http://goodRegistry", jsonOutput: true, npmAuth: "_auth = " + authToken, npmVersion: version.NewVersion("9.5.0")}
	configAfter, err := npmi.prepareConfigData(configBefore)
	if err != nil {
		t.Error(err)
	}
	actualConfigArray := strings.Split(string(configAfter), "\n")
	for _, eConfig := range expectedConfig {
		found := false
		for _, aConfig := range actualConfigArray {
			if aConfig == eConfig {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("The expected config: %s is missing from the actual configuration list:\n %s", eConfig, actualConfigArray)
		}
	}

	// Assert that NPM_CONFIG__AUTH environment variable was set
	assert.Equal(t, authToken, os.Getenv(fmt.Sprintf(npmConfigAuthEnv, "//goodRegistry", utils.NpmConfigAuthKey)))
	testsUtils.UnSetEnvAndAssert(t, fmt.Sprintf(npmConfigAuthEnv, "//goodRegistry", utils.NpmConfigAuthKey))
}

func TestSetNpmConfigAuthEnv(t *testing.T) {
	testCases := []struct {
		name        string
		npmCm       *NpmCommand
		authKey     string
		value       string
		expectedEnv string
	}{
		{
			name: "set scoped registry auth env",
			npmCm: &NpmCommand{
				npmVersion: version.NewVersion("9.3.1"),
			},
			authKey:     utils.NpmConfigAuthKey,
			value:       "some_auth_token",
			expectedEnv: "npm_config_//registry.example.com:_auth",
		},
		{
			name: "set scoped registry authToken env",
			npmCm: &NpmCommand{
				npmVersion: version.NewVersion("9.3.1"),
			},
			authKey:     utils.NpmConfigAuthTokenKey,
			value:       "some_auth_token",
			expectedEnv: "npm_config_//registry.example.com:_authToken",
		},
		{
			name: "set legacy auth env",
			npmCm: &NpmCommand{
				npmVersion: version.NewVersion("8.16.3"),
			},
			authKey:     utils.NpmConfigAuthKey,
			value:       "some_auth_token",
			expectedEnv: "npm_config__auth",
		},
		{
			name: "set legacy auth env even though authToken is passed",
			npmCm: &NpmCommand{
				npmVersion: version.NewVersion("8.16.3"),
			},
			authKey:     utils.NpmConfigAuthTokenKey,
			value:       "some_auth_token",
			expectedEnv: "npm_config__auth",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.npmCm.registry = "https://registry.example.com"
			err := tc.npmCm.setNpmConfigAuthEnv(tc.value, tc.authKey)
			assert.NoError(t, err)
			envValue := os.Getenv(tc.expectedEnv)
			assert.Equal(t, tc.value, envValue)
			assert.NoError(t, os.Unsetenv(tc.expectedEnv))
		})
	}
}

func TestSetArtifactoryAsResolutionServer(t *testing.T) {
	tmpDir, createTempDirCallback := tests.CreateTempDirWithCallbackAndAssert(t)
	defer createTempDirCallback()

	npmProjectPath := filepath.Join("..", "..", "..", "tests", "testdata", "npm-project")
	err := biutils.CopyDir(npmProjectPath, tmpDir, false, nil)
	assert.NoError(t, err)

	cwd, err := os.Getwd()
	assert.NoError(t, err)
	chdirCallback := testsUtils.ChangeDirWithCallback(t, cwd, tmpDir)
	defer chdirCallback()

	// Prepare mock server
	testServer, serverDetails, _ := commonTests.CreateRtRestsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI == "/api/system/version" {
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte("{\"version\" : \"7.75.4\"}"))
			assert.NoError(t, err)
		}
	})
	defer testServer.Close()

	depsRepo := "my-rt-resolution-repo"

	clearResolutionServerFunc, err := SetArtifactoryAsResolutionServer(serverDetails, depsRepo)
	assert.NoError(t, err)
	assert.NotNil(t, clearResolutionServerFunc)
	defer func() {
		assert.NoError(t, clearResolutionServerFunc())
	}()

	assert.FileExists(t, filepath.Join(tmpDir, ".npmrc"))
}
