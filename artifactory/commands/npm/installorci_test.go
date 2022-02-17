package npm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"

	"github.com/stretchr/testify/assert"
)

const authToken = "YWRtaW46QVBCN1ZkZFMzN3NCakJiaHRGZThVb0JlZzFl"

func TestPrepareConfigData(t *testing.T) {
	currentDir, err := os.Getwd()
	assert.NoError(t, err)
	testdataPath := filepath.Join(currentDir, "artifactory", "commands", "testdata")
	testdataPath, err = filepath.Abs(testdataPath)
	assert.NoError(t, err)
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

	npmi := NpmInstallOrCiCommand{CommonArgs: CommonArgs{registry: "http://goodRegistry", jsonOutput: true, npmAuth: "_auth = " + authToken}}
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
	assert.Equal(t, authToken, os.Getenv(npmConfigAuthEnv))
	testsutils.UnSetEnvAndAssert(t, npmConfigAuthEnv)
}
