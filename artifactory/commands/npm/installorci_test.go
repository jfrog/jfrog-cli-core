package npm

import (
	biutils "github.com/jfrog/build-info-go/utils"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	npmi := NpmInstallOrCiCommand{CommonArgs: CommonArgs{registry: "http://goodRegistry", jsonOutput: true, npmAuth: "_auth = YWRtaW46QVBCN1ZkZFMzN3NCakJiaHRGZThVb0JlZzFl"}}
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
	var typeRestrictions = map[string]biutils.TypeRestriction{
		"production=\"true\"":          biutils.ProdOnly,
		"production=true":              biutils.ProdOnly,
		"only = prod":                  biutils.ProdOnly,
		"only=production":              biutils.ProdOnly,
		"only = development":           biutils.DevOnly,
		"only=dev":                     biutils.DevOnly,
		"only=":                        biutils.DefaultRestriction,
		"omit = [\"dev\"]\ndev = true": biutils.ProdOnly,
		"omit = [\"abc\"]\ndev = true": biutils.All,
		"only=dev\nomit = [\"abc\"]":   biutils.All,
		"dev=true\nomit = [\"dev\"]":   biutils.ProdOnly,
		"kuku=true":                    biutils.DefaultRestriction}

	for json, typeRestriction := range typeRestrictions {
		npmi := NpmInstallOrCiCommand{}
		_, err := npmi.prepareConfigData([]byte(json))
		assert.NoError(t, err)
		if npmi.typeRestriction != typeRestriction {
			t.Errorf("Type restriction was supposed to be %d but set to: %d when using the json:\n%s", typeRestriction, npmi.typeRestriction, json)
		}
	}
}
