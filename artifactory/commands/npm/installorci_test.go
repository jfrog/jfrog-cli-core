package npm

import (
	"io/ioutil"
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

	npmi := NpmCommandArgs{registry: "http://goodRegistry", jsonOutput: true, npmAuth: "_auth = YWRtaW46QVBCN1ZkZFMzN3NCakJiaHRGZThVb0JlZzFl"}
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
	var typeRestrictions = map[string]typeRestriction{
		"production=\"true\"":          prodOnly,
		"production=true":              prodOnly,
		"only = prod":                  prodOnly,
		"only=production":              prodOnly,
		"only = development":           devOnly,
		"only=dev":                     devOnly,
		"only=":                        defaultRestriction,
		"omit = [\"dev\"]\ndev = true": prodOnly,
		"omit = [\"abc\"]\ndev = true": all,
		"only=dev\nomit = [\"abc\"]":   all,
		"dev=true\nomit = [\"dev\"]":   prodOnly,
		"kuku=true":                    defaultRestriction}

	for json, typeRestriction := range typeRestrictions {
		npmi := NpmCommandArgs{}
		npmi.prepareConfigData([]byte(json))
		if npmi.typeRestriction != typeRestriction {
			t.Errorf("Type restriction was supposed to be %d but set to: %d when using the json:\n%s", typeRestriction, npmi.typeRestriction, json)
		}
	}
}

func TestParseDependencies(t *testing.T) {
	dependenciesJsonList, err := ioutil.ReadFile("../testdata/dependenciesList.json")
	if err != nil {
		t.Error(err)
	}

	expectedDependenciesList := []struct {
		Key        string
		pathToRoot [][]string
	}{
		{"underscore:1.4.4", [][]string{{"binary-search-tree:0.2.4", "nedb:1.0.2", "root"}}},
		{"@jfrog/npm_scoped:1.0.0", [][]string{{"root"}}},
		{"xml:1.0.1", [][]string{{"root"}}},
		{"xpm:0.1.1", [][]string{{"@jfrog/npm_scoped:1.0.0", "root"}}},
		{"binary-search-tree:0.2.4", [][]string{{"nedb:1.0.2", "root"}}},
		{"nedb:1.0.2", [][]string{{"root"}}},
		{"@ilg/es6-promisifier:0.1.9", [][]string{{"@ilg/cli-start-options:0.1.19", "xpm:0.1.1", "@jfrog/npm_scoped:1.0.0", "root"}}},
		{"wscript-avoider:3.0.2", [][]string{{"@ilg/cli-start-options:0.1.19", "xpm:0.1.1", "@jfrog/npm_scoped:1.0.0", "root"}}},
		{"yaml:0.2.3", [][]string{{"root"}}},
		{"@ilg/cli-start-options:0.1.19", [][]string{{"xpm:0.1.1", "@jfrog/npm_scoped:1.0.0", "root"}}},
		{"async:0.2.10", [][]string{{"nedb:1.0.2", "root"}}},
		{"find:0.2.7", [][]string{{"root"}}},
		{"jquery:3.2.0", [][]string{{"root"}}},
		{"nub:1.0.0", [][]string{{"find:0.2.7", "root"}, {"root"}}},
		{"shopify-liquid:1.d7.9", [][]string{{"xpm:0.1.1", "@jfrog/npm_scoped:1.0.0", "root"}}},
	}
	npmi := NpmCommandArgs{}
	npmi.dependencies = make(map[string]*dependency)
	err = npmi.parseDependencies([]byte(dependenciesJsonList), "myScope", []string{"root"})
	if err != nil {
		t.Error(err)
	}
	if len(expectedDependenciesList) != len(npmi.dependencies) {
		t.Error("The expected dependencies list length is", len(expectedDependenciesList), "and should be:\n", expectedDependenciesList,
			"\nthe actual dependencies list length is", len(npmi.dependencies), "and the list is:\n", npmi.dependencies)
		t.Error("The expected dependencies list length is", len(expectedDependenciesList), "and should be:\n", expectedDependenciesList,
			"\nthe actual dependencies list length is", len(npmi.dependencies), "and the list is:\n", npmi.dependencies)
	}
	for _, eDependency := range expectedDependenciesList {
		found := false
		for aDependency, v := range npmi.dependencies {
			if aDependency == eDependency.Key && assert.ElementsMatch(t, v.pathToRoot, eDependency.pathToRoot) {
				found = true
				break
			}
		}
		if !found {
			t.Error("The expected dependency:", eDependency, "is missing from the actual dependencies list:\n", npmi.dependencies)
		}
	}
}
