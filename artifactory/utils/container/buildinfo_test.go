package container

import (
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/stretchr/testify/assert"
)

func TestGetManifestPaths(t *testing.T) {
	results := getManifestPaths("/hello-world/latest", "docker-local", Push)
	assert.Len(t, results, 2)
	expected := []string{"docker-local/hello-world/latest/*", "docker-local/latest/*"}
	for i, result := range results {
		assert.Equal(t, expected[i], result)
	}

	results = getManifestPaths("/hello-world/latest", "docker-local", Pull)
	assert.Len(t, results, 4)
	expected = append(expected, "docker-local/library/hello-world/latest/*", "docker-local/library/latest/*")
	for i, result := range results {
		assert.Equal(t, expected[i], result)
	}

	results = getManifestPaths("/docker-remote/hello-world/latest", "docker-remote", Push)
	assert.Len(t, results, 2)
	expected = []string{"docker-remote/docker-remote/hello-world/latest/*", "docker-remote/hello-world/latest/*"}
	for i, result := range results {
		assert.Equal(t, expected[i], result)
	}

	results = getManifestPaths("/docker-remote/hello-world/latest", "docker-remote", Pull)
	assert.Len(t, results, 4)
	expected = append(expected, "docker-remote/library/docker-remote/hello-world/latest/*", "docker-remote/library/hello-world/latest/*")
	for i, result := range results {
		assert.Equal(t, expected[i], result)
	}
}

func TestGetImageWithDigest(t *testing.T) {
	filePath := filepath.Join("..", "testdata", "container", "imageTagWithDigest")
	tag, sha256, err := GetImageTagWithDigest(filePath)
	assert.NoError(t, err)
	assert.Equal(t, "my-image-tag", tag.name)
	assert.Equal(t, "sha256:12345", sha256)
}

var dummySearchResults = &utils.ResultItem{
	Type:        "json",
	Actual_Md5:  "md5",
	Actual_Sha1: "sha1",
	Sha256:      "sha2",
	Size:        1,
}

func TestManifestConfig(t *testing.T) {
	dependencies, err := getDependenciesFromManifestConfig(createManifestConfig())
	assert.NoError(t, err)
	assert.Len(t, dependencies, 2)
}

func createManifestConfig() (map[string]*utils.ResultItem, string) {
	config := make(map[string]*utils.ResultItem, 0)
	config["manifest.json"] = dummySearchResults
	config["sha__123"] = dummySearchResults
	return config, "sha:123"
}

func TestManifestConfigNoManifestFound(t *testing.T) {
	_, err := getDependenciesFromManifestConfig(createEmptyManifestConfig())
	assert.Error(t, err)
}

func createEmptyManifestConfig() (map[string]*utils.ResultItem, string) {
	config := make(map[string]*utils.ResultItem, 0)
	return config, "sha:123"
}

func TestManifestConfigNoLayer(t *testing.T) {
	_, err := getDependenciesFromManifestConfig(createManifestConfigWithNoLayer())
	assert.Error(t, err)
}

func createManifestConfigWithNoLayer() (map[string]*utils.ResultItem, string) {
	config := make(map[string]*utils.ResultItem, 0)
	config["manifest.json"] = dummySearchResults
	return config, "sha:123"
}

func TestGetDependenciesFromManifestLayer(t *testing.T) {
	searchResults, manifest := createManifestConfigWithLayer()
	dependencies, err := getDependenciesFromManifestLayer(searchResults, manifest)
	assert.NoError(t, err)
	assert.Len(t, dependencies, 1)
}

func createManifestConfigWithLayer() (map[string]*utils.ResultItem, *manifest) {
	manifest := &manifest{
		Layers: []layer{{
			Digest:    "sha:1",
			MediaType: "MediaType",
		}},
	}
	searchResults := make(map[string]*utils.ResultItem, 0)
	searchResults["manifest.json"] = dummySearchResults
	searchResults["sha__1"] = dummySearchResults
	searchResults["sha__2"] = dummySearchResults
	return searchResults, manifest
}

func TestMissingDependenciesInManifestLayer(t *testing.T) {
	searchResults, manifest := createManifestConfigWithMissingLayer()
	_, err := getDependenciesFromManifestLayer(searchResults, manifest)
	assert.Error(t, err)
}

func createManifestConfigWithMissingLayer() (map[string]*utils.ResultItem, *manifest) {
	manifest := &manifest{
		Layers: []layer{
			{
				Digest:    "sha:1",
				MediaType: "MediaType",
			},
			//  Missing layer
			{
				Digest:    "sha:2",
				MediaType: "type",
			},
		},
	}
	searchResults := make(map[string]*utils.ResultItem, 0)
	searchResults["manifest.json"] = dummySearchResults
	searchResults["sha__1"] = dummySearchResults
	return searchResults, manifest
}

func TestForeignDependenciesInManifestLayer(t *testing.T) {
	searchResults, manifest := createManifestConfigWithForeignLayer()
	dependencies, err := getDependenciesFromManifestLayer(searchResults, manifest)
	assert.NoError(t, err)
	assert.Len(t, dependencies, 1)
}

func createManifestConfigWithForeignLayer() (map[string]*utils.ResultItem, *manifest) {
	manifest := &manifest{
		Layers: []layer{
			{
				Digest:    "sha:1",
				MediaType: "MediaType",
			},
			//  Foreign layer
			{
				Digest:    "sha:2",
				MediaType: foreignLayerMediaType,
			},
		},
	}
	searchResults := make(map[string]*utils.ResultItem, 0)
	searchResults["manifest.json"] = dummySearchResults
	searchResults["sha__1"] = dummySearchResults
	return searchResults, manifest
}
