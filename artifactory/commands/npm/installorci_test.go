package npm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	npmutils "github.com/jfrog/jfrog-cli-core/v2/utils/npm"
	"github.com/stretchr/testify/assert"
)

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
			"_auth = YWRtaW46QVBCN1ZkZFMzN3NCakJiaHRGZThVb0JlZzFl"}

	npmi := InstallCiArgs{CommonArgs: CommonArgs{registry: "http://goodRegistry", jsonOutput: true, npmAuth: "_auth = YWRtaW46QVBCN1ZkZFMzN3NCakJiaHRGZThVb0JlZzFl"}}
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
}

func TestPrepareConfigDataTypeRestriction(t *testing.T) {
	var typeRestrictions = map[string]npmutils.TypeRestriction{
		"production=\"true\"":          npmutils.ProdOnly,
		"production=true":              npmutils.ProdOnly,
		"only = prod":                  npmutils.ProdOnly,
		"only=production":              npmutils.ProdOnly,
		"only = development":           npmutils.DevOnly,
		"only=dev":                     npmutils.DevOnly,
		"only=":                        npmutils.DefaultRestriction,
		"omit = [\"dev\"]\ndev = true": npmutils.ProdOnly,
		"omit = [\"abc\"]\ndev = true": npmutils.All,
		"only=dev\nomit = [\"abc\"]":   npmutils.All,
		"dev=true\nomit = [\"dev\"]":   npmutils.ProdOnly,
		"kuku=true":                    npmutils.DefaultRestriction}

	for json, typeRestriction := range typeRestrictions {
		npmi := InstallCiArgs{}
		npmi.prepareConfigData([]byte(json))
		if npmi.typeRestriction != typeRestriction {
			t.Errorf("Type restriction was supposed to be %d but set to: %d when using the json:\n%s", typeRestriction, npmi.typeRestriction, json)
		}
	}
}
