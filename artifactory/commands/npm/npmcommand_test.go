package npm

import (
	"fmt"
	"github.com/jfrog/gofrog/version"
	testsUtils "github.com/jfrog/jfrog-client-go/utils/tests"
	"github.com/stretchr/testify/assert"
	"os"
	"strings"
	"testing"
)

// #nosec G101 -- Dummy token for tests.
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
	assert.Equal(t, authToken, os.Getenv(fmt.Sprintf(npmConfigAuthEnv, "//goodRegistry")))
	testsUtils.UnSetEnvAndAssert(t, fmt.Sprintf(npmConfigAuthEnv, "//goodRegistry"))
}

func TestSetNpmConfigAuthEnv(t *testing.T) {
	testCases := []struct {
		name        string
		npmCm       *NpmCommand
		value       string
		expectedEnv string
	}{
		{
			name: "set scoped registry auth env",
			npmCm: &NpmCommand{
				npmVersion: version.NewVersion("9.3.1"),
				registry:   "https://registry.example.com",
			},
			value:       "some_auth_token",
			expectedEnv: "npm_config_//registry.example.com:_auth",
		},
		{
			name: "set legacy auth env",
			npmCm: &NpmCommand{
				npmVersion: version.NewVersion("8.19.3"),
				registry:   "https://registry.example.com",
			},
			value:       "some_auth_token",
			expectedEnv: "npm_config__auth",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.npmCm.setNpmConfigAuthEnv(tc.value)
			assert.NoError(t, err)
			envValue := os.Getenv(tc.expectedEnv)
			assert.Equal(t, tc.value, envValue)
			assert.NoError(t, os.Unsetenv(tc.expectedEnv))
		})
	}
}
